package game_test

import (
	"encoding/json"
	"maps"
	"slices"
	"testing"

	"librehold/internal/game"
)

func TestBalanceExportHoldsOnlyBrowserTables(t *testing.T) {
	want := []string{"BONUS_BALANCE", "BUILDINGS", "BUILDING_BALANCE", "BUILDING_ORDER", "COMBAT_BALANCE", "COMMAND_BALANCE",
		"CONQUEST_BALANCE", "DEFAULT_WORLD_CONFIG", "FARM_BALANCE", "MAP", "QUESTS", "RESEARCH_BALANCE", "RESEARCH_UNLOCK_COST",
		"RESOURCE_BALANCE", "UNITS", "UNIT_BALANCE", "WORLD_LIMITS"}
	got := slices.Sorted(maps.Keys(game.BalanceExport()))
	if !slices.Equal(got, want) {
		t.Fatalf("keys %v, want %v", got, want)
	}
}

func TestBalanceExportQuestsCarryNoFunctions(t *testing.T) {
	raw, err := json.Marshal(game.BalanceExport()["QUESTS"])
	if err != nil {
		t.Fatal(err)
	}
	var qs []struct {
		ID       string          `json:"id"`
		Requires []string        `json:"requires"`
		Reward   json.RawMessage `json:"reward"`
	}
	if err := json.Unmarshal(raw, &qs); err != nil {
		t.Fatal(err)
	}
	if len(qs) != len(game.Quests) || qs[0].ID != "wood1" || string(qs[0].Reward) != `{"resources":{"wood":60,"clay":40,"iron":20}}` {
		t.Fatalf("quests export: %.200s", raw)
	}
	for i, q := range qs {
		if q.ID != game.Quests[i].ID || !slices.Equal(q.Requires, game.Quests[i].Requires) {
			t.Fatalf("quest %d: %s %v", i, q.ID, q.Requires)
		}
	}
}

func TestWorldLimitsDeclareSightRadius(t *testing.T) {
	if game.WorldLimits.SightRadius != (game.Range{Min: 0, Max: 10}) {
		t.Fatal(game.WorldLimits.SightRadius)
	}
}
