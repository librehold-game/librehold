package gametest

import (
	"cmp"
	"fmt"
	"math"
	"slices"
	"time"

	"librehold/internal/game"
)

// The reference player measures the game's pace: a scripted player in the human seat that builds, farms, mints
// coins, trains nobles and conquers. It is meant to be the lower bound of player skill: it knows only what the UI
// shows (a target's defence only from its own intel) and plays a plain routine. If it gets stuck, a player will
// too. `go run ./cmd/sim -player` prints its run and pacing_test.go holds its milestones to thresholds.

// Milestones are the moments RunReferencePlayer records, in order.
var Milestones = []string{"first attack", "first successful raid", "1,000 pts", "first hetman", "2nd village", "3rd village", "5th village", "10th village"}

// DaySnapshot is the player at the end of a day; the army, nobles and buildings are the home village's.
type DaySnapshot struct {
	Day, Points, Villages, Army, Coins, Nobles int
	Looted                                     float64
	Buildings                                  game.Buildings
}

type PlayerRunOptions struct {
	Seed  uint32
	Days  int
	Speed float64 // world speed; 0 keeps the default world settings
	OnDay func(DaySnapshot)
}

type PlayerRun struct {
	World      *MemWorld
	Human      game.PlayerID
	Milestones map[string]int64 // world ms; -1 = not reached
	Days       []DaySnapshot
}

const playerTickMs = 30 * game.MsPerMinute

type referencePlayer struct {
	w  *MemWorld
	id game.PlayerID
	// campaigns is the conquest target of each village with a noble, hit until taken (loyalty recovers +1/h).
	campaigns map[string][2]int
}

// RunReferencePlayer plays the reference player on a default world for o.Days, in 30-minute ticks.
func RunReferencePlayer(o PlayerRunOptions) (PlayerRun, error) {
	cfg := game.DefaultWorldConfig
	if o.Speed > 0 {
		cfg.WorldSpeed = o.Speed
	}
	w := NewMemWorld(o.Seed, cfg, time.Date(2026, 10, 19, 0, 0, 0, 0, time.UTC).UnixMilli())
	if err := game.NewWorld(w); err != nil {
		return PlayerRun{}, err
	}
	home, err := game.JoinHuman(w, "h1", "Player", 1, game.QuadrantRandom)
	if err != nil {
		return PlayerRun{}, err
	}
	p := &referencePlayer{w: w, id: "h1", campaigns: map[string][2]int{}}
	run := PlayerRun{World: w, Human: p.id, Milestones: map[string]int64{}}
	for _, m := range Milestones {
		run.Milestones[m] = -1
	}
	reach := func(m string, ok bool) {
		if ok && run.Milestones[m] < 0 {
			run.Milestones[m] = w.Now()
		}
	}
	for t := int64(playerTickMs); t <= int64(o.Days)*game.MsPerDay; t += playerTickMs {
		if err := Advance(w, t); err != nil {
			return run, err
		}
		for _, v := range game.PlayerVillages(w, p.id) {
			game.Touch(w, v)
			// Alternate: construction goes first one tick, recruitment the next — neither starves the other.
			first, second := p.build, p.recruit
			if (t/playerTickMs)%2 != 0 {
				first, second = p.recruit, p.build
			}
			if err := first(v); err != nil {
				return run, err
			}
			if err := second(v); err != nil {
				return run, err
			}
			if err := p.research(v); err != nil {
				return run, err
			}
			if err := p.nobles(v); err != nil {
				return run, err
			}
			if t%game.MsPerHour == 0 {
				if err := p.farm(v); err != nil {
					return run, err
				}
			}
		}

		villages := game.PlayerVillages(w, p.id)
		points := game.PlayerPoints(w, p.id)
		stats := &w.Human(p.id).Stats
		looted := stats.ResourcesLooted.Wood + stats.ResourcesLooted.Clay + stats.ResourcesLooted.Iron
		reach("first attack", stats.AttacksSent >= 1)
		reach("first successful raid", looted > 0)
		reach("1,000 pts", points >= 1000)
		reach("first hetman", slices.ContainsFunc(villages, func(v *game.Village) bool { return v.Units[game.UNoble] > 0 }))
		for _, m := range []struct {
			key string
			n   int
		}{{"2nd village", 2}, {"3rd village", 3}, {"5th village", 5}, {"10th village", 10}} {
			reach(m.key, len(villages) >= m.n)
		}

		if t%game.MsPerDay == 0 {
			v := w.Village(home.ID)
			s := DaySnapshot{Day: int(t / game.MsPerDay), Points: points, Looted: looted, Villages: len(villages),
				Army: v.Units[game.USpear] + v.Units[game.UAxe] + v.Units[game.ULightCav], Coins: w.Player(p.id).Coins,
				Nobles: v.Units[game.UNoble], Buildings: v.Buildings}
			run.Days = append(run.Days, s)
			if o.OnDay != nil {
				o.OnDay(s)
			}
		}
	}
	return run, nil
}

func attackPower(u game.UnitCounts) float64 {
	a := 0.0
	for _, id := range game.UnitIDs {
		a += float64(u[id]) * game.UnitDefs[id].Attack
	}
	return a
}

// knownDefense is a target's defence from the latest report or scouting (as the player sees it), against
// infantry or cavalry: a spearman defends three times better against cavalry. ok is false when nothing is known.
func (p *referencePlayer) knownDefense(villageID string, cavalry bool) (float64, bool) {
	in := p.w.Intel(p.id, villageID)
	if in == nil || in.Intel == nil {
		return 0, false
	}
	d := 0.0
	for _, id := range game.UnitIDs {
		def := game.UnitDefs[id].Defense
		if cavalry {
			def = game.UnitDefs[id].DefenseCavalry
		}
		d += float64(in.Intel.Units[id]) * def
	}
	wall := float64(in.Intel.Wall)
	return d*math.Pow(1.037, wall) + 20*wall, true
}

func (p *referencePlayer) armySize(v *game.Village) int {
	away := game.UnitsAway(p.w, v.ID)
	return v.Units[game.USpear] + v.Units[game.UAxe] + v.Units[game.ULightCav] + away[game.USpear] + away[game.UAxe] + away[game.ULightCav]
}

// armyTarget grows over time: 30 + 15/day. A village with an Academy also keeps an escort for campaigns.
func (p *referencePlayer) armyTarget(v *game.Village) float64 {
	base := math.Min(400, 30+15*(float64(p.w.Now())/float64(game.MsPerDay)))
	if v.Buildings[game.BAcademy] > 0 {
		return math.Min(900, base*2)
	}
	return base
}

func (p *referencePlayer) queueFull(v *game.Village) bool {
	return len(v.BuildQueue) >= game.BuildQueueSlotsFor(p.w, p.w.Player(p.id))
}

func (p *referencePlayer) enqueueBuild(v *game.Village, b game.BuildingID) error {
	_, err := game.EnqueueBuild(p.w, p.id, v.ID, b)
	return err
}

var mines = []game.BuildingID{game.BWood, game.BClay, game.BIron}

func (p *referencePlayer) build(v *game.Village) error {
	if p.queueFull(v) {
		return nil
	}
	var forced []game.BuildingID
	if game.VillageFarmCapacity(v)-game.VillagePopulationUsed(v) < 20 {
		forced = append(forced, game.BFarm)
	}
	if max(v.Resources.Wood, v.Resources.Clay, v.Resources.Iron) > game.VillageStorageCapacity(v)*0.85 {
		forced = append(forced, game.BStorage)
	}
	// No second village without the Academy, so it goes before further economy upgrades.
	if v.Buildings[game.BAcademy] < 1 {
		forced = append(forced, game.BAcademy)
	}
	for _, b := range forced {
		if len(game.CheckBuild(p.w, v, b).Blockers) == 0 {
			return p.enqueueBuild(v, b)
		}
	}
	wanted := func(b game.BuildingID) bool {
		lvl := v.Buildings[b]
		switch b {
		case game.BHide:
			return lvl < 2
		case game.BWall:
			return lvl < 5
		case game.BStatue:
			return lvl < 1
		case game.BMarket:
			return lvl < 10
		}
		return true
	}
	// "Cheapest" = shortest time to save up the cost at current production (just summing the cost starves iron).
	cfg := p.w.Config()
	hoursToAfford := func(c game.Resources) float64 {
		return max(c.Wood/math.Max(1, game.VillageProductionPerHour(v, game.Wood, cfg)),
			c.Clay/math.Max(1, game.VillageProductionPerHour(v, game.Clay, cfg)),
			c.Iron/math.Max(1, game.VillageProductionPerHour(v, game.Iron, cfg)))
	}
	type candidate struct {
		b    game.BuildingID
		cost game.Resources
	}
	var candidates []candidate
	for _, b := range game.BuildingOrder {
		if !wanted(b) {
			continue
		}
		if c := game.CheckBuild(p.w, v, b); len(c.Blockers) == 0 {
			candidates = append(candidates, candidate{b, c.Cost})
		}
	}
	slices.SortStableFunc(candidates, func(a, b candidate) int { return cmp.Compare(hoursToAfford(a.cost), hoursToAfford(b.cost)) })
	if len(candidates) > 0 {
		return p.enqueueBuild(v, candidates[0].b)
	}
	// Nothing affordable: if one resource blocks while the others are capped, upgrade its mine as soon as possible.
	amount := func(b game.BuildingID) float64 {
		switch b {
		case game.BWood:
			return v.Resources.Wood
		case game.BClay:
			return v.Resources.Clay
		}
		return v.Resources.Iron
	}
	scarce := slices.Clone(mines)
	slices.SortStableFunc(scarce, func(a, b game.BuildingID) int { return cmp.Compare(amount(a), amount(b)) })
	if len(game.CheckBuild(p.w, v, scarce[0]).Blockers) == 0 {
		return p.enqueueBuild(v, scarce[0])
	}
	return nil
}

func (p *referencePlayer) research(v *game.Village) error {
	if len(v.ResearchQueue) > 0 {
		return nil
	}
	for _, u := range []game.UnitID{game.UAxe, game.ULightCav, game.UScout} {
		if v.Research[u] >= 1 {
			continue
		}
		if len(game.CheckResearch(p.w, v, u).Blockers) == 0 {
			_, err := game.EnqueueResearch(p.w, p.id, v.ID, u)
			return err
		}
	}
	return nil
}

func (p *referencePlayer) recruit(v *game.Village) error {
	// Recruit only from surplus (storage > 45 %), so construction isn't starved.
	surplus := min(v.Resources.Wood, v.Resources.Clay, v.Resources.Iron) > game.VillageStorageCapacity(v)*0.45
	// Early game: a player trains the first handful of spearmen right away without waiting for surplus — otherwise
	// the farming probe only leaves after a week and the "first farm" milestone measures the player's sloth.
	early := p.armySize(v) < 20 && v.Buildings[game.BBarracks] >= 1
	if !p.queueFull(v) && !surplus && !early && float64(p.armySize(v)) >= p.armyTarget(v) {
		return nil
	}
	// A village with the Academy trains mostly axemen: they form the stack that escorts the noble.
	wantEscort := v.Buildings[game.BAcademy] > 0 && v.Units[game.UAxe] < 320
	var prefer []game.UnitID
	switch {
	case wantEscort && v.Research[game.UAxe] >= 1:
		prefer = []game.UnitID{game.UAxe, game.ULightCav}
	case v.Research[game.ULightCav] >= 1:
		prefer = []game.UnitID{game.ULightCav, game.UAxe}
	case v.Research[game.UAxe] >= 1:
		prefer = []game.UnitID{game.UAxe}
	case v.Units[game.USpear] < 20:
		prefer = []game.UnitID{game.USpear}
	}
	for _, u := range prefer {
		q := v.RecruitQueues.Barracks
		if u == game.ULightCav {
			q = v.RecruitQueues.Stable
		}
		if len(q) > 0 {
			continue
		}
		// Batches of 20–30 (light cavalry by 10): recruiting single units from every surplus would starve construction.
		batch, most := 20, 30
		switch {
		case early:
			batch, most = 5, 10
		case u == game.ULightCav:
			batch, most = 10, 10
		}
		n := min(most, game.MaxRecruitable(p.w, v, u))
		if n < batch {
			continue
		}
		if len(game.CheckRecruit(p.w, v, u, n).Blockers) == 0 {
			if _, err := game.EnqueueRecruit(p.w, p.id, v.ID, u, n); err != nil {
				return err
			}
		}
	}
	return nil
}

// farmParty is the smallest party that beats the known defence with a 2.5× margin. Without intel a probe goes:
// barbarians have no troops, so a handful of axemen or spearmen is enough.
func (p *referencePlayer) farmParty(v *game.Village, targetID string) (game.UnitCounts, bool) {
	// With a noble in the village the conquest stack is off limits — otherwise the escort melts away in raids
	// and the player sits on three nobles, a full storage and no troops to escort them.
	reserve := 20
	if v.Units[game.UNoble] > 0 {
		reserve = 260
	}
	known, ok := p.knownDefense(targetID, false)
	if !ok {
		if v.Units[game.UAxe] >= 15 {
			return game.UnitCounts{game.UAxe: 15}, true
		}
		if v.Units[game.USpear] >= 10 {
			return game.UnitCounts{game.USpear: 10}, true
		}
		return game.UnitCounts{}, false
	}
	defCav, ok := p.knownDefense(targetID, true)
	if !ok {
		defCav = known
	}
	type option struct {
		units game.UnitCounts
		def   float64
	}
	var options []option
	for _, n := range []int{20, 40, 60, 90} {
		if v.Units[game.ULightCav]-20 >= n {
			options = append(options, option{game.UnitCounts{game.ULightCav: n}, defCav})
		}
	}
	for _, n := range []int{40, 80, 150, 250, 400} {
		if v.Units[game.UAxe]-reserve >= n {
			options = append(options, option{game.UnitCounts{game.UAxe: n}, known})
		}
	}
	slices.SortStableFunc(options, func(a, b option) int { return cmp.Compare(attackPower(a.units), attackPower(b.units)) })
	for _, o := range options {
		if attackPower(o.units) >= o.def*2.5 {
			return o.units, true
		}
	}
	return game.UnitCounts{}, false
}

func (p *referencePlayer) send(v *game.Village, x, y int, units game.UnitCounts) error {
	if len(game.CheckSend(p.w, p.id, v.ID, x, y, units, "attack").Blockers) > 0 {
		return nil
	}
	_, err := game.SendCommand(p.w, p.id, v.ID, x, y, units, "attack", game.SendOpts{})
	return err
}

func (p *referencePlayer) farm(v *game.Village) error {
	var targets []game.FarmTarget
	for _, t := range game.FarmTargets(p.w, p.id, v.ID) {
		fresh := t.LastAttackAt == nil || p.w.Now()-*t.LastAttackAt > 2*game.MsPerHour
		if t.Distance <= 12 && fresh && !t.Inbound && (t.Wall == nil || *t.Wall <= 5) {
			targets = append(targets, t)
		}
	}
	slices.SortStableFunc(targets, func(a, b game.FarmTarget) int { return cmp.Compare(a.Distance, b.Distance) })
	for _, t := range targets {
		units, ok := p.farmParty(v, t.ID)
		if !ok {
			continue
		}
		if err := p.send(v, t.X, t.Y, units); err != nil {
			return err
		}
	}
	return nil
}

func (p *referencePlayer) nobles(v *game.Village) error {
	if v.Buildings[game.BAcademy] <= 0 {
		return nil
	}
	owned := 0
	for _, o := range game.PlayerVillages(p.w, p.id) {
		owned += game.NobleCountOf(p.w, o)
	}
	room := game.NobleCountOf(p.w, v) < game.NobleLimit(p.w.Config(), v)
	// Coins only for the next noble. Minting ahead burns all income: nothing is ever left for the noble's cost.
	if room && p.w.Player(p.id).Coins < game.CoinsForNextNoble(p.w, p.id, owned) {
		if len(game.CheckMint(v).Blockers) == 0 {
			return game.MintCoin(p.w, p.id, v.ID)
		}
		return nil
	}
	if room && len(v.RecruitQueues.Academy) == 0 && len(game.CheckRecruit(p.w, v, game.UNoble, 1).Blockers) == 0 {
		if _, err := game.EnqueueRecruit(p.w, p.id, v.ID, game.UNoble, 1); err != nil {
			return err
		}
	}
	if v.Units[game.UNoble] < 1 {
		return nil
	}
	escort := min(v.Units[game.UAxe]-40, 300)
	if escort < 60 {
		return nil
	}
	power := attackPower(game.UnitCounts{game.UAxe: escort, game.ULightCav: v.Units[game.ULightCav]})

	target, has := p.campaigns[v.ID]
	if has {
		if tv := p.w.Village(fmt.Sprintf("v:%d:%d", target[0], target[1])); tv == nil || tv.OwnerID != "" {
			delete(p.campaigns, v.ID)
			has = false
		}
	}
	if !has {
		// A player doesn't storm the fortress next door — they look for a village their stack can actually break.
		type cand struct {
			t   game.FarmTarget
			def float64
		}
		var cands []cand
		for _, t := range game.FarmTargets(p.w, p.id, v.ID) {
			if t.Distance > 15 {
				continue
			}
			def, ok := p.knownDefense(t.ID, false)
			if !ok {
				def = float64(t.Points) * 3
			}
			cands = append(cands, cand{t, def})
		}
		slices.SortStableFunc(cands, func(a, b cand) int {
			return cmp.Or(cmp.Compare(a.def, b.def), cmp.Compare(a.t.Distance, b.t.Distance))
		})
		if len(cands) == 0 || power < cands[0].def*2 {
			return nil
		}
		target = [2]int{cands[0].t.X, cands[0].t.Y}
		p.campaigns[v.ID] = target
	}
	tv := game.EnsureVillage(p.w, target[0], target[1])
	if tv == nil {
		delete(p.campaigns, v.ID)
		return nil
	}
	// The target fortified before the campaign set out — not worth the noble, find another.
	if def, ok := p.knownDefense(tv.ID, false); ok && power < def*1.5 {
		delete(p.campaigns, v.ID)
		return nil
	}
	units := game.UnitCounts{game.UNoble: v.Units[game.UNoble], game.UAxe: escort}
	if v.Units[game.ULightCav] > 40 {
		units[game.ULightCav] = v.Units[game.ULightCav] - 20
	}
	return p.send(v, target[0], target[1], units)
}
