package app

import (
	"cmp"
	"context"
	"slices"
	"time"

	"librehold/internal/game"
)

const (
	publicCacheFor   = time.Minute
	finishedShown    = 20
	topShown         = 10
	worldEventsShown = 10
)

// PublicWorldCard is a world as anyone may see it: never its seed, never who plays from which account.
type PublicWorldCard struct {
	ID         int64   `json:"id"`
	Name       string  `json:"name"`
	GameNumber int     `json:"gameNumber,omitempty"` // instant only
	Instant    bool    `json:"instant"`
	Status     string  `json:"status"` // open | closed
	Paused     bool    `json:"paused"`
	StartedAt  int64   `json:"startedAt"`        // wall ms
	EndsAt     int64   `json:"endsAt,omitempty"` // instant only, wall ms
	Day        int     `json:"day,omitempty"`    // long only, from 1
	Humans     int     `json:"humans"`
	Bots       int     `json:"bots"`
	Speed      float64 `json:"speed"`
	OpenSeats  int     `json:"openSeats,omitempty"` // instant only, while it takes players
}

type FinishedGameCard struct {
	WorldID    int64  `json:"worldId"`
	Name       string `json:"name"`
	GameNumber int    `json:"gameNumber,omitempty"`
	EndedAt    int64  `json:"endedAt"`
}

type PublicWorldList struct {
	Running  []PublicWorldCard  `json:"running"`  // running instant games, newest first
	Long     []PublicWorldCard  `json:"long"`     // every long world: open first, then by id
	Finished []FinishedGameCard `json:"finished"` // the last finishedShown
}

type PublicSettings struct {
	WorldSpeed             float64 `json:"worldSpeed"`
	UnitSpeed              float64 `json:"unitSpeed"`
	MapSize                int     `json:"mapSize"`
	BeginnerProtectionDays float64 `json:"beginnerProtectionDays"`
	LastVillage            string  `json:"lastVillage"`
	FieldBattles           bool    `json:"fieldBattles"`
	DurationMin            int     `json:"durationMin,omitempty"`
}

type PublicRankRow struct {
	Rank     int    `json:"rank"`
	Name     string `json:"name"`
	IsBot    bool   `json:"isBot"`
	Points   int    `json:"points"`
	Villages int    `json:"villages"`
}

type PublicWorld struct {
	PublicWorldCard
	Ended    bool              `json:"ended"`
	Settings PublicSettings    `json:"settings"`
	Top      []PublicRankRow   `json:"top"`    // the topShown best by points, then by id; pending players left out
	Events   []PublicEventView `json:"events"` // the latest worldEventsShown large ones
}

type PublicMapPlayer struct {
	Name  string `json:"name"`
	Color string `json:"color"`
	IsBot bool   `json:"isBot"`
}

type PublicMap struct {
	Size     int               `json:"size"`
	Players  []PublicMapPlayer `json:"players"`
	Villages [][3]int          `json:"villages"` // x, y, index into Players or -1 for barbarians
}

func (s *Service) worldCard(w WorldSummary, now int64) PublicWorldCard {
	c := PublicWorldCard{ID: w.ID, Name: w.Name, Status: w.Status, Paused: w.PausedAtMs != nil,
		StartedAt: w.StartedAtMs + w.PausedTotalMs, Humans: w.Humans, Bots: w.Bots, Speed: w.Config.WorldSpeed}
	if info := instantInfoOf(w, now); game.IsInstant(&w.Config) {
		c.Instant, c.GameNumber, c.EndsAt = true, w.GameNumber, info.endsAtMs
		if s.takesPlayers(info) {
			c.OpenSeats = w.Config.HumanSeats - w.Humans
		}
	} else {
		c.Day = int(WorldNow(&w.WorldRow, now)/game.MsPerDay) + 1
	}
	return c
}

// PublicWorlds lists the running instant games, the long worlds and the latest finished games; cached publicCacheFor.
func (s *Service) PublicWorlds(ctx context.Context) (PublicWorldList, error) {
	now := s.clock.NowMs()
	return s.publicWorlds.get(struct{}{}, now, publicCacheFor, func() (PublicWorldList, error) {
		list, err := s.worlds.List(ctx)
		if err != nil {
			return PublicWorldList{}, err
		}
		out := PublicWorldList{Running: []PublicWorldCard{}, Long: []PublicWorldCard{}, Finished: []FinishedGameCard{}}
		for _, w := range list {
			switch {
			case w.Config.Kind == game.KindLong:
				out.Long = append(out.Long, s.worldCard(w, now))
			case game.IsInstant(&w.Config) && instantInfoOf(w, now).running:
				out.Running = append(out.Running, s.worldCard(w, now))
			}
		}
		slices.SortFunc(out.Running, func(a, b PublicWorldCard) int { return cmp.Compare(b.ID, a.ID) })
		slices.SortFunc(out.Long, func(a, b PublicWorldCard) int {
			return cmp.Or(cmp.Compare(openFirst(a), openFirst(b)), cmp.Compare(a.ID, b.ID))
		})
		games, err := s.worlds.ArchivedGameList(ctx, finishedShown)
		if err != nil {
			return PublicWorldList{}, err
		}
		for _, g := range games {
			out.Finished = append(out.Finished, FinishedGameCard{WorldID: g.WorldID, Name: g.Name, GameNumber: g.GameNumber, EndedAt: g.EndedAtMs})
		}
		return out, nil
	})
}

// openFirst puts the worlds one can join and play now first.
func openFirst(c PublicWorldCard) int {
	if c.Status == "open" && !c.Paused {
		return 0
	}
	return 1
}

// publicLongWorld is the row of a long world; any other world has no public page.
func (s *Service) publicLongWorld(ctx context.Context, worldID int64) (*WorldRow, error) {
	row, err := s.worlds.Get(ctx, worldID)
	if err != nil {
		return nil, err
	}
	if row.Config.Kind != game.KindLong {
		return nil, ErrNotFound
	}
	return row, nil
}

// PublicWorld is a long world's page: its card, settings, top ten and latest large battles; cached publicCacheFor.
func (s *Service) PublicWorld(ctx context.Context, worldID int64) (*PublicWorld, error) {
	now := s.clock.NowMs()
	return s.publicWorld.get(worldID, now, publicCacheFor, func() (*PublicWorld, error) {
		if _, err := s.publicLongWorld(ctx, worldID); err != nil {
			return nil, err
		}
		var out PublicWorld
		err := s.worlds.Read(ctx, worldID, func(w WorldTx) error {
			sum := WorldSummary{WorldRow: *w.Row()}
			points := w.PointsByOwner()
			var ranked []*game.Player
			for _, p := range w.Players() {
				if p.Status == game.StatusActive {
					if p.IsHuman {
						sum.Humans++
					} else {
						sum.Bots++
					}
				}
				if p.Status != game.StatusPending {
					ranked = append(ranked, p)
				}
			}
			slices.SortFunc(ranked, func(a, b *game.Player) int {
				return cmp.Or(cmp.Compare(points[b.ID], points[a.ID]), cmp.Compare(a.ID, b.ID))
			})
			out.Top = []PublicRankRow{}
			for i, p := range ranked[:min(len(ranked), topShown)] {
				out.Top = append(out.Top, PublicRankRow{Rank: i + 1, Name: p.Name, IsBot: !p.IsHuman, Points: points[p.ID],
					Villages: len(p.VillageIDs)})
			}
			cfg := w.Config()
			out.PublicWorldCard = s.worldCard(sum, now)
			out.Ended = sum.State.EndedAt != nil
			out.Settings = PublicSettings{WorldSpeed: cfg.WorldSpeed, UnitSpeed: cfg.UnitSpeed, MapSize: cfg.MapSize,
				BeginnerProtectionDays: cfg.BeginnerProtectionDays, LastVillage: cfg.LastVillage, FieldBattles: game.FieldBattlesOn(cfg)}
			return nil
		})
		if err != nil {
			return nil, err
		}
		events, err := s.worlds.PublicEvents(ctx, PublicEventQuery{WorldID: worldID, MinTroops: game.LargeBattleTroops, Limit: worldEventsShown})
		if err != nil {
			return nil, err
		}
		out.Events = publicEventViews(events)
		return &out, nil
	})
}

// PublicMap is a long world's villages as dots: stored ones with their owner, generated ones as barbarians; cached
// publicCacheFor.
func (s *Service) PublicMap(ctx context.Context, worldID int64) (*PublicMap, error) {
	return s.publicMap.get(worldID, s.clock.NowMs(), publicCacheFor, func() (*PublicMap, error) {
		if _, err := s.publicLongWorld(ctx, worldID); err != nil {
			return nil, err
		}
		var out PublicMap
		err := s.worlds.Read(ctx, worldID, func(w WorldTx) error {
			cfg := w.Config()
			size := cfg.MapSize
			out = PublicMap{Size: size, Players: []PublicMapPlayer{}, Villages: [][3]int{}}
			index := map[game.PlayerRef]int{}
			stored := map[[2]int]bool{}
			for _, v := range w.VillagesIn(0, 0, size-1, size-1) {
				stored[[2]int{v.X, v.Y}] = true
				i, known := index[v.OwnerID]
				if !known {
					i = -1
					if p := w.Player(game.PlayerID(v.OwnerID)); v.OwnerID != "" && p != nil {
						i = len(out.Players)
						index[v.OwnerID] = i
						out.Players = append(out.Players, PublicMapPlayer{Name: p.Name, Color: p.Color, IsBot: !p.IsHuman})
					}
				}
				out.Villages = append(out.Villages, [3]int{v.X, v.Y, i})
			}
			for x := range size {
				for y := range size {
					if !stored[[2]int{x, y}] && game.TileHasVillage(w.Seed(), cfg, x, y) {
						out.Villages = append(out.Villages, [3]int{x, y, -1})
					}
				}
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
		return &out, nil
	})
}
