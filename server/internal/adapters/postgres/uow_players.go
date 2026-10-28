package postgres

import (
	"bytes"
	"errors"
	"sort"

	"github.com/jackc/pgx/v5"

	"librehold/internal/game"
)

func (u *uow) trackPlayer(id string, data []byte, accountID int64) *game.Player {
	_, known := u.players[id]
	p := track(u.players, id, data)
	if !known && p != nil {
		p.AccountID = accountID
		u.accounts[id] = accountID
	}
	return p
}

func (u *uow) Player(id game.PlayerID) *game.Player {
	if e, ok := u.players[id]; ok {
		return e.v
	}
	var data []byte
	var accountID int64
	err := u.tx.QueryRow(u.ctx, `select data, coalesce(account_id, 0) from players where world_id = $1 and id = $2`, u.row.ID, id).
		Scan(&data, &accountID)
	if errors.Is(err, pgx.ErrNoRows) {
		u.players[id] = &entry[game.Player]{}
		return nil
	}
	u.must(err)
	return u.trackPlayer(id, data, accountID)
}

func (u *uow) Players() []*game.Player {
	if !u.allPlayers {
		rows, err := u.tx.Query(u.ctx, `select id, data, coalesce(account_id, 0) from players where world_id = $1`, u.row.ID)
		u.must(err)
		for rows.Next() {
			var id string
			var data []byte
			var accountID int64
			u.must(rows.Scan(&id, &data, &accountID))
			u.trackPlayer(id, data, accountID)
		}
		u.must(rows.Err())
		u.allPlayers = true
	}
	out := make([]*game.Player, 0, len(u.players))
	for _, e := range u.players {
		if e.v != nil {
			out = append(out, e.v)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func (u *uow) PutPlayer(p *game.Player) { u.players.put(p.ID, p) }

// loadOne tracks the row of a single-key table, or a miss.
func loadOne[T any](u *uow, m idmap[string, T], table, keyCol, id string) *T {
	if e, ok := m[id]; ok {
		return e.v
	}
	var data []byte
	err := u.tx.QueryRow(u.ctx, `select data from `+table+` where world_id = $1 and `+keyCol+` = $2`, u.row.ID, id).Scan(&data)
	if errors.Is(err, pgx.ErrNoRows) {
		m[id] = &entry[T]{}
		return nil
	}
	u.must(err)
	return track(m, id, data)
}

func (u *uow) Brain(id game.PlayerID) *game.BotBrain {
	return loadOne(u, u.brains, "bot_brains", "player_id", id)
}

func (u *uow) Human(id game.PlayerID) *game.HumanState {
	h := loadOne(u, u.humans, "human_states", "player_id", id)
	if h != nil {
		u.touched[id] = true
	}
	return h
}

func (u *uow) PutBrain(id game.PlayerID, b *game.BotBrain) { u.brains.put(id, b) }

func (u *uow) PutHuman(id game.PlayerID, h *game.HumanState) {
	u.humans.put(id, h)
	u.touched[id] = true
}

func accountArg(id int64) *int64 {
	if id == 0 {
		return nil
	}
	return &id
}

func (u *uow) flushPlayers(b *pgx.Batch) {
	wid := u.row.ID
	// AccountID is not in the JSON, so a change of it alone also counts.
	for _, id := range sortedKeys(u.players) {
		e := u.players[id]
		if e.v == nil {
			continue
		}
		p, data := e.v, marshal(e.v)
		if e.stored && bytes.Equal(data, e.orig) && u.accounts[id] == p.AccountID {
			continue
		}
		b.Queue(`insert into players (world_id, id, account_id, name, is_human, status, points, data)
			values ($1, $2, $3, $4, $5, $6, $7, $8)
			on conflict (world_id, id) do update set account_id = excluded.account_id, name = excluded.name,
			is_human = excluded.is_human, status = excluded.status, points = excluded.points, data = excluded.data`,
			wid, id, accountArg(p.AccountID), p.Name, p.IsHuman, string(p.Status), p.Points, data)
	}
	changes(u.brains, func(id string, _ *game.BotBrain, data []byte) {
		b.Queue(`insert into bot_brains (world_id, player_id, data) values ($1, $2, $3)
			on conflict (world_id, player_id) do update set data = excluded.data`, wid, id, data)
	}, func(id string) { b.Queue(`delete from bot_brains where world_id = $1 and player_id = $2`, wid, id) })
	changes(u.humans, func(id string, _ *game.HumanState, data []byte) {
		u.changed.Humans = append(u.changed.Humans, id)
		b.Queue(`insert into human_states (world_id, player_id, data) values ($1, $2, $3)
			on conflict (world_id, player_id) do update set data = excluded.data`, wid, id, data)
	}, func(id string) { b.Queue(`delete from human_states where world_id = $1 and player_id = $2`, wid, id) })
}
