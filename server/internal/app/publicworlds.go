package app

import (
	"cmp"
	"context"
	"errors"
	"math"
	"slices"
	"time"

	"librehold/internal/game"
)

const (
	publicCacheFor   = time.Minute
	finishedShown    = 20
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

	BarbarianDensity string             `json:"barbarianDensity"`
	BattleLosses     string             `json:"battleLosses"`
	BotCount         int                `json:"botCount"`
	BotArrival       string             `json:"botArrival"`
	RampUpDays       float64            `json:"rampUpDays,omitempty"`
	Difficulty       string             `json:"difficulty"`
	BotStance        string             `json:"botStance"`
	HumanModifiers   game.SideModifiers `json:"humanModifiers"`
	BotModifiers     game.SideModifiers `json:"botModifiers"`
	SightRadius      float64            `json:"sightRadius"`
	NoSight          bool               `json:"noSight,omitempty"`

	// Instant games only.
	NoCoins            bool            `json:"noCoins,omitempty"`
	NobleCost          *game.Resources `json:"nobleCost,omitempty"`
	OwnedNobleDropMin  int             `json:"ownedNobleDropMin,omitempty"`
	OwnedNobleDropMax  int             `json:"ownedNobleDropMax,omitempty"`
	AcademyExtraNobles int             `json:"academyExtraNobles,omitempty"`
	HumanSeats         int             `json:"humanSeats,omitempty"`
	DominationShare    float64         `json:"dominationShare,omitempty"`
}

type PublicWorld struct {
	PublicWorldCard
	Ended    bool              `json:"ended"`
	Settings PublicSettings    `json:"settings"`
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

// publicWorldRow is the row of a world anyone may look at: a long world, or an instant game that is running.
func (s *Service) publicWorldRow(ctx context.Context, worldID int64) (*WorldRow, error) {
	row, err := s.worlds.Get(ctx, worldID)
	if err != nil {
		return nil, err
	}
	if row.Config.Kind == game.KindLong ||
		game.IsInstant(&row.Config) && instantInfoOf(WorldSummary{WorldRow: *row}, s.clock.NowMs()).running {
		return row, nil
	}
	return nil, ErrNotFound
}

func publicSettingsOf(cfg *game.WorldConfig) PublicSettings {
	st := PublicSettings{WorldSpeed: cfg.WorldSpeed, UnitSpeed: cfg.UnitSpeed, MapSize: cfg.MapSize,
		BeginnerProtectionDays: cfg.BeginnerProtectionDays, LastVillage: cfg.LastVillage, FieldBattles: game.FieldBattlesOn(cfg),
		DurationMin: cfg.DurationMin, BarbarianDensity: string(cfg.BarbarianDensity), BattleLosses: string(cfg.BattleLosses),
		BotCount: cfg.BotCount, BotArrival: cfg.BotArrival, RampUpDays: cfg.RampUpDays, Difficulty: string(cfg.Difficulty),
		BotStance: string(cfg.BotStance), HumanModifiers: cfg.HumanModifiers, BotModifiers: cfg.BotModifiers,
		SightRadius: game.SightRadiusOf(cfg), NoSight: cfg.NoSight, NoCoins: cfg.NoCoins, OwnedNobleDropMin: cfg.OwnedNobleDropMin,
		OwnedNobleDropMax: cfg.OwnedNobleDropMax, AcademyExtraNobles: cfg.AcademyExtraNobles, HumanSeats: cfg.HumanSeats,
		DominationShare: cfg.DominationShare}
	if cfg.NobleCost != (game.Resources{}) {
		cost := cfg.NobleCost
		st.NobleCost = &cost
	}
	return st
}

// PublicWorld is the page of a long world or a running instant game: its card, settings and latest large battles; cached publicCacheFor.
func (s *Service) PublicWorld(ctx context.Context, worldID int64) (*PublicWorld, error) {
	now := s.clock.NowMs()
	return s.publicWorld.get(worldID, now, publicCacheFor, func() (*PublicWorld, error) {
		if _, err := s.publicWorldRow(ctx, worldID); err != nil {
			return nil, err
		}
		var out PublicWorld
		err := s.worlds.Read(ctx, worldID, func(w WorldTx) error {
			sum := WorldSummary{WorldRow: *w.Row()}
			for _, p := range w.Players() {
				if p.Status == game.StatusActive {
					if p.IsHuman {
						sum.Humans++
					} else {
						sum.Bots++
					}
				}
			}
			cfg := w.Config()
			out.PublicWorldCard = s.worldCard(sum, now)
			out.Ended = sum.State.EndedAt != nil
			out.Settings = publicSettingsOf(cfg)
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

// PublicMap is a world's villages as dots: stored ones with their owner, generated ones as barbarians; cached
// publicCacheFor.
func (s *Service) PublicMap(ctx context.Context, worldID int64) (*PublicMap, error) {
	return s.publicMap.get(worldID, s.clock.NowMs(), publicCacheFor, func() (*PublicMap, error) {
		if _, err := s.publicWorldRow(ctx, worldID); err != nil {
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

// PublicWorldPlayer is a player's page in one world: the in-game profile plus where they rank and their conquests.
type PublicWorldPlayer struct {
	Profile
	Username  string        `json:"username,omitempty"` // a human with an account only
	Rank      int           `json:"rank"`               // by points among the players who have joined, 1-based
	Conquests []ConquestRow `json:"conquests"`          // this player's, newest first; T in wall ms
}

const playerConquestsShown = 20

type rankKey struct {
	world int64
	tab   string
}

type playerKey struct {
	world  int64
	player game.PlayerID
}

// wallAt maps an instant in a world's own time to wall-clock ms; a pause does not move it.
func wallAt(row *WorldRow, t int64) int64 { return row.StartedAtMs + row.PausedTotalMs + t }

// PublicRanking is one tab of a public world's ranking, whole; conquest times are wall ms. Cached publicCacheFor.
func (s *Service) PublicRanking(ctx context.Context, worldID int64, tab string) (any, error) {
	if !slices.Contains([]string{"points", "villages", "od", "daily", "conquests"}, tab) {
		return nil, &ErrValidation{Field: "tab", Code: "unknown"}
	}
	now := s.clock.NowMs()
	return s.publicRank.get(rankKey{worldID, tab}, now, publicCacheFor, func() (any, error) {
		row, err := s.publicWorldRow(ctx, worldID)
		if err != nil {
			return nil, err
		}
		var out any
		err = s.worlds.Read(ctx, worldID, func(w WorldTx) error {
			w.SetNow(WorldNow(w.Row(), now))
			out = rankingRows(w, tab)
			return nil
		})
		if rows, ok := out.([]ConquestRow); ok {
			for i := range rows {
				rows[i].T = wallAt(row, rows[i].T)
			}
		}
		return out, err
	})
}

// PublicWorldPlayer is one joined player of a public world; cached publicCacheFor.
func (s *Service) PublicWorldPlayer(ctx context.Context, worldID int64, playerID game.PlayerID) (*PublicWorldPlayer, error) {
	now := s.clock.NowMs()
	return s.publicPlayer.get(playerKey{worldID, playerID}, now, publicCacheFor, func() (*PublicWorldPlayer, error) {
		row, err := s.publicWorldRow(ctx, worldID)
		if err != nil {
			return nil, err
		}
		var out PublicWorldPlayer
		var accountID int64
		err = s.worlds.Read(ctx, worldID, func(w WorldTx) error {
			p := w.Player(playerID)
			if p == nil || p.Status == game.StatusPending {
				return ErrNotFound
			}
			w.SetNow(WorldNow(w.Row(), now))
			out = PublicWorldPlayer{Profile: profileOf(w, p), Rank: 1, Conquests: []ConquestRow{}}
			for _, q := range w.Players() {
				if q.Status != game.StatusPending && (q.Points > p.Points || q.Points == p.Points && q.ID < p.ID) {
					out.Rank++
				}
			}
			list := w.ConquestsSince(math.MinInt64)
			for i := len(list) - 1; i >= 0 && len(out.Conquests) < playerConquestsShown; i-- {
				if c := list[i]; c.ToID == playerID || c.FromID == game.PlayerRef(playerID) {
					r := conquestRow(w, c)
					r.T = wallAt(row, r.T)
					out.Conquests = append(out.Conquests, r)
				}
			}
			if p.IsHuman {
				accountID = p.AccountID
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
		if accountID != 0 {
			if acct, err := s.accounts.ByID(ctx, accountID); err == nil {
				out.Username = acct.Username
			} else if !errors.Is(err, ErrNotFound) {
				return nil, err
			}
		}
		return &out, nil
	})
}
