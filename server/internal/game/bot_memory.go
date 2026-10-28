package game

import (
	"fmt"
	"sort"
	"sync"
)

// What a bot knows of the map: the villages it has discovered around its own.

type BattleSeen struct {
	Units        *UnitCounts
	Wall         *int
	Hide         *float64
	Storage      *float64
	Buildings    PartialBuildings
	HaulRatio    *float64
	Losses       bool
	Won          *bool
	AttackerLost *int
}

type NearTile struct {
	X, Y   int
	ID     string
	D      float64
	Points int
}

var (
	tileCacheMu sync.Mutex
	tileCache   = map[string][]NearTile{}
)

// GeneratedVillagesNear lists the generator's village fields within r of (x, y), nearest first, with template
// points. It depends only on the world's parameters, so the cache does not affect determinism; stored villages
// (players, taken) are the caller's to check.
func GeneratedVillagesNear(seed uint32, cfg *WorldConfig, x, y, r int) []NearTile {
	key := fmt.Sprintf("%d|%d|%s|%d|%d|%d", seed, cfg.MapSize, cfg.BarbarianDensity, x, y, r)
	tileCacheMu.Lock()
	hit, ok := tileCache[key]
	tileCacheMu.Unlock()
	if ok {
		return hit
	}
	var out []NearTile
	for tx := x - r; tx <= x+r; tx++ {
		for ty := y - r; ty <= y+r; ty++ {
			d := Distance(x, y, tx, ty)
			if d == 0 || d > float64(r) || !TileHasVillage(seed, cfg, tx, ty) {
				continue
			}
			out = append(out, NearTile{tx, ty, VillageIDFor(tx, ty), d, BarbarianTemplate(seed, tx, ty).Points})
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].D < out[j].D })
	tileCacheMu.Lock()
	if len(tileCache) > 5000 {
		tileCache = map[string][]NearTile{}
	}
	tileCache[key] = out
	tileCacheMu.Unlock()
	return out
}

func rememberVillage(b *BotBrain, id string, x, y int, owner PlayerRef, points int, now int64) *KnownVillage {
	k := b.Memory.Known[id]
	if k == nil {
		k = &KnownVillage{ID: id, X: x, Y: y, Hauls: []float64{}}
		b.Memory.Known[id] = k
	}
	k.OwnerID, k.Points, k.SeenAt = owner, points, now
	return k
}

// DiscoverNearby copies public map facts — other players' villages within BotConfig.KnowRadius (owner, points)
// — into the bot's memory. Barbarians are not copied: farming reads them from the map when choosing.
func DiscoverNearby(w World, botID PlayerID) {
	brain := w.Brain(botID)
	if brain == nil {
		return
	}
	mine := PlayerVillages(w, botID)
	r := BotConfig.KnowRadius
	if len(mine) > 0 {
		x0, y0, x1, y1 := mine[0].X, mine[0].Y, mine[0].X, mine[0].Y
		for _, m := range mine {
			x0, y0, x1, y1 = min(x0, m.X), min(y0, m.Y), max(x1, m.X), max(y1, m.Y)
		}
		ri := int(r)
		for _, v := range w.VillagesIn(x0-ri, y0-ri, x1+ri, y1+ri) {
			if v.OwnerID == "" || v.OwnerID == Ref(botID) {
				continue
			}
			for _, m := range mine {
				if Distance(m.X, m.Y, v.X, v.Y) <= r {
					rememberVillage(brain, v.ID, v.X, v.Y, v.OwnerID, v.Points, w.Now())
					break
				}
			}
		}
	}
	forgetMs := BotDur(w, int64(BotConfig.ForgetDays*float64(MsPerDay)))
	ids := make([]string, 0, len(brain.Memory.Known))
	for id := range brain.Memory.Known {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		k := brain.Memory.Known[id]
		if v := w.Village(k.ID); v != nil && v.OwnerID != k.OwnerID {
			k.OwnerID, k.Points = v.OwnerID, v.Points
		}
		// A barbarian without a lesson (wall, defenders, dried up) is forgotten after a few days.
		lesson := (k.Wall != nil && *k.Wall > 0) || (k.Units != nil && k.Units.Total() > 0) ||
			(k.DeadSince != nil && w.Now()-*k.DeadSince < forgetMs)
		last := k.SeenAt
		if k.LastAttackAt != nil {
			last = *k.LastAttackAt
		}
		if k.OwnerID == "" && !lesson && w.Now()-last > forgetMs {
			delete(brain.Memory.Known, id)
		}
	}
	brain.Memory.DiscoveredAt = w.Now()
}

// DiscoverIfStale refreshes the map at most once per BotConfig.RediscoverHours (at session start).
func DiscoverIfStale(w World, botID PlayerID) {
	brain := w.Brain(botID)
	if brain == nil {
		return
	}
	if brain.Memory.DiscoveredAt < 0 || w.Now()-brain.Memory.DiscoveredAt >= BotDur(w, int64(BotConfig.RediscoverHours*float64(MsPerHour))) {
		DiscoverNearby(w, botID)
	}
}

// BotBattle is one battle a bot saw: the wall it knew before the fight and the attacker's losses.
type BotBattle struct {
	ObserverID   PlayerID
	TargetOwner  PlayerRef
	WallKnown    *int
	AttackerLost int
}

var battleSpy func(BotBattle)

// SpyBotBattles observes every battle seen by a bot (tests only); returns a restore func.
func SpyBotBattles(fn func(BotBattle)) (restore func()) {
	prev := battleSpy
	battleSpy = fn
	return func() { battleSpy = prev }
}

// ObserveBattle makes a bot remember what it saw in a battle or scouting; after 3 poor hauls (or a hiding place
// at least as large as the storage) the farm dries up.
func ObserveBattle(w World, observerID PlayerID, target *Village, seen BattleSeen) {
	brain := w.Brain(observerID)
	if brain == nil {
		return
	}
	k := rememberVillage(brain, target.ID, target.X, target.Y, target.OwnerID, target.Points, w.Now())
	if battleSpy != nil {
		lost := 0
		if seen.AttackerLost != nil {
			lost = *seen.AttackerLost
		} else if seen.Losses {
			lost = 1
		}
		var wall *int
		if k.Wall != nil {
			wall = ptrTo(*k.Wall)
		}
		battleSpy(BotBattle{ObserverID: observerID, TargetOwner: target.OwnerID, WallKnown: wall, AttackerLost: lost})
	}
	hadUnits := k.Units != nil && k.Units.Total() > 0
	if seen.Units != nil && hadUnits && target.OwnerID != "" && seen.Units.Total() == 0 {
		now := w.Now()
		RecordHabit(w, observerID, PlayerID(target.OwnerID), HabitObservation{DodgeReturnAt: &now})
	}
	if seen.Units != nil {
		u := *seen.Units
		k.Units = &u
	}
	if seen.Wall != nil {
		k.Wall = ptrTo(*seen.Wall)
	}
	if seen.Hide != nil {
		k.Hide = ptrTo(*seen.Hide)
	}
	if seen.Storage != nil {
		k.Storage = ptrTo(*seen.Storage)
	}
	if seen.Buildings != nil {
		k.Buildings = PartialBuildings{}
		for b, l := range seen.Buildings {
			k.Buildings[b] = l
		}
	}
	k.Losses = ptrTo(seen.Losses)
	if seen.HaulRatio != nil {
		k.Hauls = append(k.Hauls, *seen.HaulRatio)
		if len(k.Hauls) > 3 {
			k.Hauls = append([]float64{}, k.Hauls[len(k.Hauls)-3:]...)
		}
	}
	dryHauls := false
	if len(k.Hauls) >= 3 {
		sum := 0.0
		for _, h := range k.Hauls {
			sum += h
		}
		dryHauls = sum/float64(len(k.Hauls)) < BotConfig.DeadHaulRatio
	}
	hidden := k.Hide != nil && k.Storage != nil && *k.Hide >= *k.Storage
	if dryHauls || hidden {
		k.DeadSince = ptrTo(w.Now())
		k.Hauls = []float64{}
	}
	// Campaign progress: a win on the target renews the deadline, a loss counts as a failed operation.
	if seen.Won != nil && brain.Mood.TargetVillage == target.ID {
		if *seen.Won {
			brain.Mood.ProgressAt = ptrTo(w.Now())
		} else {
			brain.Mood.Failures++
		}
	}
}

func ptrTo[T any](v T) *T { return &v }
