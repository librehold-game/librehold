package game_test

import (
	"bytes"
	"reflect"
	"testing"

	"librehold/internal/game"
	"librehold/internal/game/gametest"
)

func TestHiddenSeed(t *testing.T) {
	w, _ := gametest.NewTestWorld(7)
	if got, want := game.HiddenSeed(w, "luck", 3), game.HashSeed(w.Seed(), "luck", 3); got != want {
		t.Fatalf("without a secret %d, want the public seed's %d", got, want)
	}
	a, b := bytes.Repeat([]byte{1}, 32), bytes.Repeat([]byte{2}, 32)
	w.SetSecret(a)
	first := game.HiddenSeed(w, "luck", 3)
	if first == game.HashSeed(w.Seed(), "luck", 3) {
		t.Fatal("a world with a secret still rolls from its public seed")
	}
	if game.HiddenSeed(w, "luck", 3) != first {
		t.Fatal("the same parts roll differently")
	}
	if game.HiddenSeed(w, "luck", 4) == first || game.HiddenSeed(w, "luck3") == first {
		t.Fatal("other parts roll the same")
	}
	w.SetSecret(b)
	if game.HiddenSeed(w, "luck", 3) == first {
		t.Fatal("another secret rolls the same")
	}
}

func TestBattleLuckComesFromTheSecret(t *testing.T) {
	luck := func(secret []byte) float64 {
		w, v := gametest.NewTestWorld(12)
		w.SetSecret(secret)
		v.Units[game.UAxe] = 50
		tx, ty := nearestUnstoredBarbarian(t, w, v, 1)
		c := mustSend(t, w, v, tx, ty, game.UnitCounts{game.UAxe: 50}, "attack")
		advance(t, w, c.ArrivesAt)
		return lastReport(w, "h1").Luck
	}
	public, a, b := luck(nil), luck(bytes.Repeat([]byte{1}, 32)), luck(bytes.Repeat([]byte{2}, 32))
	if a == public || a == b || luck(bytes.Repeat([]byte{1}, 32)) != a {
		t.Fatalf("luck without a secret %v, with two secrets %v and %v", public, a, b)
	}
}

func TestBotPersonalitiesComeFromTheSecret(t *testing.T) {
	brain := func(secret []byte) *game.BotBrain {
		cfg := gametest.TestConfig()
		cfg.BotCount = 1
		w := gametest.NewMemWorld(103, cfg, monday)
		w.SetSecret(secret)
		if err := game.NewWorld(w); err != nil {
			t.Fatal(err)
		}
		return w.Brain("bot_0")
	}
	public := brain(nil)
	if !reflect.DeepEqual(public, game.CreateBotBrain(103, "bot_0", gametest.TestConfig().Difficulty)) {
		t.Fatal("without a secret the brain is not the public seed's")
	}
	a, b := brain(bytes.Repeat([]byte{1}, 32)), brain(bytes.Repeat([]byte{2}, 32))
	if a.Skill == public.Skill || a.Skill == b.Skill || brain(bytes.Repeat([]byte{1}, 32)).Skill != a.Skill {
		t.Fatalf("skill without a secret %v, with two secrets %v and %v", public.Skill, a.Skill, b.Skill)
	}
}
