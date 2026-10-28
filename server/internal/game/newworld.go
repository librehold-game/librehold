package game

import (
	"math"
	"strconv"
)

// A new world: it starts with bots only, and humans join later.

// worldStartHooks run at the end of NewWorld (ranking and rumour schedules register here).
var worldStartHooks []func(w World)

// NewWorld creates the world's bots on the sunflower slots, or queues their ramp-up join events.
func NewWorld(w World) error {
	cfg := w.Config()
	if IsShort(cfg) {
		cfg.VillageTotal = villageTotal(w.Seed(), cfg)
	}
	slots := SpawnSlots(cfg.BotCount, cfg.MapSize, w.Seed())
	rampUp := cfg.BotArrival == "rampUp"
	for i := range cfg.BotCount {
		id := "bot_" + strconv.Itoa(i)
		jitter := 0.9 + 0.2*Mulberry32(HiddenSeed(w, "joinAt", i))()
		var joinAt int64
		status := StatusActive
		if rampUp {
			joinAt = int64(math.Round(float64(i+1) / float64(cfg.BotCount) * cfg.RampUpDays * float64(MsPerDay) * jitter))
			status = StatusPending
		}
		w.PutPlayer(&Player{
			ID: id, Name: BotName(w.Seed(), i), Color: PlayerColors[i%len(PlayerColors)], VillageIDs: []string{},
			BuildQueueSlots: 2, JoinAt: joinAt, Status: status, History: PointsHistory{Hourly: []int{}, Daily: []int{}},
		})
		w.PutBrain(id, worldBotBrain(w, id, cfg.Difficulty))
		if rampUp {
			w.Schedule("join:"+id, joinAt)
		} else {
			CreatePlayerVillage(w, id, slots[i].X, slots[i].Y)
		}
	}
	for _, hook := range worldStartHooks {
		hook(w)
	}
	scheduleEnd(w)
	scheduleSnapshot(w)
	return nil
}

// humanJoinHooks run after a human joined (the trader's first offer registers here).
var humanJoinHooks []func(w World, id PlayerID)

// JoinHuman spawns a human at the edge of the settled area in the chosen quadrant, with
// a new human's state, the starting guard and beginner protection. Human ids are `h<accountID>`.
func JoinHuman(w World, id PlayerID, name string, accountID int64, q Quadrant) (*Village, error) {
	if w.Player(id) != nil {
		return nil, Rule("alreadyMember", nil)
	}
	humans := 0
	for _, p := range w.Players() {
		if p.IsHuman {
			humans++
		}
	}
	if _, ok := quadrantStart[q]; !ok {
		q = QuadrantRandom
	}
	x, y, err := EdgeSpawnTile(w, Mulberry32(HashSeed(w.Seed(), "joinHuman", id)), q)
	if err != nil {
		return nil, err
	}
	colour := humans
	if IsShort(w.Config()) {
		// A short world shows everyone on one small map: humans take the colours after the bots'.
		colour += w.Config().BotCount
	}
	w.PutPlayer(&Player{
		ID: id, Name: name, IsHuman: true, Color: PlayerColors[colour%len(PlayerColors)], VillageIDs: []string{},
		BuildQueueSlots: 2, JoinAt: w.Now(), Status: StatusActive, History: PointsHistory{Hourly: []int{}, Daily: []int{}},
		AccountID: accountID,
	})
	h := NewHumanState()
	w.PutHuman(id, h)
	v := CreatePlayerVillage(w, id, x, y)
	if cfg := w.Config(); cfg.StartPreset == "" {
		for _, u := range UnitIDs {
			v.Units[u] += StartVillage.Units[u]
		}
	}
	h.Stats.PointsHistory = append(h.Stats.PointsHistory, [2]int64{w.Now(), int64(v.Points)})
	for _, hook := range humanJoinHooks {
		hook(w, id)
	}
	return v, nil
}
