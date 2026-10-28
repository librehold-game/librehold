package game_test

import (
	"testing"

	"librehold/internal/game"
	"librehold/internal/game/gametest"
)

func interceptsFrom(w *gametest.MemWorld, villageID string) []*game.Command {
	var out []*game.Command
	for _, c := range w.Commands() {
		if c.Type == "intercept" && c.OriginID == villageID {
			out = append(out, c)
		}
	}
	return out
}

// botUnderAttack: h1 attacks bot_0 with axes the bot remembers seeing; the bot holds 300 light cavalry.
func botUnderAttack(t *testing.T, axes int, skill float64) (*gametest.MemWorld, *game.Village, *game.Command) {
	t.Helper()
	w, bv := oneBotWorld(t, 7, skill)
	home := game.PlayerVillages(w, "h1")[0]
	home.Units[game.UAxe] = axes
	bv.Units[game.ULightCav] = 300
	seen := game.UnitCounts{game.UAxe: axes}
	w.Brain("bot_0").Memory.Known[home.ID] = &game.KnownVillage{ID: home.ID, X: home.X, Y: home.Y, OwnerID: "h1", Units: &seen}
	c, err := game.SendCommand(w, "h1", home.ID, bv.X, bv.Y, game.UnitCounts{game.UAxe: axes}, "attack", game.SendOpts{})
	if err != nil {
		t.Fatal(err)
	}
	return w, bv, c
}

func TestBotInterceptsWeakIncomingAttack(t *testing.T) {
	w, bv, c := botUnderAttack(t, 10, 0.9)
	game.PlanFieldBattles(w, "bot_0")
	got := interceptsFrom(w, bv.ID)
	if len(got) != 1 || got[0].Intercept.CommandID != c.ID {
		t.Fatalf("intercepts = %+v", got)
	}
	game.PlanFieldBattles(w, "bot_0")
	if len(interceptsFrom(w, bv.ID)) != 1 {
		t.Fatal("one interception per army")
	}
}

func TestBotLeavesStrongAttackToDefence(t *testing.T) {
	w, bv, _ := botUnderAttack(t, 1000, 0.9)
	game.PlanFieldBattles(w, "bot_0")
	if got := interceptsFrom(w, bv.ID); len(got) != 0 {
		t.Fatalf("a bot does not throw its cavalry at a big army: %+v", got)
	}
}

func TestBotIgnoresBotArmies(t *testing.T) {
	cfg := game.DefaultWorldConfig
	cfg.BotCount, cfg.BeginnerProtectionDays = 2, 0
	failOnBotErrors(t)
	w, _ := gametest.NewWorldWithHuman(11, cfg)
	a, b := game.PlayerVillages(w, "bot_0")[0], game.PlayerVillages(w, "bot_1")[0]
	w.Brain("bot_1").Skill = 0.9
	a.Units[game.UAxe], b.Units[game.ULightCav] = 10, 300
	if _, err := game.SendCommand(w, "bot_0", a.ID, b.X, b.Y, game.UnitCounts{game.UAxe: 10}, "attack", game.SendOpts{}); err != nil {
		t.Fatal(err)
	}
	game.PlanFieldBattles(w, "bot_1")
	if got := interceptsFrom(w, b.ID); len(got) != 0 {
		t.Fatalf("bots leave each other's armies alone on the road: %+v", got)
	}
}

func TestStrongBotScoutsPassingHumanAttack(t *testing.T) {
	cfg := game.InstantWorldConfig
	cfg.BotCount, cfg.BeginnerProtectionDays = 1, 0
	failOnBotErrors(t)
	w, home := gametest.NewWorldWithHuman(13, cfg)
	w.Brain("bot_0").Skill = 0.9
	bv := game.PlayerVillages(w, "bot_0")[0]
	bv.Units[game.UScout] = 20
	// h1 attacks a barbarian village on the far side of the bot.
	size := w.Config().MapSize
	bx, by := min(size-1, max(0, 2*bv.X-home.X)), min(size-1, max(0, 2*bv.Y-home.Y))
	for game.VillageExistsAt(w, bx, by) {
		bx = (bx + 1) % size
	}
	w.PutVillage(game.CreateVillage(bx, by, "", "", w.Now()))
	home.Units[game.UAxe] = 50
	c, err := game.SendCommand(w, "h1", home.ID, bx, by, game.UnitCounts{game.UAxe: 50}, "attack", game.SendOpts{})
	if err != nil {
		t.Fatal(err)
	}
	w.SetNow((c.DepartedAt + c.ArrivesAt) / 2)
	game.PlanFieldBattles(w, "bot_0")
	got := interceptsFrom(w, bv.ID)
	if len(got) != 1 || !got[0].Intercept.Scout || got[0].Units[game.UScout] > 5 {
		t.Fatalf("a strong bot scouts a passing human army with a few scouts: %+v", got)
	}
	game.PlanFieldBattles(w, "bot_0")
	if len(interceptsFrom(w, bv.ID)) != 1 {
		t.Fatal("once per army")
	}
}
