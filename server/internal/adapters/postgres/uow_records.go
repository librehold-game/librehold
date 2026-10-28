package postgres

import (
	"encoding/json"
	"errors"
	"slices"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5"

	"librehold/internal/game"
)

// Ids from NewID share one counter, so for one prefix they order by insertion when compared by length and
// then bytewise; this is the order MemWorld keeps reports and news in.
const newestFirst = `length(id) desc, id collate "C" desc`

func makeReportKey(recipient game.PlayerID, id string) reportKey {
	return reportKey(recipient + "\x00" + id)
}

func (u *uow) AddReport(recipient game.PlayerID, r *game.Report) {
	r.ID = u.NewID("rep")
	u.reports.put(makeReportKey(recipient, r.ID), r)
	u.newReports[recipient] = append(u.newReports[recipient], r)
	u.touched[recipient] = true
}

// gone lists the ids of a recipient's reports deleted in this unit of work.
func (u *uow) goneReports(recipient game.PlayerID) []string {
	prefix := recipient + "\x00"
	gone := []string{}
	for k, e := range u.reports {
		if e.v == nil && strings.HasPrefix(string(k), prefix) {
			gone = append(gone, strings.TrimPrefix(string(k), prefix))
		}
	}
	return gone
}

func (u *uow) Reports(recipient game.PlayerID, before int64, limit int) []*game.Report {
	u.touched[recipient] = true
	var out []*game.Report
	full := func() bool { return limit > 0 && len(out) >= limit }
	pending := u.newReports[recipient]
	for i := len(pending) - 1; i >= 0 && !full(); i-- {
		if r := pending[i]; u.reports[makeReportKey(recipient, r.ID)].v != nil && r.Time < before {
			out = append(out, r)
		}
	}
	if full() {
		return out
	}
	sql := `select id, data from reports where world_id = $1 and recipient_id = $2 and time < $3 and id <> all($4)
		order by ` + newestFirst
	args := []any{u.row.ID, recipient, before, u.goneReports(recipient)}
	if limit > 0 {
		sql += ` limit $5`
		args = append(args, limit-len(out))
	}
	rows, err := u.tx.Query(u.ctx, sql, args...)
	u.must(err)
	for rows.Next() {
		var id string
		var data []byte
		u.must(rows.Scan(&id, &data))
		out = append(out, track(u.reports, makeReportKey(recipient, id), data))
	}
	u.must(rows.Err())
	return out
}

func (u *uow) loadReports(recipient game.PlayerID, ids []string) {
	var missing []string
	for _, id := range ids {
		if _, ok := u.reports[makeReportKey(recipient, id)]; !ok {
			missing = append(missing, id)
		}
	}
	if len(missing) == 0 {
		return
	}
	rows, err := u.tx.Query(u.ctx, `select id, data from reports where world_id = $1 and recipient_id = $2 and id = any($3)`,
		u.row.ID, recipient, missing)
	u.must(err)
	for rows.Next() {
		var id string
		var data []byte
		u.must(rows.Scan(&id, &data))
		track(u.reports, makeReportKey(recipient, id), data)
	}
	u.must(rows.Err())
}

func (u *uow) SetReportRead(recipient game.PlayerID, ids []string, read bool) {
	u.loadReports(recipient, ids)
	for _, id := range ids {
		if e := u.reports[makeReportKey(recipient, id)]; e != nil && e.v != nil {
			e.v.Read = read
		}
	}
	u.touched[recipient] = true
}

func (u *uow) DeleteReports(recipient game.PlayerID, ids []string) {
	for _, id := range ids {
		k := makeReportKey(recipient, id)
		if e, ok := u.reports[k]; ok {
			e.v = nil
		} else {
			u.reports[k] = &entry[game.Report]{stored: true}
		}
	}
	u.touched[recipient] = true
}

func (u *uow) Intel(player game.PlayerID, villageID string) *game.Intel {
	k := makeIntelKey(player, villageID)
	if e, ok := u.intel[k]; ok {
		return e.v
	}
	var data []byte
	err := u.tx.QueryRow(u.ctx, `select data from intel where world_id = $1 and player_id = $2 and village_id = $3`,
		u.row.ID, player, villageID).Scan(&data)
	if errors.Is(err, pgx.ErrNoRows) {
		u.intel[k] = &entry[game.Intel]{}
		return nil
	}
	u.must(err)
	return track(u.intel, k, data)
}

func (u *uow) PutIntel(player game.PlayerID, villageID string, in *game.Intel) {
	u.Intel(player, villageID)
	u.intel[makeIntelKey(player, villageID)].v = in
}

func (u *uow) IntelOf(player game.PlayerID) map[string]*game.Intel {
	rows, err := u.tx.Query(u.ctx, `select village_id, data from intel where world_id = $1 and player_id = $2`, u.row.ID, player)
	u.must(err)
	for rows.Next() {
		var village string
		var data []byte
		u.must(rows.Scan(&village, &data))
		track(u.intel, makeIntelKey(player, village), data)
	}
	u.must(rows.Err())
	out := map[string]*game.Intel{}
	prefix := player + "\x00"
	for k, e := range u.intel {
		if village, ok := strings.CutPrefix(string(k), prefix); ok && e.v != nil {
			out[village] = e.v
		}
	}
	return out
}

func (u *uow) ScheduledSend(id string) *game.ScheduledSend {
	return loadOne(u, u.sends, "scheduled_sends", "id", id)
}

func (u *uow) ScheduledSendsOf(owner game.PlayerID) []*game.ScheduledSend {
	rows, err := u.tx.Query(u.ctx, `select id, data from scheduled_sends where world_id = $1 and owner_id = $2`, u.row.ID, owner)
	u.must(err)
	for rows.Next() {
		var id string
		var data []byte
		u.must(rows.Scan(&id, &data))
		track(u.sends, id, data)
	}
	u.must(rows.Err())
	var out []*game.ScheduledSend
	for _, e := range u.sends {
		if e.v != nil && e.v.OwnerID == owner {
			out = append(out, e.v)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].SendAt != out[j].SendAt {
			return out[i].SendAt < out[j].SendAt
		}
		return out[i].ID < out[j].ID
	})
	return out
}

func (u *uow) PutScheduledSend(s *game.ScheduledSend) { u.sends.put(s.ID, s) }

func (u *uow) DeleteScheduledSend(id string) {
	if u.ScheduledSend(id) != nil {
		u.sends[id].v = nil
	}
}

func (u *uow) AddNews(n *game.NewsItem) {
	n.ID = u.NewID("news")
	u.news = append(u.news, n)
}

func (u *uow) News(limit int) []*game.NewsItem {
	var out []*game.NewsItem
	for i := len(u.news) - 1; i >= 0 && (limit <= 0 || len(out) < limit); i-- {
		out = append(out, u.news[i])
	}
	if limit > 0 && len(out) >= limit {
		return out
	}
	sql := `select data from news where world_id = $1 order by ` + newestFirst
	args := []any{u.row.ID}
	if limit > 0 {
		sql += ` limit $2`
		args = append(args, limit-len(out))
	}
	rows, err := u.tx.Query(u.ctx, sql, args...)
	u.must(err)
	for rows.Next() {
		var data []byte
		u.must(rows.Scan(&data))
		n := new(game.NewsItem)
		u.must(json.Unmarshal(data, n))
		out = append(out, n)
	}
	u.must(rows.Err())
	return out
}

func (u *uow) AddConquest(c game.Conquest) { u.conquests = append(u.conquests, c) }

func (u *uow) ConquestsSince(t int64) []game.Conquest {
	rows, err := u.tx.Query(u.ctx, `select data from conquests where world_id = $1 and t >= $2 order by seq`, u.row.ID, t)
	u.must(err)
	var out []game.Conquest
	for rows.Next() {
		var data []byte
		u.must(rows.Scan(&data))
		var c game.Conquest
		u.must(json.Unmarshal(data, &c))
		out = append(out, c)
	}
	u.must(rows.Err())
	for _, c := range u.conquests {
		if c.T >= t {
			out = append(out, c)
		}
	}
	return out
}

func (u *uow) flushRecords(b *pgx.Batch) {
	wid := u.row.ID
	changes(u.reports, func(k reportKey, r *game.Report, data []byte) {
		recipient, _, _ := strings.Cut(string(k), "\x00")
		u.changed.ReportRecipients = append(u.changed.ReportRecipients, recipient)
		b.Queue(`insert into reports (world_id, recipient_id, id, time, read, data) values ($1, $2, $3, $4, $5, $6)
			on conflict (world_id, recipient_id, id) do update set time = excluded.time, read = excluded.read, data = excluded.data`,
			wid, recipient, r.ID, r.Time, r.Read, data)
	}, func(k reportKey) {
		recipient, id, _ := strings.Cut(string(k), "\x00")
		u.changed.ReportRecipients = append(u.changed.ReportRecipients, recipient)
		b.Queue(`delete from reports where world_id = $1 and recipient_id = $2 and id = $3`, wid, recipient, id)
	})
	recipients := make([]game.PlayerID, 0, len(u.newReports))
	for r := range u.newReports {
		recipients = append(recipients, r)
	}
	slices.Sort(recipients)
	for _, r := range recipients {
		b.Queue(`delete from reports where world_id = $1 and recipient_id = $2 and id not in (
			select id from reports where world_id = $1 and recipient_id = $2 order by `+newestFirst+` limit $3)`,
			wid, r, game.CombatBalance.MaxReports)
	}
	changes(u.intel, func(k intelKey, _ *game.Intel, data []byte) {
		player, village, _ := strings.Cut(string(k), "\x00")
		b.Queue(`insert into intel (world_id, player_id, village_id, data) values ($1, $2, $3, $4)
			on conflict (world_id, player_id, village_id) do update set data = excluded.data`, wid, player, village, data)
	}, func(k intelKey) {
		player, village, _ := strings.Cut(string(k), "\x00")
		b.Queue(`delete from intel where world_id = $1 and player_id = $2 and village_id = $3`, wid, player, village)
	})
	changes(u.sends, func(id string, s *game.ScheduledSend, data []byte) {
		b.Queue(`insert into scheduled_sends (world_id, id, owner_id, send_at, data) values ($1, $2, $3, $4, $5)
			on conflict (world_id, id) do update set owner_id = excluded.owner_id, send_at = excluded.send_at, data = excluded.data`,
			wid, id, s.OwnerID, s.SendAt, data)
	}, func(id string) { b.Queue(`delete from scheduled_sends where world_id = $1 and id = $2`, wid, id) })
	for _, n := range u.news {
		b.Queue(`insert into news (world_id, id, time, data) values ($1, $2, $3, $4)`, wid, n.ID, n.Time, marshal(n))
	}
	if len(u.news) > 0 {
		b.Queue(`delete from news where world_id = $1 and id not in (
			select id from news where world_id = $1 order by `+newestFirst+` limit $2)`, wid, game.NewsConfig.Max)
	}
	for _, c := range u.conquests {
		b.Queue(`insert into conquests (world_id, t, data) values ($1, $2, $3)`, wid, c.T, marshal(c))
	}
	u.flushPublicEvents(b)
}
