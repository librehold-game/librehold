package game

import "math"

// Recruitment, research and nobles.

func init() { registerHandler("research", onResearchEvent) }

// ---------- research ----------

// UnitNeedsResearch: units whose requirements include the smithy must be unlocked there; the others get
// level 1 for free.
func UnitNeedsResearch(u UnitID) bool {
	for _, r := range UnitDefs[u].Requirements {
		if r.Building == BSmithy {
			return true
		}
	}
	return false
}

func EffectiveResearchLevel(v *Village, u UnitID) int {
	lvl := v.Research[u]
	if UnitNeedsResearch(u) {
		return lvl
	}
	return max(1, lvl)
}

func ResearchBonus(level int) float64 {
	return UnitBalance.ResearchBonus[max(0, min(level, ResearchBalance.MaxLevel))]
}

func ResearchCost(u UnitID, level int) Resources {
	base := ResearchUnlockCost[u]
	mult := 0.0
	if level >= 0 && level < len(UnitBalance.ResearchCostMultiplier) {
		mult = UnitBalance.ResearchCostMultiplier[level]
	}
	return Resources{base.Wood * mult, base.Clay * mult, base.Iron * mult}
}

func ResearchTimeMs(u UnitID, level int, worldSpeed float64) int64 {
	c := ResearchCost(u, level)
	ms := ((c.Wood + c.Clay + c.Iron) * UnitBalance.ResearchTimePerResourceSec * 1000) / worldSpeed
	return max(ResearchBalance.MinResearchTimeMs, int64(math.Round(ms)))
}

type ResearchCheck struct {
	CurrentLevel int
	TargetLevel  int
	Cost         Resources
	DurationMs   int64
	Blockers     []Blocker
}

func missingResources(have, cost Resources) Resources {
	return Resources{
		Wood: math.Max(0, cost.Wood-have.Wood),
		Clay: math.Max(0, cost.Clay-have.Clay),
		Iron: math.Max(0, cost.Iron-have.Iron),
	}
}

func CheckResearch(w World, v *Village, u UnitID) ResearchCheck {
	current := EffectiveResearchLevel(v, u)
	targetLevel := current + 1
	for _, j := range v.ResearchQueue {
		if j.Unit == u {
			targetLevel = j.TargetLevel + 1
			break
		}
	}
	lvl := min(targetLevel, ResearchBalance.MaxLevel)
	cost := ResearchCost(u, lvl)
	var blockers []Blocker
	if targetLevel > ResearchBalance.MaxLevel {
		blockers = append(blockers, Blocker{"kind": "maxLevel"})
	}
	if len(v.ResearchQueue) > 0 {
		blockers = append(blockers, Blocker{"kind": "busy"})
	}
	var missing []BuildingRequirement
	for _, r := range UnitDefs[u].Requirements {
		if v.Buildings[r.Building] < r.Level {
			missing = append(missing, r)
		}
	}
	if v.Buildings[BSmithy] < 1 {
		missing = append(missing, BuildingRequirement{BSmithy, 1})
	}
	if len(missing) > 0 {
		blockers = append(blockers, Blocker{"kind": "requirements", "missing": missing})
	}
	if !HasResources(v.Resources, cost) {
		blockers = append(blockers, Blocker{"kind": "resources", "missing": missingResources(v.Resources, cost)})
	}
	return ResearchCheck{current, targetLevel, cost, ResearchTimeMs(u, lvl, w.Config().WorldSpeed), blockers}
}

func EnqueueResearch(w World, actor PlayerID, villageID string, u UnitID) (*ResearchJob, error) {
	v, err := ownsVillage(w, actor, villageID, "research")
	if err != nil {
		return nil, err
	}
	Touch(w, v)
	check := CheckResearch(w, v, u)
	if len(check.Blockers) > 0 {
		return nil, Blocked("research", check.Blockers...)
	}
	SubtractResources(&v.Resources, check.Cost)
	job := ResearchJob{ID: w.NewID("r"), Unit: u, TargetLevel: check.TargetLevel, StartedAt: w.Now(),
		CompletesAt: w.Now() + check.DurationMs}
	v.ResearchQueue = append(v.ResearchQueue, job)
	ScheduleResearchHead(w, v)
	return &job, nil
}

// CancelResearch refunds 100 % of the cost.
func CancelResearch(w World, actor PlayerID, villageID, jobID string) (Resources, error) {
	v, err := ownsVillage(w, actor, villageID, "cancel-research")
	if err != nil {
		return Resources{}, err
	}
	Touch(w, v)
	for i, job := range v.ResearchQueue {
		if job.ID != jobID {
			continue
		}
		cost := ResearchCost(job.Unit, job.TargetLevel)
		v.ResearchQueue = append(v.ResearchQueue[:i:i], v.ResearchQueue[i+1:]...)
		ScheduleResearchHead(w, v)
		v.Resources.Wood += cost.Wood
		v.Resources.Clay += cost.Clay
		v.Resources.Iron += cost.Iron
		return cost, nil
	}
	return Resources{}, Rule("noJob", map[string]any{"id": jobID})
}

func ScheduleResearchHead(w World, v *Village) {
	key := "research:" + v.ID
	if len(v.ResearchQueue) > 0 {
		w.Schedule(key, v.ResearchQueue[0].CompletesAt)
	} else {
		w.Unschedule(key)
	}
}

func onResearchEvent(w World, villageID string) error {
	v := w.Village(villageID)
	if v == nil || len(v.ResearchQueue) == 0 {
		return nil
	}
	job := v.ResearchQueue[0]
	if job.CompletesAt > w.Now() {
		ScheduleResearchHead(w, v)
		return nil
	}
	v.Research[job.Unit] = job.TargetLevel
	v.ResearchQueue = append([]ResearchJob{}, v.ResearchQueue[1:]...)
	if _, h := humanOf(w, v); h != nil {
		h.Stats.ResearchCompleted++
	}
	ScheduleResearchHead(w, v)
	return nil
}

// ---------- recruitment ----------

// RecruitTimeMs is the time of one unit; with v == nil there is no village bonus and no side multiplier.
func RecruitTimeMs(u UnitID, buildingLevel int, cfg *WorldConfig, v *Village) int64 {
	def := &UnitDefs[u]
	side := 1.0
	if v != nil {
		side = sideModifiers(cfg, v.OwnerID).Recruitment
	}
	ms := (def.BaseRecruitSec * math.Pow(UnitBalance.RecruitTimeFactor, float64(max(1, buildingLevel)-1)) * 1000) /
		(cfg.WorldSpeed * side)
	if v != nil && v.Bonus != nil {
		if v.Bonus.Kind == "infantry" && def.Building == BBarracks || v.Bonus.Kind == "cavalry" && def.Building == BStable {
			ms *= BonusBalance.RecruitTimeFactor
		}
	}
	return max(UnitBalance.MinRecruitTimeMs, int64(math.Round(ms)))
}

func UnitCost(u UnitID, count int) Resources { return UnitCostIn(nil, u, count) }

// UnitCostIn is UnitCost with the world's nobleman price when it sets one.
func UnitCostIn(cfg *WorldConfig, u UnitID, count int) Resources {
	c := UnitDefs[u].Cost
	if u == UNoble && cfg != nil && cfg.NobleCost != (Resources{}) {
		c = cfg.NobleCost
	}
	n := float64(count)
	return Resources{c.Wood * n, c.Clay * n, c.Iron * n}
}

// UnitCountIncludingQueued counts a unit at home plus in the recruitment queues (for "1 paladin" limits).
func UnitCountIncludingQueued(v *Village, u UnitID) int {
	n := v.Units[u]
	for _, b := range RecruitBuildings {
		for _, j := range *v.RecruitQueues.Q(b) {
			if j.Unit == u {
				n += j.Total - j.Done
			}
		}
	}
	return n
}

// NobleCountOf counts the village's nobles: present, queued and away.
func NobleCountOf(w World, v *Village) int {
	return UnitCountIncludingQueued(v, UNoble) + UnitsAway(w, v.ID)[UNoble]
}

// PaladinCountOf counts the village's paladins: present, queued and away.
func PaladinCountOf(w World, v *Village) int {
	return UnitCountIncludingQueued(v, UPaladin) + UnitsAway(w, v.ID)[UPaladin]
}

func TotalNoblesOwned(w World, id PlayerID) int {
	n := 0
	for _, v := range PlayerVillages(w, id) {
		n += NobleCountOf(w, v)
	}
	return n
}

// CoinsNeededFor is the coins a player needs for `count` more nobles (each one dearer by one).
func CoinsNeededFor(w World, id PlayerID, count int) int {
	owned := TotalNoblesOwned(w, id)
	total := 0
	for i := range count {
		total += CoinsForNextNoble(w, id, owned+i)
	}
	return total
}

func NobleLimit(cfg *WorldConfig, v *Village) int {
	if v.Buildings[BAcademy] <= 0 {
		return 0
	}
	return v.Buildings[BAcademy] + cfg.AcademyExtraNobles
}

type RecruitCheck struct {
	Cost          Resources
	Pop           int
	PerUnitMs     int64
	MaxAffordable int // most units orderable with the current resources and population (0 if blocked)
	Blockers      []Blocker
}

func MaxRecruitable(w World, v *Village, u UnitID) int {
	def := &UnitDefs[u]
	free := VillageFarmCapacity(v) - PopulationUsed(w, v)
	n := math.MaxInt
	if def.Pop > 0 {
		n = int(math.Floor(float64(free) / float64(def.Pop)))
	}
	unitCost := UnitCostIn(w.Config(), u, 1)
	for _, r := range ResourceIDs {
		if c := unitCost.Get(r); c > 0 {
			n = min(n, int(math.Floor(v.Resources.Get(r)/c)))
		}
	}
	n = max(0, n)
	if u == UPaladin {
		n = min(n, max(0, 1-PaladinCountOf(w, v)))
	}
	if u == UNoble {
		n = min(n, max(0, NobleLimit(w.Config(), v)-NobleCountOf(w, v)))
		if owner := OwnerOf(w, v); owner != nil && !w.Config().NoCoins {
			affordable := 0
			for affordable < n && CoinsNeededFor(w, owner.ID, affordable+1) <= owner.Coins {
				affordable++
			}
			n = affordable
		}
	}
	if n == math.MaxInt {
		return 0
	}
	return n
}

func CheckRecruit(w World, v *Village, u UnitID, count int) RecruitCheck {
	def := &UnitDefs[u]
	cost := UnitCostIn(w.Config(), u, count)
	pop := def.Pop * count
	perUnitMs := RecruitTimeMs(u, v.Buildings[def.Building], w.Config(), v)
	var blockers []Blocker

	var missing []BuildingRequirement
	for _, r := range def.Requirements {
		if v.Buildings[r.Building] < r.Level {
			missing = append(missing, r)
		}
	}
	if len(missing) > 0 {
		blockers = append(blockers, Blocker{"kind": "requirements", "missing": missing})
	}
	if EffectiveResearchLevel(v, u) < 1 {
		blockers = append(blockers, Blocker{"kind": "research"})
	}
	if count < 1 {
		blockers = append(blockers, Blocker{"kind": "count"})
	}
	if u == UPaladin && PaladinCountOf(w, v)+count > 1 {
		blockers = append(blockers, Blocker{"kind": "limit", "max": 1})
	}
	if u == UNoble {
		if NobleCountOf(w, v)+count > NobleLimit(w.Config(), v) {
			blockers = append(blockers, Blocker{"kind": "limit", "max": NobleLimit(w.Config(), v)})
		}
		if owner := OwnerOf(w, v); owner != nil && !w.Config().NoCoins {
			if needed := CoinsNeededFor(w, owner.ID, count); owner.Coins < needed {
				blockers = append(blockers, Blocker{"kind": "coins", "have": owner.Coins, "needed": needed})
			}
		}
	}
	if !HasResources(v.Resources, cost) {
		blockers = append(blockers, Blocker{"kind": "resources", "missing": missingResources(v.Resources, cost)})
	}
	if free := VillageFarmCapacity(v) - PopulationUsed(w, v); pop > free {
		blockers = append(blockers, Blocker{"kind": "population", "needed": pop, "free": free})
	}
	hardBlocked := false
	for _, b := range blockers {
		if k := b.Kind(); k == "requirements" || k == "research" {
			hardBlocked = true
		}
	}
	maxAffordable := 0
	if !hardBlocked {
		maxAffordable = MaxRecruitable(w, v, u)
	}
	return RecruitCheck{cost, pop, perUnitMs, maxAffordable, blockers}
}

func rescheduleRecruitQueue(queue []RecruitJob, now int64) {
	cursor := now
	for i := range queue {
		job := &queue[i]
		if i == 0 && job.StartedAt <= now {
			cursor = job.CompletesAt
			continue
		}
		job.StartedAt = cursor
		job.CompletesAt = job.StartedAt + int64(job.Total-job.Done)*job.PerUnitMs
		cursor = job.CompletesAt
	}
}

func EnqueueRecruit(w World, actor PlayerID, villageID string, u UnitID, count int) (*RecruitJob, error) {
	v, err := ownsVillage(w, actor, villageID, "recruit")
	if err != nil {
		return nil, err
	}
	Touch(w, v)
	check := CheckRecruit(w, v, u, count)
	if len(check.Blockers) > 0 {
		return nil, Blocked("recruit", check.Blockers...)
	}
	SubtractResources(&v.Resources, check.Cost)
	if owner := OwnerOf(w, v); u == UNoble && owner != nil {
		if !w.Config().NoCoins {
			owner.Coins -= CoinsNeededFor(w, owner.ID, count)
		}
		owner.NoblesTrained += count
	}
	queue := v.RecruitQueues.Q(UnitDefs[u].Building)
	*queue = append(*queue, RecruitJob{
		ID: w.NewID("q"), Unit: u, Total: count, PerUnitMs: check.PerUnitMs,
		StartedAt: w.Now(), CompletesAt: w.Now() + int64(count)*check.PerUnitMs,
	})
	rescheduleRecruitQueue(*queue, w.Now())
	job := (*queue)[len(*queue)-1]
	return &job, nil
}

// CancelRecruit refunds 100 % of the units not yet produced (and, for nobles, their coins).
func CancelRecruit(w World, actor PlayerID, villageID, jobID string) (Resources, error) {
	v, err := ownsVillage(w, actor, villageID, "cancel-recruit")
	if err != nil {
		return Resources{}, err
	}
	Touch(w, v)
	for _, b := range RecruitBuildings {
		queue := v.RecruitQueues.Q(b)
		for i, job := range *queue {
			if job.ID != jobID {
				continue
			}
			undone := job.Total - job.Done
			refund := UnitCostIn(w.Config(), job.Unit, undone)
			*queue = append((*queue)[:i:i], (*queue)[i+1:]...)
			if owner := OwnerOf(w, v); job.Unit == UNoble && owner != nil && !w.Config().NoCoins {
				// Coins come back at the price of the player's "last" nobles.
				owner.Coins += CoinsNeededFor(w, owner.ID, undone)
			}
			v.Resources.Wood += refund.Wood
			v.Resources.Clay += refund.Clay
			v.Resources.Iron += refund.Iron
			rescheduleRecruitQueue(*queue, w.Now())
			return refund, nil
		}
	}
	return Resources{}, Rule("noJob", map[string]any{"id": jobID})
}

// SettleRecruitQueues settles recruitment lazily: the units present at `now` are a function of the queue.
func SettleRecruitQueues(v *Village, now int64) {
	for _, b := range RecruitBuildings {
		queue := v.RecruitQueues.Q(b)
		for len(*queue) > 0 {
			job := &(*queue)[0]
			if now < job.StartedAt {
				break
			}
			done := min(job.Total, int((now-job.StartedAt)/job.PerUnitMs))
			if done > job.Done {
				v.Units[job.Unit] += done - job.Done
				job.Done = done
			}
			if job.Done < job.Total {
				break
			}
			*queue = append([]RecruitJob{}, (*queue)[1:]...)
		}
	}
}

// UnitsForBuilding lists the units recruited in a building, in table order.
func UnitsForBuilding(b BuildingID) []UnitID {
	var out []UnitID
	for _, u := range UnitIDs {
		if UnitDefs[u].Building == b {
			out = append(out, u)
		}
	}
	return out
}
