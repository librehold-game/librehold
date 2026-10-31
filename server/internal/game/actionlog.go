package game

import (
	"encoding/json"
	"slices"
	"sort"
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
	for _, v := range sortedVillages(w, p) {
		s.Villages = append(s.Villages, snapshotVillage(w, v))
	}
	w.LogAction(ActorOf(p), false, "snapshot", s)
}

func sortedVillages(w World, p *Player) []*Village {
	var out []*Village
	for _, vid := range slices.Sorted(slices.Values(p.VillageIDs)) {
		if v := w.Village(vid); v != nil {
			out = append(out, v)
		}
	}
	return out
}

func snapshotVillage(w World, v *Village) SnapshotVillage {
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
	return SnapshotVillage{ID: v.ID, X: v.X, Y: v.Y, Buildings: v.Buildings, UnitsHome: UnitCounts(v.Units),
		UnitsAway: away, Resources: settled.Resources}
}

// Observation is what a human had in front of them when giving an order, logged as an "observe" entry right
// before the order: the situation half of a decision to learn from.
type Observation struct {
	WallMs        int64               `json:"wallMs"` // the real time, for the hour of the day; set by the caller
	Points        int                 `json:"points"`
	Villages      []ObservedVillage   `json:"villages"`
	FarmAssistant *FarmAssistantState `json:"farmAssistant,omitempty"` // templates, filters, auto-farm
	Choice        *ObservedChoice     `json:"choice,omitempty"`        // set by the caller for orders aimed at a tile
}

// ObservedOptions caps the farms an observed choice lists.
const ObservedOptions = 20

// ObservedChoice is what the player knew when aiming an order at a tile: the target, and the farms of the origin
// village's farm list they could have picked instead, nearest first.
type ObservedChoice struct {
	Target  ObservedTarget `json:"target"`
	Options []FarmTarget   `json:"options"`
}

type ObservedTarget struct {
	FarmTarget
	Owner string `json:"owner"` // barbarian | own | human | bot | empty
}

// ObserveChoice reads the target and the options as the actor's farm list and reports show them.
func ObserveChoice(w World, actor PlayerID, originID string, x, y int) *ObservedChoice {
	from := w.Village(originID)
	if from == nil || from.OwnerID != Ref(actor) {
		return nil
	}
	inbound := ownInbound(w, actor)
	t := TileInfo(w, x, y)
	target := ObservedTarget{FarmTarget: farmTargetFor(w, actor, from, t, inbound)}
	switch {
	case t.Kind != "village":
		target.Owner = "empty"
	case t.OwnerID == "":
		target.Owner = "barbarian"
	case t.OwnerID == Ref(actor):
		target.Owner = "own"
	case IsHumanID(PlayerID(t.OwnerID)):
		target.Owner = "human"
	default:
		target.Owner = "bot"
	}
	options := FarmTargets(w, actor, originID)
	sort.SliceStable(options, func(i, j int) bool {
		if options[i].Distance != options[j].Distance {
			return options[i].Distance < options[j].Distance
		}
		return options[i].ID < options[j].ID
	})
	if len(options) > ObservedOptions {
		options = options[:ObservedOptions]
	}
	if options == nil {
		options = []FarmTarget{}
	}
	return &ObservedChoice{Target: target, Options: options}
}

type ObservedVillage struct {
	SnapshotVillage
	Research     UnitCounts        `json:"research"`
	StorageCap   float64           `json:"storageCap"`
	PopUsed      int               `json:"popUsed"`
	PopCap       int               `json:"popCap"`
	Production   Resources         `json:"production"` // per hour
	BuildQueue   []BuildJob        `json:"buildQueue"`
	RecruitQueue []ObservedRecruit `json:"recruitQueue"`
	Outgoing     []ObservedCommand `json:"outgoing"`
	Incoming     []ObservedCommand `json:"incoming"`
}

type ObservedRecruit struct {
	Unit        UnitID `json:"unit"`
	Left        int    `json:"left"`
	CompletesAt int64  `json:"completesAt"`
}

// ObservedCommand is an army or trade on the move; an incoming one carries no units, as the player cannot see them.
type ObservedCommand struct {
	ID        string      `json:"id"`
	Type      string      `json:"type"`
	OriginID  string      `json:"originId"`
	TargetID  string      `json:"targetId"`
	ArrivesAt int64       `json:"arrivesAt"`
	Units     *UnitCounts `json:"units,omitempty"`
}

// ObserveHuman reads the human's villages without settling them.
func ObserveHuman(w World, id PlayerID) Observation {
	o := Observation{Villages: []ObservedVillage{}}
	p := w.Player(id)
	if p == nil {
		return o
	}
	o.Points = p.Points
	if h := w.Human(id); h != nil {
		fa := h.FarmAssistant
		o.FarmAssistant = &fa
	}
	for _, v := range sortedVillages(w, p) {
		ov := ObservedVillage{SnapshotVillage: snapshotVillage(w, v), Research: UnitCounts(v.Research), StorageCap: VillageStorageCapacity(v),
			PopUsed: PopulationUsed(w, v), PopCap: VillageFarmCapacity(v), BuildQueue: slices.Clone(v.BuildQueue),
			RecruitQueue: []ObservedRecruit{}, Outgoing: []ObservedCommand{}, Incoming: []ObservedCommand{}}
		if ov.BuildQueue == nil {
			ov.BuildQueue = []BuildJob{}
		}
		for _, r := range ResourceIDs {
			ov.Production.Set(r, VillageProductionPerHour(v, r, w.Config()))
		}
		for _, b := range RecruitBuildings {
			for _, j := range *v.RecruitQueues.Q(b) {
				ov.RecruitQueue = append(ov.RecruitQueue, ObservedRecruit{Unit: j.Unit, Left: j.Total - j.Done, CompletesAt: j.CompletesAt})
			}
		}
		for _, c := range w.CommandsHome(v.ID) {
			units := c.Units
			ov.Outgoing = append(ov.Outgoing, ObservedCommand{ID: c.ID, Type: c.Type, OriginID: c.OriginID, TargetID: c.TargetID,
				ArrivesAt: c.ArrivesAt, Units: &units})
		}
		for _, c := range w.CommandsTo(v.ID) {
			if c.Type == "return" {
				continue
			}
			ov.Incoming = append(ov.Incoming, ObservedCommand{ID: c.ID, Type: c.Type, OriginID: c.OriginID, TargetID: c.TargetID,
				ArrivesAt: c.ArrivesAt})
		}
		o.Villages = append(o.Villages, ov)
	}
	return o
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
