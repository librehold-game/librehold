package game

import (
	"math"
	"sort"
)

// Loyalty, nobles, coins and village takeovers.

// CoinsForNextNoble is the coins for the next noble: villages + nobles owned.
func CoinsForNextNoble(w World, id PlayerID, ownedNobles int) int {
	villages := 0
	if p := w.Player(id); p != nil {
		villages = len(p.VillageIDs)
	}
	return villages*ConquestBalance.CoinsPerVillage + ownedNobles*ConquestBalance.CoinsPerNoble
}

// msPerLoyaltyPoint: a village regains +1 loyalty per hour, sped up by world speed.
func msPerLoyaltyPoint(worldSpeed float64) float64 {
	return float64(MsPerHour) / (ConquestBalance.LoyaltyPerHour * worldSpeed)
}

// SettleLoyalty regenerates loyalty lazily: whole points since the last settlement, at most 100.
func SettleLoyalty(v *Village, now int64, worldSpeed float64) {
	if now <= v.LoyaltyUpdatedAt {
		return
	}
	step := msPerLoyaltyPoint(worldSpeed)
	points := math.Floor(float64(now-v.LoyaltyUpdatedAt) / step)
	if points <= 0 {
		return
	}
	if v.Loyalty >= ConquestBalance.MaxLoyalty {
		v.LoyaltyUpdatedAt = now
		return
	}
	v.Loyalty = math.Min(ConquestBalance.MaxLoyalty, v.Loyalty+points)
	v.LoyaltyUpdatedAt += int64(math.Round(points * step))
}

// LoyaltyAt is the loyalty at now without mutation (for views).
func LoyaltyAt(v *Village, now int64, worldSpeed float64) float64 {
	points := math.Max(0, math.Floor(float64(now-v.LoyaltyUpdatedAt)/msPerLoyaltyPoint(worldSpeed)))
	return math.Min(ConquestBalance.MaxLoyalty, v.Loyalty+points)
}

// ApplyNobles: every surviving noble drops loyalty by rng(20..35); at ≤ 0 the village is conquered (the other
// nobles are not used up). The change of owner is TransferVillage's.
func ApplyNobles(w World, target *Village, c *Command, nobles int) (loyaltyAfter float64, conquered bool) {
	rng := Mulberry32(HiddenSeed(w, c.ID, "loyalty"))
	conquerable := CanBeConquered(w, target)
	loyalty := target.Loyalty
	dropMin, dropMax := NobleDropRange(w.Config(), target.OwnerID != "")
	for range nobles {
		loyalty -= float64(RandInt(rng, dropMin, dropMax))
		if loyalty <= 0 {
			if !conquerable {
				loyalty = 1
				break
			}
			target.Loyalty = 0
			target.LoyaltyUpdatedAt = w.Now()
			return 0, true
		}
	}
	target.Loyalty = loyalty
	target.LoyaltyUpdatedAt = w.Now()
	return loyalty, false
}

// CanBeConquered: a player's last village is untouchable when the world says so — nobles then stop at 1.
func CanBeConquered(w World, v *Village) bool {
	if v.OwnerID == "" || w.Config().LastVillage == "canBeLost" {
		return true
	}
	owner := w.Player(PlayerID(v.OwnerID))
	// Abandoned accounts are not protected — inactive players are the nobles' first target.
	if owner != nil && owner.Status == StatusQuit {
		return true
	}
	return owner != nil && len(owner.VillageIDs) > 1
}

// autoFarmSync re-checks a human's auto-farm schedules after a village changed hands (set by farm.go).
var autoFarmSync func(w World, humanID PlayerID)

// dropTroopsAway: the previous owner's troops outside the village (marching, returning, supporting others) are
// lost — otherwise they would come back to the new owner.
func dropTroopsAway(w World, villageID string) {
	for _, c := range w.CommandsHome(villageID) {
		if c.Type == "trade" {
			continue
		}
		w.DeleteCommand(c.ID)
		w.Unschedule("cmd:" + c.ID)
	}
	for _, host := range w.SupportHosts(villageID) {
		delete(host.SupportFrom, villageID)
	}
}

var transferSpy func(from, to PlayerRef, conquest bool)

// SpyTransfers observes every change of village owner (measurements and tests); returns a restore func.
func SpyTransfers(fn func(from, to PlayerRef, conquest bool)) (restore func()) {
	prev := transferSpy
	transferSpy = fn
	return func() { transferSpy = prev }
}

// TransferVillage changes a village's owner — a conquest (byVillageID = where the nobles came from) or an
// abandonment (byVillageID ""). On conquest: 1–3 building levels lost, loyalty 25, research from zero. Always:
// queues cleared, foreign support sent home, the previous owner's troops away lost. A player left without
// villages: a human still playing starts again, a bot is eliminated.
func TransferVillage(w World, v *Village, newOwner PlayerRef, byVillageID string) {
	prevOwner := v.OwnerID
	if transferSpy != nil {
		transferSpy(prevOwner, newOwner, byVillageID != "")
	}
	if byVillageID != "" {
		rng := Mulberry32(HiddenSeed(w, v.ID, "conquest", w.Now()))
		v.Loyalty = ConquestBalance.LoyaltyAfterConquest
		v.LoyaltyUpdatedAt = w.Now()
		for _, b := range BuildingOrder {
			if v.Buildings[b] <= 0 {
				continue
			}
			loss := RandInt(rng, ConquestBalance.PlunderLevelsMin, ConquestBalance.PlunderLevelsMax)
			minLevel := 0
			if b == BMain {
				minLevel = 1
			}
			v.Buildings[b] = max(minLevel, v.Buildings[b]-loss)
		}
		v.Research = Units{}
	}
	v.ResearchQueue = []ResearchJob{}
	v.RecruitQueues = EmptyRecruitQueues()
	v.BuildQueue = []BuildJob{}
	v.BuildPlan = nil
	w.Unschedule("plan:" + v.ID)
	ScheduleBuildHead(w, v)
	ScheduleResearchHead(w, v)
	homes := make([]string, 0, len(v.SupportFrom))
	for homeID := range v.SupportFrom {
		homes = append(homes, homeID)
	}
	sort.Strings(homes)
	for _, homeID := range homes {
		if w.Village(homeID) != nil {
			CreateReturn(w, v.ID, homeID, v.SupportFrom[homeID], nil)
		}
	}
	v.SupportFrom = map[string]UnitCounts{}
	dropTroopsAway(w, v.ID)
	SetOwner(w, v, newOwner)
	recordTimeline(w, TimelineEntry{Kind: "owner", X: float64(v.X), Y: float64(v.Y), Owner: newOwner})
	if newOwner != "" && byVillageID != "" {
		RecordConquest(w, v, prevOwner, PlayerID(newOwner))
	}
	if newOwner != "" {
		defer checkDomination(w, PlayerID(newOwner))
	}
	newHuman := newOwner != "" && w.Human(PlayerID(newOwner)) != nil
	if newOwner == "" {
		v.Name = ""
	} else if newHuman {
		w.PutIntel(PlayerID(newOwner), v.ID, nil)
	}
	v.ResourcesUpdatedAt = w.Now()
	v.ConqueredAt = ptrTo(w.Now())
	RecomputePoints(v)

	if newHuman {
		h := w.Human(PlayerID(newOwner))
		h.Stats.VillagesConquered++
		h.Stats.PointsHistory = append(h.Stats.PointsHistory, [2]int64{w.Now(), int64(PlayerPoints(w, PlayerID(newOwner)))})
	}
	if prevOwner != "" {
		if h := w.Human(PlayerID(prevOwner)); h != nil {
			h.FarmAssistant.AutoFarmVillages = removeString(h.FarmAssistant.AutoFarmVillages, v.ID)
			for i := range h.Groups {
				h.Groups[i].VillageIDs = removeString(h.Groups[i].VillageIDs, v.ID)
			}
		}
	}
	if autoFarmSync != nil {
		for _, id := range []PlayerRef{prevOwner, newOwner} {
			if id != "" && w.Human(PlayerID(id)) != nil {
				autoFarmSync(w, PlayerID(id))
			}
		}
	}

	if prevOwner == "" {
		return
	}
	prev := w.Player(PlayerID(prevOwner))
	if b := w.Brain(PlayerID(prevOwner)); b != nil && byVillageID != "" {
		b.LastSessionStats.VillagesLost++
	}
	if prev != nil && len(prev.VillageIDs) == 0 {
		if prev.IsHuman {
			if prev.Status == StatusActive {
				_ = RespawnPlayer(w, prev.ID)
			}
		} else if prev.Status == StatusActive {
			prev.Status = StatusEliminated
		}
	}
}

func removeString(list []string, s string) []string {
	out := list[:0:0]
	for _, x := range list {
		if x != s {
			out = append(out, x)
		}
	}
	return out
}

// CoinCost is fixed.
func CoinCost() Resources { return ConquestBalance.CoinBaseCost }

type MintCheck struct {
	Cost     Resources
	Blockers []Blocker
}

func CheckMint(v *Village) MintCheck {
	cost := CoinCost()
	var blockers []Blocker
	if v.Buildings[BAcademy] < 1 {
		blockers = append(blockers, Blocker{"kind": "academy"})
	}
	if !HasResources(v.Resources, cost) {
		blockers = append(blockers, Blocker{"kind": "resources", "missing": missingResources(v.Resources, cost)})
	}
	return MintCheck{cost, blockers}
}

// MintCoin is immediate; the coins go to the village owner (shared by all their villages).
func MintCoin(w World, actor PlayerID, villageID string) error {
	if w.Config().NoCoins {
		return Rule("noCoins", nil)
	}
	v, err := ownsVillage(w, actor, villageID, "mint")
	if err != nil {
		return err
	}
	Touch(w, v)
	check := CheckMint(v)
	if len(check.Blockers) > 0 {
		return Blocked("mint", check.Blockers...)
	}
	SubtractResources(&v.Resources, check.Cost)
	w.Player(actor).Coins++
	if h := w.Human(actor); h != nil {
		h.Stats.CoinsMinted++
	}
	return nil
}

// NoblesNeeded is how many more nobles a loyalty needs (average drop 27.5).
func NoblesNeeded(loyalty float64) int {
	return max(1, int(math.Ceil(loyalty/ConquestBalance.NobleDropAvg)))
}

// NobleDropRange is the loyalty one nobleman takes: the world's own range against an owned village.
func NobleDropRange(cfg *WorldConfig, owned bool) (min, max int) {
	if owned && cfg.OwnedNobleDropMax > 0 {
		return cfg.OwnedNobleDropMin, cfg.OwnedNobleDropMax
	}
	return ConquestBalance.NobleDropMin, ConquestBalance.NobleDropMax
}

// NoblesNeededFor is NoblesNeeded with the world's drop range.
func NoblesNeededFor(cfg *WorldConfig, owned bool, loyalty float64) int {
	lo, hi := NobleDropRange(cfg, owned)
	return max(1, int(math.Ceil(loyalty/(float64(lo+hi)/2))))
}
