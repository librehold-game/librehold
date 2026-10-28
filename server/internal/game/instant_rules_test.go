package game_test

import (
	"errors"
	"testing"

	"librehold/internal/game"
	"librehold/internal/game/gametest"
)

func noCoinsConfig() game.WorldConfig {
	cfg := game.InstantWorldConfig
	cfg.NoCoins = true
	return cfg
}

func richVillage(v *game.Village) {
	v.Buildings[game.BAcademy] = 3
	v.Units[game.UNoble] = 0
	v.Resources = game.Resources{Wood: 50000, Clay: 50000, Iron: 50000}
}

func hasBlocker(bs []game.Blocker, kind string) bool {
	for _, b := range bs {
		if b.Kind() == kind {
			return true
		}
	}
	return false
}

func TestNoCoinsNobleNeedsNoCoins(t *testing.T) {
	w, v := gametest.NewWorldWithHuman(3, noCoinsConfig())
	richVillage(v)
	if hasBlocker(game.CheckRecruit(w, v, game.UNoble, 1).Blockers, "coins") {
		t.Fatal("a coin blocker in a no-coin world")
	}
	job, err := game.EnqueueRecruit(w, "h1", v.ID, game.UNoble, 1)
	if err != nil {
		t.Fatal(err)
	}
	if got := w.Player("h1").Coins; got != 0 {
		t.Fatalf("coins after recruiting %d", got)
	}
	if _, err := game.CancelRecruit(w, "h1", v.ID, job.ID); err != nil {
		t.Fatal(err)
	}
	if got := w.Player("h1").Coins; got != 0 {
		t.Fatalf("coins after cancelling %d", got)
	}
	if got := game.MaxRecruitable(w, v, game.UNoble); got < 1 {
		t.Fatalf("max recruitable %d", got)
	}
}

func TestNoCoinsMintRefused(t *testing.T) {
	w, v := gametest.NewWorldWithHuman(3, noCoinsConfig())
	richVillage(v)
	err := game.MintCoin(w, "h1", v.ID)
	var rule *game.RuleError
	if !errors.As(err, &rule) || rule.Code != "noCoins" {
		t.Fatalf("mint: %v", err)
	}
	if w.Player("h1").Coins != 0 {
		t.Fatal("coins minted")
	}
}

func TestLongWorldStillUsesCoins(t *testing.T) {
	w, v := gametest.NewWorldWithHuman(3, game.DefaultWorldConfig)
	richVillage(v)
	var coin game.Blocker
	for _, b := range game.CheckRecruit(w, v, game.UNoble, 1).Blockers {
		if b.Kind() == "coins" {
			coin = b
		}
	}
	if coin == nil || coin["needed"] != 1 {
		t.Fatalf("coin blocker %v", coin)
	}
	if err := game.MintCoin(w, "h1", v.ID); err != nil {
		t.Fatal(err)
	}
	if hasBlocker(game.CheckRecruit(w, v, game.UNoble, 1).Blockers, "coins") {
		t.Fatal("still blocked after minting")
	}
}

func TestInstantNoblePrice(t *testing.T) {
	cfg := noCoinsConfig()
	cfg.NobleCost = game.Resources{Wood: 1000, Clay: 2000, Iron: 3000}
	w, v := gametest.NewWorldWithHuman(3, cfg)
	richVillage(v)
	if got := game.CheckRecruit(w, v, game.UNoble, 1).Cost; got != cfg.NobleCost {
		t.Fatalf("cost %+v", got)
	}
	if got := game.CheckRecruit(w, v, game.UNoble, 2).Cost; got != (game.Resources{Wood: 2000, Clay: 4000, Iron: 6000}) {
		t.Fatalf("cost of two %+v", got)
	}
}

func TestOwnedDropRange(t *testing.T) {
	cfg := noCoinsConfig()
	cfg.OwnedNobleDropMin, cfg.OwnedNobleDropMax = 50, 60
	if lo, hi := game.NobleDropRange(&cfg, true); lo != 50 || hi != 60 {
		t.Fatalf("owned %d-%d", lo, hi)
	}
	if lo, hi := game.NobleDropRange(&cfg, false); lo != 20 || hi != 35 {
		t.Fatalf("barbarian %d-%d", lo, hi)
	}
	if got := game.NoblesNeededFor(&cfg, true, 100); got != 2 {
		t.Fatalf("nobles for 100 owned loyalty: %d", got)
	}
	if got := game.NoblesNeededFor(&cfg, false, 15); got != 1 {
		t.Fatalf("nobles for a barbarian: %d", got)
	}
}

func TestTwoNoblesTakeAnOwnedVillage(t *testing.T) {
	cfg := noCoinsConfig()
	cfg.OwnedNobleDropMin, cfg.OwnedNobleDropMax = 50, 60
	w, home := gametest.NewWorldWithHuman(3, cfg)
	target := game.CreateVillage(home.X+3, home.Y, "Target", "", w.Now())
	w.PutVillage(target)
	game.SetOwner(w, target, game.Ref("h1"))
	home.Buildings[game.BAcademy] = 1
	target.Loyalty = 100

	if _, conquered := game.ApplyNobles(w, target, &game.Command{ID: "c1"}, 1); conquered {
		t.Fatal("one noble took an owned village")
	}
	if _, conquered := game.ApplyNobles(w, target, &game.Command{ID: "c2"}, 1); !conquered {
		t.Fatalf("two nobles failed, loyalty %v", target.Loyalty)
	}

	barb := game.CreateVillage(home.X+6, home.Y, "", "", w.Now())
	barb.Loyalty = cfg.BarbarianLoyalty
	w.PutVillage(barb)
	if _, conquered := game.ApplyNobles(w, barb, &game.Command{ID: "c3"}, 1); !conquered {
		t.Fatal("one noble failed on a barbarian")
	}
}

func TestNoCoinsQuestChainCompletes(t *testing.T) {
	w, _ := gametest.NewWorldWithHuman(3, noCoinsConfig())
	q := game.QuestByID["coin"]
	if q == nil || !q.Condition(w, "h1") {
		t.Fatal("coin quest open in a no-coin world")
	}
	w2, _ := gametest.NewWorldWithHuman(3, game.DefaultWorldConfig)
	if game.QuestByID["coin"].Condition(w2, "h1") {
		t.Fatal("coin quest done in a long world without minting")
	}
}

func TestInstantAcademyHoldsOneMoreNoble(t *testing.T) {
	w, v := gametest.NewWorldWithHuman(3, noCoinsConfig())
	richVillage(v)
	v.Buildings[game.BAcademy] = 1
	if got := game.NobleLimit(w.Config(), v); got != 2 {
		t.Fatalf("academy 1 holds %d nobles in an instant world", got)
	}
	v.Buildings[game.BAcademy] = 0
	if got := game.NobleLimit(w.Config(), v); got != 0 {
		t.Fatalf("no academy holds %d nobles", got)
	}
	long := game.DefaultWorldConfig
	v.Buildings[game.BAcademy] = 1
	if got := game.NobleLimit(&long, v); got != 1 {
		t.Fatalf("academy 1 holds %d nobles in a long world", got)
	}
}
