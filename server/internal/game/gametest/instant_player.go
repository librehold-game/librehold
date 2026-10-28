package gametest

import (
	"cmp"
	"slices"

	"librehold/internal/game"
)

// The instant player is a scripted active human for an instant world: it plays the whole hour the way a good
// player would (builds, recruits, farms barbarians, trains hetmans and takes villages) and acts at most once per
// playerStepMs per village. The pacing checks are measured on it; the idle guests of RunInstantSim are the other
// side of the table.

type InstantPlayerOptions struct {
	Seed       uint32
	IdleHumans int // guests that send their starting hetman at 30 s and then do nothing
}

type InstantPlayerRun struct {
	World            *MemWorld
	Player           game.PlayerID // "h1", the active player
	Villages         int           // villages of h1 at the end
	Rank             int           // 1 = most points at the end, among all players
	TopBotVillages   int           // most villages any bot holds at the end
	OwnedChanges     int           // conquests whose target had an owner before
	FirstConquestMs  int64         // h1's first conquest, -1 if none
	BotSendsAfterEnd int           // bot scheduled sends due after the world's end
}

const (
	playerStepMs = 20_000
	// barbarianFirst is how many villages the player holds before it turns to villages with an owner.
	barbarianFirst = 3
	scoutAgeMs     = 90_000
	farmAgainMs    = 90_000
)

type instantPlayer struct {
	referencePlayer
	capital string
}

// RunInstantPlayer plays an instant world's hour with the active player in seat h1.
func RunInstantPlayer(o InstantPlayerOptions) (InstantPlayerRun, error) {
	cfg := game.InstantWorldConfig
	w := NewMemWorld(o.Seed, cfg, 0)
	if err := game.NewWorld(w); err != nil {
		return InstantPlayerRun{}, err
	}
	home, err := game.JoinHuman(w, "h1", "Player", 1, game.QuadrantRandom)
	if err != nil {
		return InstantPlayerRun{}, err
	}
	var idle []game.PlayerID
	for i := range o.IdleHumans {
		id := game.PlayerID("h" + string(rune('2'+i)))
		if _, err := game.JoinHuman(w, id, "Guest"+string(rune('2'+i)), int64(i+2), game.QuadrantRandom); err != nil {
			return InstantPlayerRun{}, err
		}
		idle = append(idle, id)
	}
	p := &instantPlayer{referencePlayer: referencePlayer{w: w, id: "h1", campaigns: map[string][2]int{}}, capital: home.ID}
	run := InstantPlayerRun{World: w, Player: p.id, FirstConquestMs: -1}
	end := int64(cfg.DurationMin) * game.MsPerMinute
	sentIdle := false
	for now := int64(playerStepMs); now <= end; now += playerStepMs {
		if err := Advance(w, now); err != nil {
			return run, err
		}
		if !sentIdle && now >= 30_000 {
			sentIdle = true
			for _, id := range idle {
				h := w.Village(w.Player(id).VillageIDs[0])
				x, y := NearestBarbarianXY(w, h)
				units := game.UnitCounts{game.UNoble: 1, game.UAxe: 50}
				if _, err := game.SendCommand(w, id, h.ID, x, y, units, "attack", game.SendOpts{}); err != nil {
					return run, err
				}
			}
		}
		for _, v := range game.PlayerVillages(w, p.id) {
			game.Touch(w, v)
			if err := p.step(v); err != nil {
				return run, err
			}
		}
	}
	p.summarize(&run, end)
	return run, nil
}

func (p *instantPlayer) summarize(run *InstantPlayerRun, end int64) {
	w := p.w
	run.Villages = len(w.Player(p.id).VillageIDs)
	mine := game.PlayerPoints(w, p.id)
	run.Rank = 1
	for _, pl := range w.Players() {
		if pl.ID != p.id && game.PlayerPoints(w, pl.ID) > mine {
			run.Rank++
		}
		if !pl.IsHuman {
			run.TopBotVillages = max(run.TopBotVillages, len(pl.VillageIDs))
			for _, s := range w.ScheduledSendsOf(pl.ID) {
				if s.SendAt > end {
					run.BotSendsAfterEnd++
				}
			}
		}
	}
	for _, c := range w.ConquestsSince(0) {
		if c.FromID != "" {
			run.OwnedChanges++
		}
		if c.ToID == p.id && run.FirstConquestMs < 0 {
			run.FirstConquestMs = c.T
		}
	}
}

func (p *instantPlayer) step(v *game.Village) error {
	for _, f := range []func(*game.Village) error{p.constructions, p.troops, p.nobleTrain, p.farmBarbarians} {
		if err := f(v); err != nil {
			return err
		}
	}
	return nil
}

// ownedMode: past its barbarian villages the player takes villages with an owner, which needs as many hetmans
// in the capital at once as the owned drop asks, and so an Academy that holds them.
func (p *instantPlayer) ownedMode() bool {
	return len(game.PlayerVillages(p.w, p.id)) >= barbarianFirst
}

// nobleReserve is what the capital holds back: the next Academy level, then the next hetman while it needs one.
func (p *instantPlayer) nobleReserve(v *game.Village) game.Resources {
	if v.ID != p.capital {
		return game.Resources{}
	}
	if p.academyTooSmall(v) {
		return game.BuildCost(game.BAcademy, v.Buildings[game.BAcademy]+1)
	}
	if game.TotalNoblesOwned(p.w, p.id) >= p.noblesWanted() {
		return game.Resources{}
	}
	return game.UnitCostIn(p.w.Config(), game.UNoble, 1)
}

// noblesWanted: one for a barbarian village, as many as an owned village takes for the rest.
func (p *instantPlayer) noblesWanted() int {
	if !p.ownedMode() {
		return 1
	}
	return game.NoblesNeededFor(p.w.Config(), true, game.ConquestBalance.MaxLoyalty)
}

func (p *instantPlayer) academyTooSmall(v *game.Village) bool {
	return game.NobleLimit(p.w.Config(), v) < p.noblesWanted()
}

func (p *instantPlayer) constructions(v *game.Village) error {
	if p.queueFull(v) {
		return nil
	}
	if v.ID == p.capital && p.academyTooSmall(v) {
		if len(game.CheckBuild(p.w, v, game.BAcademy).Blockers) == 0 {
			return p.enqueueBuild(v, game.BAcademy)
		}
		return nil
	}
	reserve := p.nobleReserve(v)
	if min(v.Resources.Wood-reserve.Wood, v.Resources.Clay-reserve.Clay, v.Resources.Iron-reserve.Iron) < game.VillageStorageCapacity(v)*0.0 {
		return nil
	}
	return p.build(v)
}

func (p *instantPlayer) troops(v *game.Village) error {
	reserve := p.nobleReserve(v)
	for _, u := range []game.UnitID{game.UScout, game.UAxe, game.ULightCav} {
		queue := v.RecruitQueues.Barracks
		batch := 100
		switch u {
		case game.ULightCav:
			queue, batch = v.RecruitQueues.Stable, 40
		case game.UScout:
			queue, batch = v.RecruitQueues.Stable, 10
			if v.Units[game.UScout] >= 10 {
				continue
			}
		}
		if len(queue) > 1 || v.Buildings[game.UnitDefs[u].Building] < 1 {
			continue
		}
		cost := game.UnitCostIn(p.w.Config(), u, batch)
		if v.Resources.Wood-reserve.Wood < cost.Wood || v.Resources.Clay-reserve.Clay < cost.Clay || v.Resources.Iron-reserve.Iron < cost.Iron {
			continue
		}
		if game.MaxRecruitable(p.w, v, u) >= batch && len(game.CheckRecruit(p.w, v, u, batch).Blockers) == 0 {
			if _, err := game.EnqueueRecruit(p.w, p.id, v.ID, u, batch); err != nil {
				return err
			}
		}
	}
	return nil
}

// nobleTrain trains hetmans in the capital and sends them off with an escort at the best target in reach.
func (p *instantPlayer) nobleTrain(v *game.Village) error {
	if v.ID != p.capital {
		return nil
	}
	if game.TotalNoblesOwned(p.w, p.id) < p.noblesWanted() && len(v.RecruitQueues.Academy) == 0 &&
		game.NobleCountOf(p.w, v) < game.NobleLimit(p.w.Config(), v) && len(game.CheckRecruit(p.w, v, game.UNoble, 1).Blockers) == 0 {
		if _, err := game.EnqueueRecruit(p.w, p.id, v.ID, game.UNoble, 1); err != nil {
			return err
		}
	}
	if v.Units[game.UNoble] < 1 {
		return nil
	}
	if p.ownedMode() {
		if sent, err := p.takeOwned(v); sent || err != nil {
			return err
		}
	}
	return p.takeBarbarian(v)
}

func (p *instantPlayer) inboundNoble(targetID string) bool {
	for _, v := range game.PlayerVillages(p.w, p.id) {
		for _, c := range p.w.CommandsHome(v.ID) {
			if c.TargetID == targetID && c.Units[game.UNoble] > 0 {
				return true
			}
		}
	}
	return false
}

func (p *instantPlayer) takeBarbarian(v *game.Village) error {
	var targets []game.FarmTarget
	for _, t := range game.FarmTargets(p.w, p.id, v.ID) {
		if !p.inboundNoble(t.ID) {
			targets = append(targets, t)
		}
	}
	slices.SortStableFunc(targets, func(a, b game.FarmTarget) int { return cmp.Compare(a.Distance, b.Distance) })
	if len(targets) == 0 || v.Units[game.UAxe] < 50 {
		return nil
	}
	t := targets[0]
	return p.send(v, t.X, t.Y, game.UnitCounts{game.UNoble: 1, game.UAxe: min(v.Units[game.UAxe], 100)})
}

// takeOwned scouts villages with an owner around the capital and sends a full train at the biggest one it can break.
func (p *instantPlayer) takeOwned(v *game.Village) (bool, error) {
	cfg := p.w.Config()
	if v.Units[game.UNoble] < game.NoblesNeededFor(cfg, true, game.ConquestBalance.MaxLoyalty) {
		return false, nil
	}
	type cand struct {
		tile  game.Tile
		def   float64
		known bool
		need  int
	}
	var cands []cand
	for _, t := range game.VillagesInRadius(p.w, v.X, v.Y, 20) {
		if t.Kind != "village" || t.OwnerID == "" || t.OwnerID == game.Ref(p.id) || p.inboundNoble(t.ID) {
			continue
		}
		c := cand{tile: t, need: game.NoblesNeededFor(cfg, true, game.ConquestBalance.MaxLoyalty)}
		if in := p.w.Intel(p.id, t.ID); in != nil && in.Intel != nil && p.w.Now()-in.Intel.At <= scoutAgeMs {
			c.def, _ = p.knownDefense(t.ID, false)
			c.known = true
			loyalty := game.LoyaltyAt(&game.Village{Loyalty: in.Intel.Loyalty, LoyaltyUpdatedAt: in.Intel.At}, p.w.Now(), cfg.WorldSpeed)
			c.need = game.NoblesNeededFor(cfg, true, loyalty)
		}
		cands = append(cands, c)
	}
	slices.SortStableFunc(cands, func(a, b cand) int {
		return cmp.Or(cmp.Compare(b.tile.Points, a.tile.Points), cmp.Compare(game.Distance(v.X, v.Y, a.tile.X, a.tile.Y), game.Distance(v.X, v.Y, b.tile.X, b.tile.Y)))
	})
	scouted := 0
	for _, c := range cands {
		if !c.known {
			if scouted < 2 && v.Units[game.UScout] >= 5 {
				scouted++
				if err := p.send(v, c.tile.X, c.tile.Y, game.UnitCounts{game.UScout: 5}); err != nil {
					return false, err
				}
			}
			continue
		}
		escort := game.UnitCounts{game.UAxe: v.Units[game.UAxe]}
		if v.Units[game.UNoble] < c.need || attackPower(escort) < c.def*5 {
			continue
		}
		// The hetmans are slower than the army: they leave now, the army is timed to land a second ahead of them.
		train := game.UnitCounts{game.UNoble: c.need, game.UAxe: 20}
		if len(game.CheckSend(p.w, p.id, v.ID, c.tile.X, c.tile.Y, train, "attack").Blockers) > 0 {
			return false, nil
		}
		cmd, err := game.SendCommand(p.w, p.id, v.ID, c.tile.X, c.tile.Y, train, "attack", game.SendOpts{})
		if err != nil {
			return false, err
		}
		escort[game.UAxe] -= 20
		_, err = game.ScheduleSend(p.w, p.id, game.ScheduleSendInput{OriginID: v.ID, X: c.tile.X, Y: c.tile.Y,
			Units: escort, Type: "attack", Mode: "arriveAt", At: cmd.ArrivesAt - 1000})
		return err == nil, err
	}
	return false, nil
}

func (p *instantPlayer) farmBarbarians(v *game.Village) error {
	sent := 0
	var targets []game.FarmTarget
	for _, t := range game.FarmTargets(p.w, p.id, v.ID) {
		fresh := t.LastAttackAt == nil || p.w.Now()-*t.LastAttackAt > farmAgainMs
		if t.Distance <= 12 && fresh && !t.Inbound && (t.Wall == nil || *t.Wall <= 5) {
			targets = append(targets, t)
		}
	}
	slices.SortStableFunc(targets, func(a, b game.FarmTarget) int { return cmp.Compare(a.Distance, b.Distance) })
	for _, t := range targets {
		if sent >= 3 || v.Units[game.ULightCav] < 10 {
			break
		}
		party := game.UnitCounts{game.ULightCav: 10}
		if def, ok := p.knownDefense(t.ID, true); ok && attackPower(party) < def*2.5 {
			continue
		}
		if err := p.send(v, t.X, t.Y, party); err != nil {
			return err
		}
		sent++
	}
	return nil
}
