package game

import (
	"math"
	"sort"
)

// Instant worlds: short, small, fast worlds a guest plays at once. Every rule here is off (zero values) in a
// long world, so long-world balance and its test fixtures stay as they are.

type WorldKind string

const (
	KindLong    WorldKind = ""
	KindInstant WorldKind = "instant"
	KindPlayer  WorldKind = "player"
)

// InstantWorldConfig is the preset every instant world starts from; the numbers are tuned with the instant
// simulator (gametest.RunInstantSim) so a guest conquers a village within five minutes.
var InstantWorldConfig = func() WorldConfig {
	cfg := DefaultWorldConfig
	cfg.Kind = KindInstant
	cfg.DurationMin = 60
	cfg.JoinWindowMin = 10
	cfg.HumanSeats = 12
	cfg.BotCount = 18
	cfg.MapSize = 40
	cfg.BarbarianDensity = "sparse"
	cfg.WorldSpeed = 100
	cfg.UnitSpeed = 100
	cfg.BeginnerProtectionDays = 10.0 / 1440
	cfg.BotThinkEverySec = 20
	cfg.BotTimeScale = 24
	cfg.BarbarianLoyalty = 15
	cfg.NoCoins = true
	cfg.NobleCost = Resources{Wood: 18000, Clay: 18000, Iron: 18000}
	cfg.OwnedNobleDropMin, cfg.OwnedNobleDropMax = 50, 60
	cfg.AcademyExtraNobles = 1
	cfg.DominationShare = 0.6
	cfg.StartPreset = "instant"
	cfg.SnapshotEveryMin = 5
	cfg.BotStance = StanceNormal
	cfg.Difficulty = Easy
	cfg.BotArrival = "allAtOnce"
	cfg.LastVillage = "canBeLost"
	// Armies cross a tile in seconds here, so a passing army is seen from further away.
	cfg.SightRadius = 5
	return cfg
}()

// InstantStartVillage is the built-up village a player of an instant world starts with: the first weeks
// skipped, every smithy unit researched, a basic army and a hetman ready to march. Every level meets its
// requirements, so the academy stands on main 20, market 10 and smithy 20.
var InstantStartVillage = struct {
	Buildings map[BuildingID]int
	Resources Resources
	Units     UnitCounts
}{
	Buildings: map[BuildingID]int{BMain: 20, BBarracks: 15, BStable: 10, BWorkshop: 5, BSmithy: 20, BAcademy: 1,
		BMarket: 10, BFarm: 25, BStorage: 20, BWood: 20, BClay: 20, BIron: 20, BHide: 5, BWall: 10},
	Resources: Resources{20000, 20000, 20000},
	Units:     UnitCounts{USpear: 50, UAxe: 100, ULightCav: 30, UScout: 5, UNoble: 1},
}

func IsInstant(cfg *WorldConfig) bool { return cfg.Kind == KindInstant }

// IsShort is a world that plays one short game: an instant world or a player-hosted one.
func IsShort(cfg *WorldConfig) bool { return cfg.Kind == KindInstant || cfg.Kind == KindPlayer }

// BotDur scales a bot-brain duration (hours, days of moods and plans) to the world's bot time scale; in a short
// world it never exceeds a quarter of the game, so every plan fits before the bell.
func BotDur(w World, d int64) int64 {
	cfg := w.Config()
	if cfg.BotTimeScale > 1 {
		d = int64(math.Round(float64(d) / cfg.BotTimeScale))
	}
	if IsShort(cfg) && cfg.DurationMin > 0 {
		d = min(d, int64(cfg.DurationMin)*MsPerMinute/4)
	}
	return d
}

// villageTotal counts the villages a world can hold: generated village fields plus a village per bot and seat.
func villageTotal(seed uint32, cfg *WorldConfig) int {
	n := 0
	for x := range cfg.MapSize {
		for y := range cfg.MapSize {
			if TileHasVillage(seed, cfg, x, y) {
				n++
			}
		}
	}
	return n + cfg.BotCount + cfg.HumanSeats
}

// applyStartPreset builds up a new player village in an instant world.
func applyStartPreset(w World, v *Village) {
	if w.Config().StartPreset != "instant" {
		return
	}
	for b, lvl := range InstantStartVillage.Buildings {
		v.Buildings[b] = lvl
	}
	v.Resources = InstantStartVillage.Resources
	for _, u := range UnitIDs {
		v.Units[u] += InstantStartVillage.Units[u]
		if UnitNeedsResearch(u) {
			v.Research[u] = max(v.Research[u], 1)
		}
	}
	RecomputePoints(v)
}

// startInstantBot keeps an instant world's bot always online, thinking every BotThinkEverySec from a few seconds
// after it settles.
func startInstantBot(w World, id PlayerID) {
	b := w.Brain(id)
	if b == nil {
		return
	}
	b.Online = true
	b.SessionEndsAt = math.MaxInt64
	rng := Mulberry32(HiddenSeed(w, "firstThink", id))
	w.Schedule("think:"+id, w.Now()+int64(rng()*5000))
}

func instantThinkGap(w World, id PlayerID) int64 {
	rng := Mulberry32(HiddenSeed(w, "think", id, w.Now()))
	return int64(math.Round(w.Config().BotThinkEverySec * 1000 * uniform(rng, 0.6, 1.4)))
}

func init() { registerHandler("end", onEndEvent) }

// WorldState is what a world keeps beside its settings: when and how an instant world ended, and its totals.
type WorldState struct {
	EndedAt   *int64      `json:"endedAt,omitempty"`
	EndReason string      `json:"endReason,omitempty"` // "time" | "domination" | "host"
	WinnerID  PlayerID    `json:"winnerId,omitempty"`
	Totals    WorldTotals `json:"totals"`
	// Archive is built when the world ends; Archived says the app has copied it to the permanent archive.
	Archive  *Archive `json:"archive,omitempty"`
	Archived bool     `json:"archived,omitempty"`
}

// WorldTotals are an instant world's running counters, kept as the game goes so the archive needs no recount.
type WorldTotals struct {
	AttacksSent           int    `json:"attacksSent"`
	LargestBattleUnits    int    `json:"largestBattleUnits"`
	LargestBattleReportID string `json:"largestBattleReportId,omitempty"`
}

func Ended(w World) bool { return w.State().EndedAt != nil }

// scheduleEnd plans the end of an instant world at its time limit (world time).
func scheduleEnd(w World) {
	if cfg := w.Config(); IsShort(cfg) && cfg.DurationMin > 0 {
		w.Schedule("end:time", int64(cfg.DurationMin)*MsPerMinute)
	}
}

// onEndEvent ends the world: "end:time" at the limit (the top player by points wins), "end:<id>" when that human
// holds the domination share.
func onEndEvent(w World, ref string) error {
	if ref == "time" {
		endWorld(w, "time", topPlayer(w))
	} else {
		endWorld(w, "domination", PlayerID(ref))
	}
	return nil
}

// endWorld ends the world now for reason with winner; a world ends once.
func endWorld(w World, reason string, winner PlayerID) {
	if Ended(w) {
		return
	}
	s := w.State()
	now := w.Now()
	s.EndedAt = &now
	s.EndReason = reason
	s.WinnerID = winner
	if endHook != nil {
		endHook(w)
	}
	snapshotHumans(w)
}

// endHook writes the archive when a world ends (set by the archive code).
var endHook func(w World)

func topPlayer(w World) PlayerID {
	points := w.PointsByOwner()
	ps := w.Players()
	sort.SliceStable(ps, func(i, j int) bool {
		if points[ps[i].ID] != points[ps[j].ID] {
			return points[ps[i].ID] > points[ps[j].ID]
		}
		return ps[i].ID < ps[j].ID
	})
	if len(ps) == 0 {
		return ""
	}
	return ps[0].ID
}

// checkDomination ends an instant world early when a human holds the domination share of its villages.
func checkDomination(w World, owner PlayerID) {
	cfg := w.Config()
	if !IsShort(cfg) || cfg.DominationShare <= 0 || cfg.VillageTotal <= 0 || !IsHumanID(owner) {
		return
	}
	p := w.Player(owner)
	if p != nil && float64(len(p.VillageIDs)) >= cfg.DominationShare*float64(cfg.VillageTotal) {
		w.Schedule("end:"+string(owner), w.Now())
	}
}

func countAttack(w World, typ string) {
	if IsShort(w.Config()) && typ == "attack" {
		w.State().Totals.AttacksSent++
	}
}

func countBattle(w World, units int, reportID string) {
	if t := &w.State().Totals; IsShort(w.Config()) && units > t.LargestBattleUnits {
		t.LargestBattleUnits, t.LargestBattleReportID = units, reportID
	}
}
