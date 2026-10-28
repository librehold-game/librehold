package postgres

import (
	"encoding/json"
	"sort"

	"github.com/jackc/pgx/v5"

	"librehold/internal/game"
)

func (u *uow) touchCommand(c *game.Command) {
	for _, id := range []string{c.OriginID, c.TargetID} {
		if e := u.villages[id]; e != nil && e.v != nil {
			u.touch(e.v.OwnerID)
		} else if v := u.Village(id); v != nil {
			u.touch(v.OwnerID)
		}
	}
}

// loadCommands tracks the stored commands matching where (on $2) that are not tracked yet.
func (u *uow) loadCommands(where string, arg any) {
	rows, err := u.tx.Query(u.ctx, `select id, data from commands where world_id = $1 and `+where, u.row.ID, arg)
	u.must(err)
	for rows.Next() {
		var id string
		var data []byte
		u.must(rows.Scan(&id, &data))
		track(u.commands, id, data)
	}
	u.must(rows.Err())
}

func (u *uow) Command(id string) *game.Command {
	if _, ok := u.commands[id]; !ok {
		u.loadCommands("id = $2", id)
		if _, ok := u.commands[id]; !ok {
			u.commands[id] = &entry[game.Command]{}
		}
	}
	c := u.commands[id].v
	if c != nil {
		u.touchCommand(c)
	}
	return c
}

func (u *uow) commandsWhere(match func(c *game.Command) bool) []*game.Command {
	var out []*game.Command
	for _, e := range u.commands {
		if e.v != nil && match(e.v) {
			out = append(out, e.v)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].ArrivesAt != out[j].ArrivesAt {
			return out[i].ArrivesAt < out[j].ArrivesAt
		}
		return out[i].ID < out[j].ID
	})
	for _, c := range out {
		u.touchCommand(c)
	}
	return out
}

func (u *uow) CommandsHome(villageID string) []*game.Command {
	u.loadCommands("home_id = $2", villageID)
	return u.commandsWhere(func(c *game.Command) bool { return game.CommandHome(c) == villageID })
}

func (u *uow) CommandsTo(villageID string) []*game.Command {
	u.loadCommands("target_id = $2", villageID)
	return u.commandsWhere(func(c *game.Command) bool { return c.TargetID == villageID })
}

// ArmiesMarching loads the armies on the road at now; ones already tracked keep their unit-of-work state.
func (u *uow) ArmiesMarching(now int64) []*game.Command {
	u.loadCommands("type in ('attack', 'support', 'return') and arrives_at > $2", now)
	return u.commandsSorted(func(c *game.Command) bool { return game.IsArmy(c) && c.ArrivesAt > now })
}

// MovingCommands loads every army and intercept on the road at now; ones already tracked keep their state.
func (u *uow) MovingCommands(now int64) []*game.Command {
	u.loadCommands("type in ('attack', 'support', 'return', 'intercept') and arrives_at > $2", now)
	return u.commandsSorted(func(c *game.Command) bool { return (game.IsArmy(c) || c.Type == "intercept") && c.ArrivesAt > now })
}

// commandsSorted are the tracked commands matching, by ID, without marking their owners touched.
func (u *uow) commandsSorted(match func(c *game.Command) bool) []*game.Command {
	var out []*game.Command
	for _, e := range u.commands {
		if e.v != nil && match(e.v) {
			out = append(out, e.v)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func (u *uow) PutCommand(c *game.Command) {
	u.commands.put(c.ID, c)
	u.touchCommand(c)
}

func (u *uow) DeleteCommand(id string) {
	if c := u.Command(id); c != nil {
		u.commands[id].v = nil
	}
}

func (u *uow) flushCommands(b *pgx.Batch) {
	wid := u.row.ID
	owners := func(c *game.Command) {
		for _, vid := range []string{c.OriginID, c.TargetID} {
			if v := u.Village(vid); v != nil {
				u.changed.CommandOwners = append(u.changed.CommandOwners, v.OwnerID)
			}
		}
	}
	old := func(orig []byte) {
		if orig != nil {
			var c game.Command
			u.must(json.Unmarshal(orig, &c))
			owners(&c)
		}
	}
	changesOrig(u.commands, func(id string, c *game.Command, data, orig []byte) {
		owners(c)
		old(orig)
		b.Queue(`insert into commands (world_id, id, type, origin_id, target_id, home_id, arrives_at, data)
			values ($1, $2, $3, $4, $5, $6, $7, $8)
			on conflict (world_id, id) do update set type = excluded.type, origin_id = excluded.origin_id,
			target_id = excluded.target_id, home_id = excluded.home_id, arrives_at = excluded.arrives_at, data = excluded.data`,
			wid, id, c.Type, c.OriginID, c.TargetID, game.CommandHome(c), c.ArrivesAt, data)
	}, func(id string, orig []byte) {
		old(orig)
		b.Queue(`delete from commands where world_id = $1 and id = $2`, wid, id)
	})
}
