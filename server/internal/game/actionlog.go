package game

import (
	"encoding/json"
	"slices"
)

// LoggedAction is one entry of a world's action log, kept as data to learn from: an accepted order, a join or
// leave, an admin change, a bot action, or a snapshot of a human's own state.
type LoggedAction struct {
	Seq     int64           `json:"seq"`
	At      int64           `json:"at"`    // world time
	Actor   string          `json:"actor"` // a human's player key, a bot's player id, "admin" or "world"
	IsBot   bool            `json:"isBot"`
	Kind    string          `json:"kind"` // an order name for orders, bots' included
	Payload json.RawMessage `json:"payload"`
}

// NewLoggedAction encodes the payload at once, so later changes to it never reach the log.
func NewLoggedAction(at int64, actor string, isBot bool, kind string, payload any) LoggedAction {
	b, err := json.Marshal(payload)
	if err != nil {
		panic(&StoreError{Err: err})
	}
	return LoggedAction{At: at, Actor: actor, IsBot: isBot, Kind: kind, Payload: b}
}

// Snapshot is a human's own state at one moment: enough to learn from without replaying the log.
type Snapshot struct {
	Villages []SnapshotVillage `json:"villages"`
	Points   int               `json:"points"`
}

type SnapshotVillage struct {
	ID        string     `json:"id"`
	X         int        `json:"x"`
	Y         int        `json:"y"`
	Buildings Buildings  `json:"buildings"`
	UnitsHome UnitCounts `json:"unitsHome"`
	UnitsAway UnitCounts `json:"unitsAway"` // on the move or supporting elsewhere
	Resources Resources  `json:"resources"`
}

const longSnapshotEveryMin = 600

func snapshotEvery(cfg *WorldConfig) int64 {
	if cfg.SnapshotEveryMin > 0 {
		return int64(cfg.SnapshotEveryMin) * MsPerMinute
	}
	return longSnapshotEveryMin * MsPerMinute
}

// ActorOf is how a player appears in the action log: a human by its player key, a bot by its id.
func ActorOf(p *Player) string {
	if p.IsHuman && p.Key != "" {
		return p.Key
	}
	return string(p.ID)
}

// SnapshotHuman logs the human's snapshot; it reads the villages without settling them.
func SnapshotHuman(w World, id PlayerID) {
	p := w.Player(id)
	if p == nil || !p.IsHuman || p.Status != StatusActive {
		return
	}
	s := Snapshot{Villages: []SnapshotVillage{}, Points: p.Points}
	for _, vid := range slices.Sorted(slices.Values(p.VillageIDs)) {
		v := w.Village(vid)
		if v == nil {
			continue
		}
		settled := *v
		Accrue(&settled, w.Now(), w.Config())
		var away UnitCounts
		add := func(u UnitCounts) {
			for i, n := range u {
				away[i] += n
			}
		}
		for _, c := range w.CommandsHome(v.ID) {
			if c.Type != "trade" {
				add(c.Units)
			}
		}
		for _, host := range w.SupportHosts(v.ID) {
			add(host.SupportFrom[v.ID])
		}
		s.Villages = append(s.Villages, SnapshotVillage{ID: v.ID, X: v.X, Y: v.Y, Buildings: v.Buildings,
			UnitsHome: UnitCounts(v.Units), UnitsAway: away, Resources: settled.Resources})
	}
	w.LogAction(ActorOf(p), false, "snapshot", s)
}

func snapshotHumans(w World) {
	for _, p := range w.Players() {
		if p.IsHuman {
			SnapshotHuman(w, p.ID)
		}
	}
}

func scheduleSnapshot(w World) { w.Schedule("snap", w.Now()+snapshotEvery(w.Config())) }

func init() {
	registerHandler("snap", func(w World, _ string) error {
		snapshotHumans(w)
		scheduleSnapshot(w)
		return nil
	})
}
