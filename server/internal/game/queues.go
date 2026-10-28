package game

import "math"

// Build queues, with an ownership check on every order.

func init() { registerHandler("build", onBuildEvent) }

// EffectiveBuildingLevels are the levels after every queued job (build +1, demolish −1).
func EffectiveBuildingLevels(v *Village) Buildings {
	levels := v.Buildings
	for _, job := range v.BuildQueue {
		levels[job.Building] = job.TargetLevel
	}
	return levels
}

// levelBefore is a building's level after the jobs queued before index.
func levelBefore(v *Village, b BuildingID, beforeIndex int) int {
	lvl := v.Buildings[b]
	for i := 0; i < beforeIndex && i < len(v.BuildQueue); i++ {
		if v.BuildQueue[i].Building == b {
			lvl = v.BuildQueue[i].TargetLevel
		}
	}
	return lvl
}

func DemolishTimeMs(b BuildingID, level, mainLevel int, worldSpeed float64) int64 {
	return max(BuildingBalance.MinBuildTimeMs, int64(math.Round(float64(BuildTimeMs(b, level, mainLevel, worldSpeed))/2)))
}

func jobDurationMs(v *Village, job *BuildJob, index int, cfg *WorldConfig) int64 {
	mainLevel := levelBefore(v, BMain, index)
	var base int64
	if job.Kind == "demolish" {
		base = DemolishTimeMs(job.Building, job.TargetLevel+1, mainLevel, cfg.WorldSpeed)
	} else {
		base = BuildTimeMs(job.Building, job.TargetLevel, mainLevel, cfg.WorldSpeed)
	}
	return int64(math.Round(float64(base) / sideModifiers(cfg, v.OwnerID).Building))
}

// RescheduleBuildQueue recomputes the queue's timetable: a started first job keeps its times, the next ones
// start when the previous one ends.
func RescheduleBuildQueue(v *Village, now int64, cfg *WorldConfig) {
	cursor := now
	for i := range v.BuildQueue {
		job := &v.BuildQueue[i]
		if i == 0 && job.StartedAt <= now && job.CompletesAt > now {
			cursor = job.CompletesAt
			continue
		}
		job.StartedAt = max(cursor, now)
		job.CompletesAt = job.StartedAt + jobDurationMs(v, job, i, cfg)
		cursor = job.CompletesAt
	}
}

type BuildCheck struct {
	TargetLevel int
	Cost        Resources
	PopDelta    int
	DurationMs  int64
	Blockers    []Blocker
}

// CheckBuild says whether the next level of a building can be queued (no mutation).
func CheckBuild(w World, v *Village, b BuildingID) BuildCheck {
	def := &BuildingDefs[b]
	cfg := w.Config()
	levels := EffectiveBuildingLevels(v)
	targetLevel := levels[b] + 1
	cost := BuildCost(b, min(targetLevel, def.MaxLevel))
	popDelta := BuildingPopulation(b, targetLevel) - BuildingPopulation(b, targetLevel-1)
	durationMs := int64(math.Round(float64(BuildTimeMs(b, min(targetLevel, def.MaxLevel), levels[BMain], cfg.WorldSpeed)) /
		sideModifiers(cfg, v.OwnerID).Building))
	var blockers []Blocker

	if targetLevel > def.MaxLevel {
		blockers = append(blockers, Blocker{"kind": "maxLevel"})
	}
	if slots := BuildQueueSlotsFor(w, OwnerOf(w, v)); len(v.BuildQueue) >= slots {
		blockers = append(blockers, Blocker{"kind": "queueFull", "slots": slots})
	}
	if !MeetsRequirements(levels, b) {
		var missing []BuildingRequirement
		for _, r := range def.Requirements {
			if levels[r.Building] < r.Level {
				missing = append(missing, r)
			}
		}
		blockers = append(blockers, Blocker{"kind": "requirements", "missing": missing})
	}
	if !HasResources(v.Resources, cost) {
		blockers = append(blockers, Blocker{"kind": "resources", "missing": Resources{
			Wood: math.Max(0, cost.Wood-v.Resources.Wood),
			Clay: math.Max(0, cost.Clay-v.Resources.Clay),
			Iron: math.Max(0, cost.Iron-v.Resources.Iron),
		}})
	}
	free := VillageFarmCapacity(v) - PopulationUsed(w, v)
	if popDelta > 0 && popDelta > free {
		blockers = append(blockers, Blocker{"kind": "population", "needed": popDelta, "free": free})
	}
	return BuildCheck{TargetLevel: targetLevel, Cost: cost, PopDelta: popDelta, DurationMs: durationMs, Blockers: blockers}
}

func EnqueueBuild(w World, actor PlayerID, villageID string, b BuildingID) (*BuildJob, error) {
	v, err := ownsVillage(w, actor, villageID, "build")
	if err != nil {
		return nil, err
	}
	Touch(w, v)
	check := CheckBuild(w, v, b)
	if len(check.Blockers) > 0 {
		return nil, Blocked("build", check.Blockers...)
	}
	SubtractResources(&v.Resources, check.Cost)
	v.BuildQueue = append(v.BuildQueue, BuildJob{
		ID: w.NewID("b"), Building: b, TargetLevel: check.TargetLevel, Kind: "build",
		StartedAt: w.Now(), CompletesAt: w.Now(),
	})
	RescheduleBuildQueue(v, w.Now(), w.Config())
	ScheduleBuildHead(w, v)
	job := v.BuildQueue[len(v.BuildQueue)-1]
	return &job, nil
}

func CanDemolish(w World, v *Village, b BuildingID) bool {
	levels := EffectiveBuildingLevels(v)
	minLevel := 0
	if b == BMain {
		minLevel = 1
	}
	return levels[b] > minLevel && len(v.BuildQueue) < BuildQueueSlotsFor(w, OwnerOf(w, v))
}

// EnqueueDemolish: half the build time of the level removed, no refund, frees population when done.
func EnqueueDemolish(w World, actor PlayerID, villageID string, b BuildingID) (*BuildJob, error) {
	v, err := ownsVillage(w, actor, villageID, "demolish")
	if err != nil {
		return nil, err
	}
	Touch(w, v)
	if !CanDemolish(w, v, b) {
		return nil, Rule("cannotDemolish", map[string]any{"building": b.String()})
	}
	levels := EffectiveBuildingLevels(v)
	v.BuildQueue = append(v.BuildQueue, BuildJob{
		ID: w.NewID("b"), Building: b, TargetLevel: levels[b] - 1, Kind: "demolish",
		StartedAt: w.Now(), CompletesAt: w.Now(),
	})
	RescheduleBuildQueue(v, w.Now(), w.Config())
	ScheduleBuildHead(w, v)
	job := v.BuildQueue[len(v.BuildQueue)-1]
	return &job, nil
}

// CancelBuild refunds 90 % of a started job or 100 % of a waiting one; later jobs of that building drop a level.
func CancelBuild(w World, actor PlayerID, villageID, jobID string) (Resources, error) {
	v, err := ownsVillage(w, actor, villageID, "cancel-build")
	if err != nil {
		return Resources{}, err
	}
	Touch(w, v)
	index := -1
	for i, j := range v.BuildQueue {
		if j.ID == jobID {
			index = i
		}
	}
	if index < 0 {
		return Resources{}, Rule("noJob", map[string]any{"id": jobID})
	}
	job := v.BuildQueue[index]
	started := index == 0 && job.StartedAt <= w.Now()
	var refund Resources
	if job.Kind == "build" {
		cost := BuildCost(job.Building, job.TargetLevel)
		ratio := BuildingBalance.CancelRefundQueued
		if started {
			ratio = BuildingBalance.CancelRefundStarted
		}
		refund = Resources{math.Floor(cost.Wood * ratio), math.Floor(cost.Clay * ratio), math.Floor(cost.Iron * ratio)}
	}
	v.BuildQueue = append(v.BuildQueue[:index:index], v.BuildQueue[index+1:]...)
	delta := 1
	if job.Kind == "build" {
		delta = -1
	}
	for i := index; i < len(v.BuildQueue); i++ {
		if v.BuildQueue[i].Building == job.Building {
			v.BuildQueue[i].TargetLevel += delta
		}
	}
	v.Resources.Wood += refund.Wood
	v.Resources.Clay += refund.Clay
	v.Resources.Iron += refund.Iron
	RescheduleBuildQueue(v, w.Now(), w.Config())
	ScheduleBuildHead(w, v)
	return refund, nil
}

// ScheduleBuildHead schedules `build:<village>` at the end of the first job, or cancels it for an empty queue.
func ScheduleBuildHead(w World, v *Village) {
	key := "build:" + v.ID
	if len(v.BuildQueue) > 0 {
		w.Schedule(key, v.BuildQueue[0].CompletesAt)
	} else {
		w.Unschedule(key)
	}
}

// planFeeder sends the next build plan step to the queue (set by plan.go).
var planFeeder func(w World, villageID string)

// onBuildEvent finishes the first job if its time has come.
func onBuildEvent(w World, villageID string) error {
	v := w.Village(villageID)
	if v == nil || len(v.BuildQueue) == 0 {
		return nil
	}
	job := v.BuildQueue[0]
	if job.CompletesAt > w.Now() {
		ScheduleBuildHead(w, v)
		return nil
	}
	// Production up to the completion counts at the old level.
	Touch(w, v)
	v.Buildings[job.Building] = job.TargetLevel
	v.BuildQueue = append([]BuildJob{}, v.BuildQueue[1:]...)
	RecomputePoints(v)
	if id, h := humanOf(w, v); h != nil {
		SamplePoints(w, id)
	}
	RescheduleBuildQueue(v, w.Now(), w.Config())
	ScheduleBuildHead(w, v)
	if len(v.BuildPlan) > 0 && planFeeder != nil {
		planFeeder(w, villageID)
	}
	return nil
}
