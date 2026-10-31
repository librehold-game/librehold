package app

import (
	"context"
	"math"
	"slices"
	"sort"
	"strconv"

	"librehold/internal/game"
)

// PlayerState is everything one player may see of a world: their own villages in full, the public face of
// every player, and of other players' commands only those coming to them, without the troops.
type PlayerState struct {
	Me            game.PlayerID                  `json:"me"`
	WorldID       int64                          `json:"worldId"`
	WorldName     string                         `json:"worldName"`
	GameNumber    int                            `json:"gameNumber,omitempty"`
	Now           int64                          `json:"now"`
	ServerTimeMs  int64                          `json:"serverTime"`
	ClockOrigin   int64                          `json:"clockOriginLocalMs"`
	Paused        bool                           `json:"paused"`
	Config        game.WorldConfig               `json:"config"`
	Villages      map[string]*game.Village       `json:"villages"`
	Player        *OwnPlayer                     `json:"player"`
	Players       map[game.PlayerID]PublicPlayer `json:"players"`
	Commands      []CommandView                  `json:"commands"`
	Intel         map[string]*game.Intel         `json:"intel"`
	Scheduled     []*game.ScheduledSend          `json:"scheduled"`
	Human         *game.HumanState               `json:"human"`
	UnreadReports int                            `json:"unreadReports"`
	Instant       *InstantView                   `json:"instant,omitempty"`
	Hosted        *HostedView                    `json:"hosted,omitempty"`
}

// InstantView is a short world's clock as its players see it, in world time.
type InstantView struct {
	EndsAt    int64      `json:"endsAt"`
	JoinUntil int64      `json:"joinUntil"`
	Ended     *EndedView `json:"ended,omitempty"`
}

type EndedView struct {
	Reason string `json:"reason"`
	// ResultsRef is what the results page is reached by: the world id, or a player world's results code.
	ResultsRef  string `json:"resultsRef"`
	At          int64  `json:"at"`
	WinnerID    string `json:"winnerId"`
	WinnerName  string `json:"winnerName"`
	WinnerIsBot bool   `json:"winnerIsBot"`
}

func instantView(w WorldTx) *InstantView {
	cfg := w.Config()
	if !game.IsShort(cfg) {
		return nil
	}
	v := &InstantView{EndsAt: int64(cfg.DurationMin) * game.MsPerMinute, JoinUntil: int64(cfg.JoinWindowMin) * game.MsPerMinute}
	if s := w.State(); s.EndedAt != nil {
		e := &EndedView{Reason: s.EndReason, ResultsRef: strconv.FormatInt(w.Row().ID, 10), At: *s.EndedAt, WinnerID: string(s.WinnerID)}
		if h := w.Row().Host; h != nil {
			e.ResultsRef = h.ResultsCode
		}
		if p := w.Player(s.WinnerID); p != nil {
			e.WinnerName, e.WinnerIsBot = p.Name, !p.IsHuman
		}
		v.Ended = e
	}
	return v
}

type PublicPlayer struct {
	ID         game.PlayerID     `json:"id"`
	Name       string            `json:"name"`
	Color      string            `json:"color"`
	Status     game.PlayerStatus `json:"status"`
	Points     int               `json:"points"`
	VillageIDs []string          `json:"villageIds"`
	IsBot      bool              `json:"isBot"`
}

func publicPlayer(p *game.Player) PublicPlayer {
	return PublicPlayer{ID: p.ID, Name: p.Name, Color: p.Color, Status: p.Status, Points: p.Points, VillageIDs: p.VillageIDs,
		IsBot: !p.IsHuman}
}

// OwnPlayer is the player's full record (coins, queue slots, protection…), without the isHuman flag that the
// state never carries.
type OwnPlayer struct {
	*game.Player
	IsHuman bool `json:"isHuman,omitempty"`
}

// CommandView is a command as a player sees it. The fields below shadow the command's own: for someone else's
// command only its origin, target, type and times show, plus the load of a trade.
type CommandView struct {
	*game.Command
	Units          *game.UnitCounts     `json:"units,omitempty"`
	CatapultTarget *game.BuildingID     `json:"catapultTarget,omitempty"`
	Haul           *game.Resources      `json:"haul,omitempty"`
	Intercept      *game.InterceptOrder `json:"intercept,omitempty"`
	Threats        []string             `json:"threats,omitempty"`
	FieldHit       bool                 `json:"fieldHit,omitempty"`
}

func ownCommand(c *game.Command) CommandView {
	return CommandView{Command: c, Units: &c.Units, CatapultTarget: c.CatapultTarget, Haul: c.Haul, Intercept: c.Intercept,
		Threats: c.Threats, FieldHit: c.FieldHit}
}

func foreignCommand(c *game.Command) CommandView {
	v := CommandView{Command: c}
	if c.Type == "trade" {
		v.Haul = c.Haul
	}
	return v
}

// MemberOf is the account's player in the world, or ErrNotMember.
func (s *Service) MemberOf(ctx context.Context, acct *Account, worldID int64) (game.PlayerID, error) {
	return s.memberID(ctx, acct, worldID)
}

// WorldTime is the world's time and the wall clock it was taken at.
func (s *Service) WorldTime(ctx context.Context, worldID int64) (now, wall int64, err error) {
	row, err := s.worlds.Get(ctx, worldID)
	if err != nil {
		return 0, 0, err
	}
	wall = s.clock.NowMs()
	return WorldNow(row, wall), wall, nil
}

// view runs fn on committed state at world time now, for a member of the world.
func (s *Service) view(ctx context.Context, acct *Account, worldID int64, fn func(w WorldTx, me game.PlayerID) error) error {
	me, err := s.memberID(ctx, acct, worldID)
	if err != nil {
		return err
	}
	return s.worlds.Read(ctx, worldID, func(w WorldTx) error {
		w.SetNow(WorldNow(w.Row(), s.clock.NowMs()))
		return fn(w, me)
	})
}

// State processes the world's due events, then reads the player's state; villages are settled to now in
// memory only.
func (s *Service) State(ctx context.Context, acct *Account, worldID int64) (*PlayerState, error) {
	if _, err := s.memberID(ctx, acct, worldID); err != nil {
		return nil, err
	}
	if err := s.paced(ctx, acct); err != nil {
		return nil, err
	}
	return s.state(ctx, acct, worldID)
}

func (s *Service) state(ctx context.Context, acct *Account, worldID int64) (*PlayerState, error) {
	if _, _, err := s.ProcessDue(ctx, worldID, 0); err != nil {
		return nil, err
	}
	var st *PlayerState
	err := s.view(ctx, acct, worldID, func(w WorldTx, me game.PlayerID) error {
		st = s.playerState(w, me)
		return nil
	})
	return st, err
}

// humanView is a copy of the stored human with the quest bars filled in, so the view data is never persisted.
func humanView(w WorldTx, me game.PlayerID) *game.HumanState {
	h := w.Human(me)
	if h == nil {
		return nil
	}
	cp := *h
	cp.Quests.Progress = game.QuestProgress(w, me)
	return &cp
}

func (s *Service) playerState(w WorldTx, me game.PlayerID) *PlayerState {
	row := w.Row()
	st := &PlayerState{Me: me, WorldID: row.ID, WorldName: row.Name, GameNumber: row.GameNumber, Now: w.Now(), ServerTimeMs: s.clock.NowMs(),
		ClockOrigin: w.ClockOrigin(), Paused: row.PausedAtMs != nil, Config: *w.Config(),
		Villages: map[string]*game.Village{}, Players: map[game.PlayerID]PublicPlayer{}, Commands: []CommandView{},
		Intel: w.IntelOf(me), Scheduled: w.ScheduledSendsOf(me), Human: humanView(w, me), Instant: instantView(w), Hosted: hostedView(w)}
	for _, p := range w.Players() {
		st.Players[p.ID] = publicPlayer(p)
	}
	if st.Scheduled == nil {
		st.Scheduled = []*game.ScheduledSend{}
	}
	if p := w.Player(me); p != nil {
		st.Player = &OwnPlayer{Player: p}
	}
	own := game.Ref(me)
	seen := map[string]bool{}
	var mine []*game.Command
	if p := w.Player(me); p != nil {
		for _, id := range p.VillageIDs {
			v := w.Village(id)
			if v == nil || v.OwnerID != own {
				continue
			}
			game.Touch(w, v)
			st.Villages[id] = v
			for _, c := range w.CommandsHome(id) {
				if !seen[c.ID] {
					seen[c.ID] = true
					st.Commands = append(st.Commands, ownCommand(c))
					mine = append(mine, c)
				}
			}
		}
		for id := range st.Villages {
			for _, c := range w.CommandsTo(id) {
				if !seen[c.ID] {
					seen[c.ID] = true
					st.Commands = append(st.Commands, foreignCommand(c))
				}
			}
		}
		// Armies sent to meet one of the player's own armies on the road.
		for _, own := range mine {
			for _, id := range own.Threats {
				if c := w.Command(id); c != nil && !seen[id] {
					seen[id] = true
					st.Commands = append(st.Commands, foreignCommand(c))
				}
			}
		}
		// Other players' armies passing near the player's villages.
		for _, c := range game.Sighted(w, me, w.Now()) {
			if !seen[c.ID] {
				seen[c.ID] = true
				st.Commands = append(st.Commands, foreignCommand(c))
			}
		}
	}
	sort.Slice(st.Commands, func(i, j int) bool {
		a, b := st.Commands[i], st.Commands[j]
		if a.ArrivesAt != b.ArrivesAt {
			return a.ArrivesAt < b.ArrivesAt
		}
		return a.ID < b.ID
	})
	for _, r := range w.Reports(me, math.MaxInt64, 0) {
		if !r.Read {
			st.UnreadReports++
		}
	}
	return st
}

// Reports pages the player's reports, newest first, before a report time; at most 100 at once.
func (s *Service) Reports(ctx context.Context, acct *Account, worldID int64, before int64, limit int) ([]*game.Report, error) {
	if limit <= 0 || limit > 100 {
		limit = 100
	}
	if before <= 0 {
		before = math.MaxInt64
	}
	var out []*game.Report
	err := s.view(ctx, acct, worldID, func(w WorldTx, me game.PlayerID) error {
		out = w.Reports(me, before, limit)
		return nil
	})
	if out == nil {
		out = []*game.Report{}
	}
	return out, err
}

type MapTile struct {
	X          int                `json:"x"`
	Y          int                `json:"y"`
	ID         string             `json:"id"`
	Name       string             `json:"name"`
	OwnerID    game.PlayerRef     `json:"ownerId"`
	OwnerIsBot bool               `json:"ownerIsBot,omitempty"`
	Points     int                `json:"points"`
	Bonus      *game.VillageBonus `json:"bonus,omitempty"`
}

const maxMapSpan = 100

// MapRegion lists the villages of a rectangle, stored and not yet materialised, clamped to the map.
func (s *Service) MapRegion(ctx context.Context, acct *Account, worldID int64, x0, y0, x1, y1 int) ([]MapTile, error) {
	out := []MapTile{}
	err := s.view(ctx, acct, worldID, func(w WorldTx, _ game.PlayerID) error {
		size := w.Config().MapSize
		x0, y0, x1, y1 := max(0, x0), max(0, y0), min(size-1, x1), min(size-1, y1)
		if x0 > x1 || y0 > y1 {
			return nil
		}
		// Checked on the clamped region: on raw coordinates x1 − x0 can overflow past the limit.
		if x1-x0 > maxMapSpan || y1-y0 > maxMapSpan {
			return &ErrValidation{Field: "region", Code: "tooLarge"}
		}
		out = regionTiles(w, x0, y0, x1, y1)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// regionTiles are the villages of a rectangle inside the map, stored and not yet materialised, by x then y.
func regionTiles(w WorldTx, x0, y0, x1, y1 int) []MapTile {
	out := []MapTile{}
	stored := map[[2]int]bool{}
	for _, ref := range w.VillagesIn(x0, y0, x1, y1) {
		stored[[2]int{ref.X, ref.Y}] = true
		v := w.Village(ref.ID)
		tile := MapTile{X: v.X, Y: v.Y, ID: v.ID, Name: v.Name, OwnerID: v.OwnerID, Points: v.Points, Bonus: v.Bonus}
		if p := w.Player(game.PlayerID(v.OwnerID)); v.OwnerID != "" && p != nil {
			tile.OwnerIsBot = !p.IsHuman
		}
		out = append(out, tile)
	}
	for x := x0; x <= x1; x++ {
		for y := y0; y <= y1; y++ {
			if stored[[2]int{x, y}] || !game.TileHasVillage(w.Seed(), w.Config(), x, y) {
				continue
			}
			tp := game.BarbarianTemplate(w.Seed(), x, y)
			out = append(out, MapTile{X: x, Y: y, ID: game.VillageIDFor(x, y),
				Points: tp.Points, Bonus: tp.Bonus})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].X != out[j].X {
			return out[i].X < out[j].X
		}
		return out[i].Y < out[j].Y
	})
	return out
}

type PlayerRow struct {
	ID       game.PlayerID `json:"id"`
	Name     string        `json:"name"`
	Color    string        `json:"color"`
	Points   int           `json:"points"`
	Villages int           `json:"villages"`
	Att      float64       `json:"att"`
	Def      float64       `json:"def"`
	Sup      float64       `json:"sup"`
	Quit     bool          `json:"quit"`
	IsBot    bool          `json:"isBot"`
}

type DailyRow struct {
	ID    game.PlayerID `json:"id"`
	Name  string        `json:"name"`
	Att   float64       `json:"att"`
	Def   float64       `json:"def"`
	Loot  float64       `json:"loot"`
	Farm  float64       `json:"farm"`
	IsBot bool          `json:"isBot"`
}

type ConquestRow struct {
	T         int64          `json:"t"`
	X         int            `json:"x"`
	Y         int            `json:"y"`
	ToID      game.PlayerID  `json:"toId"`
	To        string         `json:"to"`
	FromID    game.PlayerRef `json:"fromId"`
	From      *string        `json:"from"`
	ToIsBot   bool           `json:"toIsBot"`
	FromIsBot bool           `json:"fromIsBot"`
}

// Ranking is one tab of the public rankings: points, villages and od share the player rows; daily is
// today's counters; conquests the takeovers, newest first.
func (s *Service) Ranking(ctx context.Context, acct *Account, worldID int64, tab string) (any, error) {
	if !slices.Contains([]string{"points", "villages", "od", "daily", "conquests"}, tab) {
		return nil, &ErrValidation{Field: "tab", Code: "unknown"}
	}
	var out any
	err := s.view(ctx, acct, worldID, func(w WorldTx, _ game.PlayerID) error {
		out = rankingRows(w, tab)
		return nil
	})
	return out, err
}

// rankingRows are one tab's rows of a world at the time its tx is set to; the tab is already checked.
func rankingRows(w WorldTx, tab string) any {
	players := w.Players()
	switch tab {
	case "daily":
		today := game.LocalDayIndex(w, w.Now())
		rows := []DailyRow{}
		for _, p := range players {
			for _, d := range p.Daily {
				if d.Day == today {
					rows = append(rows, DailyRow{ID: p.ID, Name: p.Name, Att: d.Att, Def: d.Def, Loot: d.Loot, Farm: d.Farm, IsBot: !p.IsHuman})
				}
			}
		}
		return rows
	case "conquests":
		list := w.ConquestsSince(math.MinInt64)
		rows := make([]ConquestRow, 0, len(list))
		for i := len(list) - 1; i >= 0; i-- {
			rows = append(rows, conquestRow(w, list[i]))
		}
		return rows
	default:
		return playerRows(players)
	}
}

func conquestRow(w WorldTx, c game.Conquest) ConquestRow {
	name := func(id game.PlayerID) string {
		if p := w.Player(id); p != nil {
			return p.Name
		}
		return "?"
	}
	isBot := func(id game.PlayerID) bool {
		p := w.Player(id)
		return p != nil && !p.IsHuman
	}
	row := ConquestRow{T: c.T, X: c.X, Y: c.Y, ToID: c.ToID, To: name(c.ToID), FromID: c.FromID, ToIsBot: isBot(c.ToID)}
	if c.FromID != "" {
		from := name(game.PlayerID(c.FromID))
		row.From = &from
		row.FromIsBot = isBot(game.PlayerID(c.FromID))
	}
	return row
}

// playerRows are the ranking's rows of every player who has joined.
func playerRows(players []*game.Player) []PlayerRow {
	rows := []PlayerRow{}
	for _, p := range players {
		if p.Status == game.StatusPending {
			continue
		}
		rows = append(rows, PlayerRow{ID: p.ID, Name: p.Name, Color: p.Color, Points: p.Points, Villages: len(p.VillageIDs),
			Att: p.OD.Att, Def: p.OD.Def, Sup: p.OD.Sup, Quit: p.Status == game.StatusQuit, IsBot: !p.IsHuman})
	}
	return rows
}

type ProfileVillage struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	X      int    `json:"x"`
	Y      int    `json:"y"`
	Points int    `json:"points"`
}

type Profile struct {
	Player   PublicPlayer       `json:"player"`
	Villages []ProfileVillage   `json:"villages"`
	History  game.PointsHistory `json:"history"`
}

func (s *Service) Profile(ctx context.Context, acct *Account, worldID int64, playerID game.PlayerID) (Profile, error) {
	var out Profile
	err := s.view(ctx, acct, worldID, func(w WorldTx, _ game.PlayerID) error {
		p := w.Player(playerID)
		if p == nil {
			return ErrNotFound
		}
		out = profileOf(w, p)
		return nil
	})
	return out, err
}

func profileOf(w WorldTx, p *game.Player) Profile {
	out := Profile{Player: publicPlayer(p), Villages: []ProfileVillage{}, History: p.History}
	for _, id := range p.VillageIDs {
		if v := w.Village(id); v != nil {
			out.Villages = append(out.Villages, ProfileVillage{ID: v.ID, Name: v.Name, X: v.X, Y: v.Y, Points: v.Points})
		}
	}
	return out
}

// FarmTargets is the farm assistant's list for one of the player's villages, after the player's filters;
// empty for a village that is not theirs.
func (s *Service) FarmTargets(ctx context.Context, acct *Account, worldID int64, villageID string) ([]game.FarmTarget, error) {
	out := []game.FarmTarget{}
	err := s.view(ctx, acct, worldID, func(w WorldTx, me game.PlayerID) error {
		for _, t := range game.FarmTargets(w, me, villageID) {
			if game.PassesFilters(w, me, t) {
				out = append(out, t)
			}
		}
		return nil
	})
	return out, err
}

// News is the world's news, newest first, with the private items of this player only.
func (s *Service) News(ctx context.Context, acct *Account, worldID int64, limit int) ([]*game.NewsItem, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	out := []*game.NewsItem{}
	err := s.view(ctx, acct, worldID, func(w WorldTx, me game.PlayerID) error {
		for _, n := range w.News(0) {
			if n.For == "" || n.For == me {
				out = append(out, n)
				if len(out) == limit {
					break
				}
			}
		}
		return nil
	})
	return out, err
}
