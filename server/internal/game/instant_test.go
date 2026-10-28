package game_test

import (
	"testing"

	"librehold/internal/game"
	"librehold/internal/game/gametest"
)

func newInstant(t *testing.T, seed uint32) (*gametest.MemWorld, *game.Village) {
	t.Helper()
	w := gametest.NewMemWorld(seed, game.InstantWorldConfig, 0)
	if err := game.NewWorld(w); err != nil {
		t.Fatal(err)
	}
	v, err := game.JoinHuman(w, "h1", "Ala", 1, game.QuadrantRandom)
	if err != nil {
		t.Fatal(err)
	}
	return w, v
}

func TestInstantHumanStartsBuiltWithNoble(t *testing.T) {
	_, v := newInstant(t, 7)
	if v.Buildings[game.BAcademy] != 1 || v.Buildings[game.BMain] != 20 || v.Buildings[game.BSmithy] != 20 || v.Buildings[game.BStorage] != 20 {
		t.Fatalf("buildings %+v", v.Buildings)
	}
	if v.Units[game.UNoble] != 1 || v.Units[game.UAxe] != 100 || v.Units[game.ULightCav] != 30 || v.Units[game.USpear] != 50 {
		t.Fatalf("units %+v", v.Units)
	}
	if v.Resources.Wood != 20000 {
		t.Fatalf("resources %+v", v.Resources)
	}
}

// The preset skips the build-up, so it must be a village the rules could have produced: every building's
// requirements met, every unit but the paladin recruitable without research, resources and army that fit.
func TestInstantStartVillageIsConsistent(t *testing.T) {
	w, v := newInstant(t, 7)
	for _, b := range game.BuildingIDs {
		if v.Buildings[b] > 0 && !game.MeetsRequirements(v.Buildings, b) {
			t.Errorf("%s %d lacks its requirements: %+v", b, v.Buildings[b], game.BuildingDefs[b].Requirements)
		}
	}
	for _, u := range game.UnitIDs {
		if u == game.UPaladin {
			continue
		}
		for _, bl := range game.CheckRecruit(w, v, u, 1).Blockers {
			if k := bl.Kind(); k == "requirements" || k == "research" {
				t.Errorf("%s blocked: %v", u, bl)
			}
		}
	}
	if c := game.StorageCapacity(v.Buildings[game.BStorage], nil); v.Resources.Wood > c || v.Resources.Clay > c || v.Resources.Iron > c {
		t.Errorf("resources %+v over storage %v", v.Resources, c)
	}
	if used, farm := game.PopulationUsed(w, v), game.VillageFarmCapacity(v); used > farm {
		t.Errorf("population %d over farm %d", used, farm)
	}
}

func TestInstantBotsStartBuiltToo(t *testing.T) {
	w, _ := newInstant(t, 7)
	for _, p := range w.Players() {
		if p.IsHuman {
			continue
		}
		bv := w.Village(p.VillageIDs[0])
		if bv.Buildings[game.BAcademy] != 1 || bv.Units[game.UNoble] != 1 {
			t.Fatalf("bot %s village %+v %+v", p.ID, bv.Buildings, bv.Units)
		}
	}
}

func TestInstantBarbariansStartLowLoyalty(t *testing.T) {
	w, v := newInstant(t, 7)
	x, y := gametest.NearestBarbarianXY(w, v)
	b := game.EnsureVillage(w, x, y)
	if b == nil || b.Loyalty != game.InstantWorldConfig.BarbarianLoyalty {
		t.Fatalf("barbarian loyalty %+v", b)
	}
	long := gametest.NewMemWorld(7, gametest.TestConfig(), 0)
	if lb := game.EnsureVillage(long, x, y); lb != nil && lb.Loyalty != 100 {
		t.Fatalf("long-world barbarian loyalty %v", lb.Loyalty)
	}
}

func TestInstantBotsThinkEveryFewSeconds(t *testing.T) {
	w, _ := newInstant(t, 7)
	if err := gametest.Advance(w, 2*game.MsPerMinute); err != nil {
		t.Fatal(err)
	}
	bots := 0
	for _, p := range w.Players() {
		if p.IsHuman {
			continue
		}
		bots++
		if w.IsScheduled("session:" + p.ID) {
			t.Fatalf("%s runs sessions in an instant world", p.ID)
		}
		at, ok := w.ScheduledAt("think:" + p.ID)
		if !ok || at-w.Now() > int64(1.4*game.InstantWorldConfig.BotThinkEverySec*1000)+1 {
			t.Fatalf("%s next think at %d (now %d, ok %v)", p.ID, at, w.Now(), ok)
		}
	}
	if bots != game.InstantWorldConfig.BotCount {
		t.Fatalf("%d bots", bots)
	}
}

func TestInstantNewcomerGraceBlocksAttacks(t *testing.T) {
	w, v := newInstant(t, 7)
	var bot *game.Player
	for _, p := range w.Players() {
		if !p.IsHuman {
			bot = p
			break
		}
	}
	bv := w.Village(bot.VillageIDs[0])
	units := game.UnitCounts{game.UAxe: 10}
	blocked := func() bool {
		for _, b := range game.CheckSend(w, bot.ID, bv.ID, v.X, v.Y, units, "attack").Blockers {
			if k := b.Kind(); k == "targetProtected" || k == "selfProtected" {
				return true
			}
		}
		return false
	}
	if !blocked() {
		t.Fatal("a bot may attack a newcomer at once")
	}
	w.SetNow(11 * game.MsPerMinute)
	if blocked() {
		t.Fatal("still protected after 11 minutes")
	}
}

func TestLongWorldBotTimingUnchanged(t *testing.T) {
	w := gametest.NewMemWorld(7, gametest.TestConfig(), 0)
	if got := game.BotDur(w, 4*game.MsPerHour); got != 4*game.MsPerHour {
		t.Fatalf("long world: %d", got)
	}
	iw := gametest.NewMemWorld(7, game.InstantWorldConfig, 0)
	if got := game.BotDur(iw, 4*game.MsPerHour); got != int64(float64(4*game.MsPerHour)/game.InstantWorldConfig.BotTimeScale) {
		t.Fatalf("instant world: %d", got)
	}
}

func TestInstantSightRadiusAndFieldBattlesDefaults(t *testing.T) {
	instant, long := game.InstantWorldConfig, game.DefaultWorldConfig
	if r := game.SightRadiusOf(&instant); r != 5 {
		t.Errorf("instant sight radius = %v, want 5", r)
	}
	if r := game.SightRadiusOf(&long); r != 1 {
		t.Errorf("long world sight radius = %v, want 1", r)
	}
	if !game.FieldBattlesOn(&instant) || !game.FieldBattlesOn(&long) {
		t.Error("field battles should be on in every world by default")
	}
	off := long
	off.NoFieldBattles = true
	if game.FieldBattlesOn(&off) {
		t.Error("NoFieldBattles should turn field battles off")
	}
}
