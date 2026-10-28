package postgres

import (
	"encoding/json"
	"sort"

	"github.com/jackc/pgx/v5"

	"librehold/internal/app"
	"librehold/internal/game"
)

func ownerArg(owner game.PlayerRef) *string {
	if owner == "" {
		return nil
	}
	s := string(owner)
	return &s
}

// loadVillages tracks the stored villages among ids that are not tracked yet.
func (u *uow) loadVillages(ids []string) {
	var missing []string
	for _, id := range ids {
		if _, ok := u.villages[id]; !ok {
			missing = append(missing, id)
		}
	}
	if len(missing) == 0 {
		return
	}
	rows, err := u.tx.Query(u.ctx, `select id, data from villages where world_id = $1 and id = any($2)`, u.row.ID, missing)
	u.must(err)
	for rows.Next() {
		var id string
		var data []byte
		u.must(rows.Scan(&id, &data))
		track(u.villages, id, data)
	}
	u.must(rows.Err())
	for _, id := range missing {
		if _, ok := u.villages[id]; !ok {
			u.villages[id] = &entry[game.Village]{}
		}
	}
}

func (u *uow) Village(id string) *game.Village {
	u.loadVillages([]string{id})
	v := u.villages[id].v
	if v != nil {
		u.touch(v.OwnerID)
	}
	return v
}

func (u *uow) PutVillage(v *game.Village) {
	u.villages.put(v.ID, v)
	u.touch(v.OwnerID)
}

func (u *uow) VillagesIn(x0, y0, x1, y1 int) []game.VillageRef {
	byID := map[string]game.VillageRef{}
	rows, err := u.tx.Query(u.ctx, `select id, x, y, name, coalesce(owner_id, ''), points from villages
		where world_id = $1 and x between $2 and $3 and y between $4 and $5`, u.row.ID, x0, x1, y0, y1)
	u.must(err)
	for rows.Next() {
		var r game.VillageRef
		u.must(rows.Scan(&r.ID, &r.X, &r.Y, &r.Name, &r.OwnerID, &r.Points))
		byID[r.ID] = r
	}
	u.must(rows.Err())
	for id, e := range u.villages {
		if v := e.v; v != nil && v.X >= x0 && v.X <= x1 && v.Y >= y0 && v.Y <= y1 {
			byID[id] = game.VillageRef{ID: v.ID, X: v.X, Y: v.Y, Name: v.Name, OwnerID: v.OwnerID, Points: v.Points}
		} else {
			delete(byID, id)
		}
	}
	out := make([]game.VillageRef, 0, len(byID))
	for _, r := range byID {
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func (u *uow) SupportHosts(homeID string) []*game.Village {
	rows, err := u.tx.Query(u.ctx, `select host_id from village_support where world_id = $1 and home_id = $2`, u.row.ID, homeID)
	u.must(err)
	ids, err := pgx.CollectRows(rows, pgx.RowTo[string])
	u.must(err)
	u.loadVillages(ids)
	var out []*game.Village
	for _, e := range u.villages {
		if v := e.v; v != nil {
			if _, ok := v.SupportFrom[homeID]; ok {
				out = append(out, v)
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	for _, v := range out {
		u.touch(v.OwnerID)
	}
	return out
}

func (u *uow) PointsByOwner() map[game.PlayerID]int {
	tracked := make([]string, 0, len(u.villages))
	for id := range u.villages {
		tracked = append(tracked, id)
	}
	rows, err := u.tx.Query(u.ctx, `select owner_id, sum(points) from villages
		where world_id = $1 and owner_id is not null and id <> all($2) group by owner_id`, u.row.ID, tracked)
	u.must(err)
	out := map[game.PlayerID]int{}
	for rows.Next() {
		var owner string
		var points int
		u.must(rows.Scan(&owner, &points))
		out[owner] = points
	}
	u.must(rows.Err())
	for _, e := range u.villages {
		if v := e.v; v != nil && v.OwnerID != "" {
			out[game.PlayerID(v.OwnerID)] += v.Points
		}
	}
	return out
}

func (u *uow) flushVillages(b *pgx.Batch) {
	changesOrig(u.villages, func(id string, v *game.Village, data, orig []byte) {
		ch := app.VillageChange{ID: id, NewOwner: v.OwnerID, OnMap: true}
		if orig != nil {
			var old struct {
				OwnerID game.PlayerRef `json:"ownerId"`
				Name    string         `json:"name"`
			}
			u.must(json.Unmarshal(orig, &old))
			ch.OldOwner, ch.OnMap = old.OwnerID, old.OwnerID != v.OwnerID || old.Name != v.Name
		}
		u.changed.Villages = append(u.changed.Villages, ch)
		b.Queue(`insert into villages (world_id, id, x, y, name, owner_id, points, data) values ($1, $2, $3, $4, $5, $6, $7, $8)
			on conflict (world_id, id) do update set x = excluded.x, y = excluded.y, name = excluded.name,
			owner_id = excluded.owner_id, points = excluded.points, data = excluded.data`,
			u.row.ID, id, v.X, v.Y, v.Name, ownerArg(v.OwnerID), v.Points, data)
		homes := make([]string, 0, len(v.SupportFrom))
		for home := range v.SupportFrom {
			homes = append(homes, home)
		}
		sort.Strings(homes)
		b.Queue(`delete from village_support where world_id = $1 and host_id = $2`, u.row.ID, id)
		if len(homes) > 0 {
			b.Queue(`insert into village_support (world_id, host_id, home_id) select $1, $2, unnest($3::text[])`, u.row.ID, id, homes)
		}
	}, func(string, []byte) {})
}
