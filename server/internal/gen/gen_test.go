package gen_test

import (
	"bytes"
	"encoding/json"
	"maps"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"testing"

	"librehold/internal/game"
	"librehold/internal/gen"
)

func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("no caller info")
	}
	return filepath.Join(filepath.Dir(file), "..", "..", "..")
}

func TestGeneratedFilesAreCurrent(t *testing.T) {
	root := repoRoot(t)
	files := gen.Files()
	if len(files) == 0 {
		t.Fatal("no generated files")
	}
	for _, f := range files {
		want, err := f.Bytes()
		if err != nil {
			t.Fatalf("%s: %v", f.Path, err)
		}
		got, err := os.ReadFile(filepath.Join(root, f.Path))
		if err != nil || !bytes.Equal(got, want) {
			t.Errorf("%s is stale: run npm run gen", f.Path)
		}
	}
}

func TestBalanceJSONIsCanonical(t *testing.T) {
	b, err := gen.BalanceJSON()
	if err != nil {
		t.Fatal(err)
	}
	var v map[string]any
	if err := json.Unmarshal(b, &v); err != nil {
		t.Fatal(err)
	}
	again, _ := json.MarshalIndent(v, "", "  ")
	if !bytes.Equal(append(again, '\n'), b) {
		t.Fatal("balance.json is not in canonical form")
	}
	if got, want := slices.Sorted(maps.Keys(v)), slices.Sorted(maps.Keys(game.BalanceExport())); !slices.Equal(got, want) {
		t.Fatalf("keys %v, want %v", got, want)
	}
}

func TestRulesJSONCoversEveryTable(t *testing.T) {
	b, err := gen.RulesJSON()
	if err != nil {
		t.Fatal(err)
	}
	var v map[string][]map[string]any
	if err := json.Unmarshal(b, &v); err != nil {
		t.Fatal(err)
	}
	want := []string{"buildCost", "buildTime", "farm", "hide", "loyalty", "nobleLimit", "noblesNeeded", "production", "recruitTime",
		"researchBonus", "researchCost", "storage", "travel"}
	if got := slices.Sorted(maps.Keys(v)); !slices.Equal(got, want) {
		t.Fatalf("keys %v", got)
	}
	if len(v["travel"]) != 6*game.NumUnits*4 {
		t.Fatalf("%d travel cases", len(v["travel"]))
	}
	for k, cases := range v {
		if len(cases) == 0 {
			t.Errorf("%s is empty", k)
		}
	}
	if !slices.ContainsFunc(gen.Files(), func(f gen.File) bool { return f.Path == "tests/fixtures/rules.json" }) {
		t.Error("rules.json is not generated")
	}
}

func TestWorldFixturesAreDeterministic(t *testing.T) {
	a, err := gen.WorldFixture("test-bot")
	if err != nil {
		t.Fatal(err)
	}
	b, err := gen.WorldFixture("test-bot")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(a, b) {
		t.Fatal("two runs differ")
	}
}

type fixtureVillage struct {
	ID      string         `json:"id"`
	X       int            `json:"x"`
	Y       int            `json:"y"`
	OwnerID string         `json:"ownerId"`
	Units   map[string]int `json:"units"`
}

type worldFixture struct {
	State struct {
		Me       string                     `json:"me"`
		Villages map[string]fixtureVillage  `json:"villages"`
		Players  map[string]json.RawMessage `json:"players"`
	} `json:"state"`
	Map         []json.RawMessage `json:"map"`
	FarmTargets []json.RawMessage `json:"farmTargets"`
	BotVillage  *fixtureVillage   `json:"botVillage"`
}

func loadWorldFixture(t *testing.T, kind string) worldFixture {
	t.Helper()
	raw, err := gen.WorldFixture(kind)
	if err != nil {
		t.Fatal(err)
	}
	var f worldFixture
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatal(err)
	}
	return f
}

func TestWorldFixtureShape(t *testing.T) {
	d := loadWorldFixture(t, "default")
	if d.State.Me != "h1" || len(d.State.Villages) != 1 || len(d.State.Players) != 101 || len(d.Map) == 0 || d.BotVillage != nil {
		t.Fatalf("default: me %q, %d villages, %d players, %d tiles", d.State.Me, len(d.State.Villages), len(d.State.Players), len(d.Map))
	}
	for _, v := range d.State.Villages {
		if v.Units["spear"] == 0 {
			t.Error("the default world keeps the starting guard")
		}
	}
	tb := loadWorldFixture(t, "test-bot")
	if tb.BotVillage == nil || tb.BotVillage.OwnerID != "bot_test" {
		t.Fatalf("bot village %+v", tb.BotVillage)
	}
	var home fixtureVillage
	for _, v := range tb.State.Villages {
		home = v
		for u, n := range v.Units {
			if n != 0 {
				t.Errorf("test world village has %d %s", n, u)
			}
		}
	}
	if dx, dy := tb.BotVillage.X-home.X, tb.BotVillage.Y-home.Y; dx*dx+dy*dy > 400 {
		t.Fatalf("bot village %d|%d far from home %d|%d", tb.BotVillage.X, tb.BotVillage.Y, home.X, home.Y)
	}
	if _, ok := tb.State.Players["bot_test"]; !ok {
		t.Error("bot_test is not in the players")
	}
	for _, kind := range []string{"default", "test", "test-bot"} {
		path := "tests/fixtures/world-" + kind + ".json"
		if !slices.ContainsFunc(gen.Files(), func(f gen.File) bool { return f.Path == path }) {
			t.Errorf("%s is not generated", path)
		}
	}
}
