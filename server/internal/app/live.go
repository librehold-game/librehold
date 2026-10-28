package app

import (
	"context"
	"time"

	"librehold/internal/game"
)

const (
	liveCacheFor = 30 * time.Second
	liveEvents   = 5
)

// PublicEventView is a public battle or conquest as the site shows it.
type PublicEventView struct {
	At          int64  `json:"at"` // wall ms
	Kind        string `json:"kind"`
	WorldID     int64  `json:"worldId"`
	WorldName   string `json:"worldName"`
	GameNumber  int    `json:"gameNumber,omitempty"`
	Instant     bool   `json:"instant"`
	Attacker    string `json:"attacker"`
	Defender    string `json:"defender"`
	AttackerBot bool   `json:"attackerBot"`
	DefenderBot bool   `json:"defenderBot"`
	Troops      int    `json:"troops"`
	X           int    `json:"x"`
	Y           int    `json:"y"`
}

func publicEventViews(list []game.PublicEvent) []PublicEventView {
	out := make([]PublicEventView, len(list))
	for i, e := range list {
		out[i] = PublicEventView{At: e.At.UnixMilli(), Kind: e.Kind, WorldID: e.WorldID, WorldName: e.WorldName,
			GameNumber: e.GameNumber, Instant: e.WorldKind == game.KindInstant, Attacker: e.Attacker, Defender: e.Defender,
			AttackerBot: e.AttackerBot, DefenderBot: e.DefenderBot, Troops: e.Troops, X: e.X, Y: e.Y}
	}
	return out
}

// LiveStrip is the front page's proof of life: the instant games running now and the latest large battles.
type LiveStrip struct {
	Games        int               `json:"games"`
	Humans       int               `json:"humans"`
	Bots         int               `json:"bots"`
	OpenSeats    int               `json:"openSeats"`
	LastBattleAt *int64            `json:"lastBattleAt"` // wall ms; null when there was none
	Events       []PublicEventView `json:"events"`
}

// Live is cached for liveCacheFor, so the front page never adds load to the game tables.
func (s *Service) Live(ctx context.Context) (LiveStrip, error) {
	return s.live.get(struct{}{}, s.clock.NowMs(), liveCacheFor, func() (LiveStrip, error) {
		worlds, err := s.instantWorlds(ctx)
		if err != nil {
			return LiveStrip{}, err
		}
		var out LiveStrip
		for _, w := range worlds {
			if w.running {
				out.Games++
				out.Humans += w.Humans
				out.Bots += w.Bots
			}
		}
		out.OpenSeats, _, _ = s.openInstant(worlds)
		events, err := s.worlds.PublicEvents(ctx, PublicEventQuery{MinTroops: game.LargeBattleTroops, Limit: liveEvents})
		if err != nil {
			return LiveStrip{}, err
		}
		out.Events = publicEventViews(events)
		at, ok, err := s.worlds.LastBattleAt(ctx)
		if err != nil {
			return LiveStrip{}, err
		}
		if ok {
			ms := at.UnixMilli()
			out.LastBattleAt = &ms
		}
		return out, nil
	})
}

// openInstant is the instant game taking players now: its free seats and how long until it stops taking them.
func (s *Service) openInstant(worlds []instantInfo) (seats int, closesInMs int64, ok bool) {
	for _, w := range worlds {
		if s.takesPlayers(w) {
			closes := int64(w.Config.JoinWindowMin)*game.MsPerMinute - WorldNow(&w.WorldRow, s.clock.NowMs())
			return w.Config.HumanSeats - w.Humans, closes, true
		}
	}
	return 0, 0, false
}
