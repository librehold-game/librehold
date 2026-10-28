package gen

import (
	"context"
	"fmt"

	"librehold/internal/adapters/memory"
	"librehold/internal/adapters/security"
	"librehold/internal/app"
	"librehold/internal/game"
	"librehold/internal/game/gametest"
)

// The UI tests run on worlds the server built: tests/fixtures/world-<kind>.json holds what GET /state,
// GET /map (the window around the home village) and GET /farm-targets return for the human, plus for "test-bot"
// the bot village next door the map and army tests aim at.
//
//	default   the default world settings: 100 bots, the human with the starting guard
//	test      world speed 1, no bots, the human's village without units
//	test-bot  test plus a village of player bot_test on the nearest free field

const (
	fixtureNowMs  = 1_700_000_000_000
	fixtureSeed   = 12345
	fixtureRadius = 10
)

type fixedClock struct{}

func (fixedClock) NowMs() int64 { return fixtureNowMs }

var fixtureKinds = []string{"default", "test", "test-bot"}

// WorldFixture is tests/fixtures/world-<kind>.json.
func WorldFixture(kind string) ([]byte, error) {
	var cfg game.WorldConfig
	switch kind {
	case "default":
		cfg = game.DefaultWorldConfig
	case "test", "test-bot":
		cfg = gametest.TestConfig()
	default:
		return nil, fmt.Errorf("unknown world fixture %q", kind)
	}
	ctx := context.Background()
	clock := fixedClock{}
	worlds := memory.NewWorlds()
	svc := app.NewService(memory.NewAccounts(), worlds, memory.NewCounters(clock), clock, security.NewArgon2(), security.NewTokens(), app.Config{})
	row := app.WorldRow{Name: "Fixture", Status: "open", Seed: fixtureSeed, Secret: make([]byte, 32), Config: cfg, StartedAtMs: fixtureNowMs}
	wid, err := worlds.Create(ctx, row, func(w app.WorldTx) error { return game.NewWorld(w) })
	if err != nil {
		return nil, err
	}
	ala := &app.Account{ID: 1, Username: "Ala"}
	me, err := svc.JoinWorld(ctx, ala, wid, game.QuadrantRandom)
	if err != nil {
		return nil, err
	}
	var home, bot *game.Village
	err = worlds.Run(ctx, wid, func(w app.WorldTx) error {
		home = w.Village(w.Player(me).VillageIDs[0])
		if kind == "default" {
			return nil
		}
		home.Units = game.Units{}
		if kind == "test-bot" {
			gametest.NewBotPlayer(w, "bot_test").Name = "Test bot"
			bot = gametest.MakeBotVillage(w, home)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	st, err := svc.State(ctx, ala, wid)
	if err != nil {
		return nil, err
	}
	tiles, err := svc.MapRegion(ctx, ala, wid, home.X-fixtureRadius, home.Y-fixtureRadius, home.X+fixtureRadius, home.Y+fixtureRadius)
	if err != nil {
		return nil, err
	}
	targets, err := svc.FarmTargets(ctx, ala, wid, home.ID)
	if err != nil {
		return nil, err
	}
	out := map[string]any{"state": st, "map": tiles, "farmTargets": targets}
	if bot != nil {
		out["botVillage"] = bot
	}
	return canonical(out)
}
