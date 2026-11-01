// Command botpace plays a long world (speed 4, 100 bots, a 200-tile map) with bots only and prints how fast the
// leading bots grow at a few checkpoints, so bot changes can be compared run against run.
//
//	go run ./cmd/botpace -seeds 1,2,3,4,5,6,7,8
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"os"
	"slices"
	"sort"
	"strconv"
	"strings"

	"librehold/internal/game"
	"librehold/internal/game/gametest"
)

const longWorldConfig = `{"mapSize": 200, "botCount": 100, "botStance": "normal", "nobleCost": {"clay": 0, "iron": 0, "wood": 0},
 "unitSpeed": 1, "botArrival": "allAtOnce", "difficulty": "normal", "rampUpDays": 14, "worldSpeed": 4,
 "lastVillage": "canBeLost", "sightRadius": 1, "battleLosses": "normal",
 "botModifiers": {"building": 1, "resources": 1, "recruitment": 1},
 "humanModifiers": {"building": 1, "resources": 1, "recruitment": 1}, "barbarianDensity": "normal",
 "beginnerProtectionDays": 5}`

const startMs = 1793209655024

type row struct {
	hour                                  int
	top1, top5, median, strongMed         float64
	mediumMed, weakMed                    float64
	mines, main, farm, hide, wall, stable float64
	lightCav, homeInf, idleQueue, idleRes float64
	freePop, popBlocked                   float64
	storage, storageHours, storageFull    float64
	lootShare                             float64
}

func main() {
	seedsFlag := flag.String("seeds", "1,2,3", "comma-separated seeds")
	hoursFlag := flag.String("hours", "24,48,66,95", "checkpoints in hours")
	debugFlag := flag.Bool("debug", false, "print the top strong bots' state after each seed")
	storageHours := flag.Float64("storage-hours", 0, "override EconomyPlay.StorageHours of the strong and medium bots")
	flag.Parse()
	if *storageHours > 0 {
		for tier, plays := range game.EconomyPlays {
			plays.Economy.StorageHours, plays.Farming.StorageHours = *storageHours, *storageHours
			game.EconomyPlays[tier] = plays
		}
	}
	cfg := game.DefaultWorldConfig
	if err := json.Unmarshal([]byte(longWorldConfig), &cfg); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	var hours []int
	for _, h := range strings.Split(*hoursFlag, ",") {
		n, err := strconv.Atoi(h)
		if err != nil || n <= 0 {
			fmt.Fprintf(os.Stderr, "-hours: %q is not a positive number of hours\n", h)
			os.Exit(2)
		}
		hours = append(hours, n)
	}
	var seeds []uint32
	for _, s := range strings.Split(*seedsFlag, ",") {
		seed, err := strconv.ParseUint(s, 10, 32)
		if err != nil {
			fmt.Fprintf(os.Stderr, "-seeds: %q is not a seed (0 to %d)\n", s, uint32(math.MaxUint32))
			os.Exit(2)
		}
		seeds = append(seeds, uint32(seed))
	}
	sums := map[int][]row{}
	for _, seed := range seeds {
		w := gametest.NewMemWorld(seed, cfg, startMs)
		if err := game.NewWorld(w); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		for _, h := range hours {
			if err := gametest.Advance(w, int64(h)*game.MsPerHour); err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
			sums[h] = append(sums[h], measure(w, h))
		}
		if *debugFlag {
			dump(w)
		}
	}
	fmt.Println("hour |  top1  top5 | median: all strong medium weak | strong top-10 avg: mines main farm hide wall stable | LC  home-inf | empty queue  idle res | free pop  pop-blocked | storage  hours  full | loot/prod")
	for _, h := range hours {
		r := mean(sums[h])
		fmt.Printf("%4d | %5.0f %5.0f | %11.0f %6.0f %6.0f %4.0f | %18.1f %4.1f %4.1f %4.1f %4.1f %6.1f | %3.0f %8.0f | %10.0f%% %9.0f | %7.0f%% %10.0f%% | %7.1f %6.1f %4.0f%% | %8.0f%%\n", h, r.top1, r.top5,
			r.median, r.strongMed, r.mediumMed, r.weakMed, r.mines, r.main, r.farm, r.hide, r.wall, r.stable, r.lightCav, r.homeInf, r.idleQueue*100, r.idleRes,
			r.freePop*100, r.popBlocked*100, r.storage, r.storageHours, r.storageFull*100, r.lootShare*100)
	}
}

func measure(w *gametest.MemWorld, hour int) row {
	points := w.PointsByOwner()
	type bot struct {
		id     game.PlayerID
		pts    float64
		strong bool
		tier   game.BotTier
	}
	var bots []bot
	for _, p := range w.Players() {
		if b := w.Brain(p.ID); b != nil && p.Status == game.StatusActive {
			bots = append(bots, bot{p.ID, float64(points[p.ID]), b.Skill >= game.BotConfig.StrongSkill, game.TierOf(b.Skill)})
		}
	}
	sort.Slice(bots, func(i, j int) bool { return bots[i].pts > bots[j].pts })
	r := row{hour: hour, top1: bots[0].pts}
	var all, strong, medium, weak []float64
	for i, b := range bots {
		all = append(all, b.pts)
		switch b.tier {
		case game.TierMedium:
			medium = append(medium, b.pts)
		case game.TierWeak:
			weak = append(weak, b.pts)
		}
		if b.strong {
			strong = append(strong, b.pts)
		}
		if i < 5 {
			r.top5 += b.pts / 5
		}
	}
	r.median, r.strongMed, r.mediumMed, r.weakMed = median(all), median(strong), median(medium), median(weak)
	n := 0.0
	for _, b := range bots {
		if n == 10 {
			break
		}
		if !b.strong {
			continue
		}
		n++
		p := w.Player(b.id)
		v := w.Village(p.VillageIDs[0])
		bl := v.Buildings
		r.mines += float64(bl[game.BWood]+bl[game.BClay]+bl[game.BIron]) / 3
		r.main += float64(bl[game.BMain])
		r.farm += float64(bl[game.BFarm])
		r.hide += float64(bl[game.BHide])
		r.wall += float64(bl[game.BWall])
		r.stable += float64(bl[game.BStable])
		r.lightCav += float64(v.Units[game.ULightCav])
		r.homeInf += float64(v.Units[game.USpear] + v.Units[game.USword] + v.Units[game.UAxe])
		if len(v.BuildQueue) == 0 {
			r.idleQueue++
		}
		settled := *v
		game.Accrue(&settled, w.Now(), w.Config())
		r.idleRes += game.SumResources(settled.Resources)
		capacity := game.VillageStorageCapacity(&settled)
		r.storage += float64(bl[game.BStorage])
		top := 0.0
		for _, res := range game.ResourceIDs {
			top = max(top, game.VillageProductionPerHour(&settled, res, w.Config()))
			if settled.Resources.Get(res) >= 0.95*capacity {
				r.storageFull++
				break
			}
		}
		r.storageHours += capacity / top
		production := 0.0
		for _, res := range game.ResourceIDs {
			production += game.VillageProductionPerHour(&settled, res, w.Config())
		}
		hours := float64(w.Now()-p.JoinAt) / float64(game.MsPerHour)
		r.lootShare += game.SumResources(w.Brain(b.id).Looted) / hours / float64(len(p.VillageIDs)) / production
		r.freePop += 1 - float64(game.PopulationUsed(w, v))/float64(game.VillageFarmCapacity(v))
		if next, ok := game.NextBuilding(w, b.id, &settled, game.Mulberry32(1)); ok {
			for _, bl := range game.CheckBuild(w, &settled, next).Blockers {
				if bl.Kind() == "population" {
					r.popBlocked++
				}
			}
		}
	}
	for _, f := range []*float64{&r.mines, &r.main, &r.farm, &r.hide, &r.wall, &r.stable, &r.lightCav, &r.homeInf, &r.idleQueue, &r.idleRes,
		&r.freePop, &r.popBlocked, &r.storage, &r.storageHours, &r.storageFull, &r.lootShare} {
		*f /= max(n, 1)
	}
	return r
}

func mean(rs []row) row {
	var m row
	for _, r := range rs {
		for i, f := range fields(&r) {
			*fields(&m)[i] += *f / float64(len(rs))
		}
	}
	return m
}

func fields(r *row) []*float64 {
	return []*float64{&r.top1, &r.top5, &r.median, &r.strongMed, &r.mediumMed, &r.weakMed, &r.mines, &r.main, &r.farm, &r.hide, &r.wall,
		&r.stable, &r.lightCav, &r.homeInf, &r.idleQueue, &r.idleRes, &r.freePop, &r.popBlocked,
		&r.storage, &r.storageHours, &r.storageFull, &r.lootShare}
}

func median(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	s := slices.Clone(xs)
	slices.Sort(s)
	return s[len(s)/2]
}

func dump(w *gametest.MemWorld) {
	points := w.PointsByOwner()
	ps := w.Players()
	sort.Slice(ps, func(i, j int) bool { return points[ps[i].ID] > points[ps[j].ID] })
	n := 0
	for _, p := range ps {
		b := w.Brain(p.ID)
		if b == nil || b.Skill < game.BotConfig.StrongSkill || n == 5 {
			continue
		}
		n++
		v := w.Village(p.VillageIDs[0])
		settled := *v
		game.Accrue(&settled, w.Now(), w.Config())
		next, ok := game.NextBuilding(w, p.ID, &settled, game.Mulberry32(1))
		var blockers []string
		for _, bl := range game.CheckBuild(w, &settled, next).Blockers {
			blockers = append(blockers, bl.Kind())
		}
		cost := game.BuildCost(next, game.EffectiveBuildingLevels(&settled)[next]+1)
		fmt.Printf("%s pts %d style %s skill %.2f online %v | res %.0f/%.0f/%.0f cap %.0f pop %d/%d | next %v ok %v blockers %v cost %.0f/%.0f/%.0f | plan %d queue %d\n",
			p.ID, points[p.ID], b.Style, b.Skill, b.Online, settled.Resources.Wood, settled.Resources.Clay, settled.Resources.Iron,
			game.VillageStorageCapacity(&settled), game.PopulationUsed(w, &settled), game.VillageFarmCapacity(&settled),
			game.BuildingDefs[next].ID, ok, blockers, cost.Wood, cost.Clay, cost.Iron, len(v.BuildPlan), len(v.BuildQueue))
		rs, _ := json.Marshal(v.Research)
		us, _ := json.Marshal(v.Units)
		rq, _ := json.Marshal(v.RecruitQueues)
		var rb []string
		for _, bl := range game.CheckResearch(w, &settled, game.ULightCav).Blockers {
			rb = append(rb, bl.Kind())
		}
		fmt.Printf("    research %s lc-blockers %v researchQ %d role %s\n    units %s\n    recruitQ %s\n", rs, rb, len(v.ResearchQueue), game.ChooseRole(w, p.ID, v), us, rq)
		bs, _ := json.Marshal(v.Buildings)
		fmt.Printf("    buildings %s\n", bs)
	}
}
