package game

import (
	"math"
	"slices"
	"sort"
	"strconv"
	"time"
)

// Rumours at 08:00 local (Warsaw) time, from public facts only.

const (
	gossipMaxPerDay = 8
	gossipMorning   = 8 * 60
)

func init() {
	registerHandler("gossip", onGossipEvent)
	worldStartHooks = append(worldStartHooks, ScheduleGossip)
}

func ScheduleGossip(w World) {
	today := LocalDayIndex(w, w.Now())
	at := WorldTimeOf(w, today, gossipMorning)
	if at <= w.Now() {
		at = WorldTimeOf(w, today+1, gossipMorning)
	}
	w.Schedule("gossip", at)
}

// Variants per rumour kind; the client words them as gossip.<kind>.<variant>.
var gossipVariants = map[string]int{"conquest": 3, "topConqueror": 2, "inactive": 3, "topFarmer": 2, "topAttacker": 2}

type gossipItem struct {
	key    string
	params map[string]any
}

func pickGossip(rng func() float64, kind string, params map[string]any) gossipItem {
	n := gossipVariants[kind]
	return gossipItem{"gossip." + kind + "." + strconv.Itoa(int(math.Floor(rng()*float64(n)))), params}
}

// WriteGossip writes the day's rumours from public facts (takeovers, player statuses, daily rankings): takeovers
// of the last day, on Monday the week's top conqueror, who disappeared, yesterday's farmer and attacker. A few
// variants each, chosen from the world seed and day — the same day gives the same texts. At most 8 a day.
func WriteGossip(w World) {
	day := LocalDayIndex(w, w.Now())
	rng := Mulberry32(HiddenSeed(w, "gossip", day))
	// An empty name is the barbarians; the client words it.
	name := func(id PlayerRef) string {
		if id == "" {
			return ""
		}
		if p := w.Player(PlayerID(id)); p != nil {
			return p.Name
		}
		return "?"
	}
	var items []gossipItem
	for _, c := range w.ConquestsSince(w.Now() - MsPerDay + 1) {
		village := ""
		if v := w.Village(c.VillageID); v != nil {
			village = v.Name
		}
		items = append(items, pickGossip(rng, "conquest", map[string]any{"a": name(Ref(c.ToID)), "b": name(c.FromID),
			"village": village, "x": c.X, "y": c.Y}))
	}
	if time.UnixMilli(day*MsPerDay).UTC().Weekday() == time.Monday {
		counts := map[PlayerID]int{}
		var order []PlayerID
		for _, c := range w.ConquestsSince(w.Now() - 7*MsPerDay + 1) {
			if counts[c.ToID] == 0 {
				order = append(order, c.ToID)
			}
			counts[c.ToID]++
		}
		sort.SliceStable(order, func(i, j int) bool { return counts[order[i]] > counts[order[j]] })
		if len(order) > 0 {
			items = append(items, pickGossip(rng, "topConqueror", map[string]any{"a": name(Ref(order[0])), "n": counts[order[0]]}))
		}
	}
	for _, p := range w.Players() {
		if p.Status != StatusQuit || p.QuitAt == nil {
			continue
		}
		if *p.QuitAt > w.Now()-3*MsPerDay && *p.QuitAt <= w.Now()-2*MsPerDay {
			items = append(items, pickGossip(rng, "inactive", map[string]any{"a": p.Name}))
		}
	}
	type entry struct {
		id PlayerID
		d  DailyCounts
	}
	var yesterday []entry
	for _, p := range w.Players() {
		for _, d := range p.Daily {
			if d.Day == day-1 {
				yesterday = append(yesterday, entry{p.ID, d})
			}
		}
	}
	best := func(key func(DailyCounts) float64) (entry, bool) {
		if len(yesterday) == 0 {
			return entry{}, false
		}
		sorted := slices.Clone(yesterday)
		sort.SliceStable(sorted, func(i, j int) bool { return key(sorted[i].d) > key(sorted[j].d) })
		return sorted[0], true
	}
	if f, ok := best(func(d DailyCounts) float64 { return d.Farm }); ok && f.d.Farm > 0 {
		items = append(items, pickGossip(rng, "topFarmer", map[string]any{"a": name(Ref(f.id)), "n": math.Round(f.d.Farm)}))
	}
	if a, ok := best(func(d DailyCounts) float64 { return d.Att }); ok && a.d.Att > 0 {
		items = append(items, pickGossip(rng, "topAttacker", map[string]any{"a": name(Ref(a.id)), "n": math.Round(a.d.Att)}))
	}
	for i, it := range items {
		if i >= gossipMaxPerDay {
			break
		}
		AddNewsItem(w, it.key, it.params, "", false, "")
	}
}

func onGossipEvent(w World, _ string) error {
	WriteGossip(w)
	ScheduleGossip(w)
	return nil
}
