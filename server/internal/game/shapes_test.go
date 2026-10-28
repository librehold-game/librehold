package game

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

type shapesFixture struct {
	Villages []json.RawMessage          `json:"villages"`
	Players  []json.RawMessage          `json:"players"`
	Brains   map[string]json.RawMessage `json:"brains"`
	Commands []json.RawMessage          `json:"commands"`
	Reports  []json.RawMessage          `json:"reports"`
	Human    json.RawMessage            `json:"human"`
	News     []json.RawMessage          `json:"news"`
	Intel    json.RawMessage            `json:"intel"`
}

// dropZeroPartialUnits removes zero entries from partial unit records (objects whose keys are all unit ids
// and that are not full 12-key records): UnitCounts drops `{spear: 0}`, and every reader treats a missing key
// as 0.
func dropZeroPartialUnits(v any) any {
	switch x := v.(type) {
	case map[string]any:
		allUnits := len(x) > 0
		for k := range x {
			if _, ok := ParseUnit(k); !ok {
				allUnits = false
			}
		}
		if allUnits && len(x) < NumUnits {
			for k, val := range x {
				if n, ok := val.(float64); ok && n == 0 {
					delete(x, k)
				}
			}
			return x
		}
		for k, val := range x {
			x[k] = dropZeroPartialUnits(val)
		}
		return x
	case []any:
		for i := range x {
			x[i] = dropZeroPartialUnits(x[i])
		}
		return x
	}
	return v
}

func roundTrip[T any](t *testing.T, name string, raw json.RawMessage, drop ...string) {
	t.Helper()
	var want map[string]any
	if err := json.Unmarshal(raw, &want); err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	for _, k := range drop {
		delete(want, k)
	}
	dropZeroPartialUnits(want)
	var e T
	if err := json.Unmarshal(raw, &e); err != nil {
		t.Fatalf("%s: unmarshal into %T: %v", name, e, err)
	}
	out, err := json.Marshal(e)
	if err != nil {
		t.Fatalf("%s: marshal: %v", name, err)
	}
	var got map[string]any
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		wj, _ := json.Marshal(want)
		t.Errorf("%s: round trip differs\n go: %s\n ts: %s", name, out, wj)
	}
}

func TestEntityJSONRoundTrips(t *testing.T) {
	raw, err := os.ReadFile("testdata/golden/shapes.json")
	if err != nil {
		t.Fatal(err)
	}
	var f shapesFixture
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatal(err)
	}
	if len(f.Villages) < 2 || len(f.Players) < 2 || len(f.Commands) < 2 || len(f.Reports) < 1 {
		t.Fatalf("fixture too small: %d villages, %d players, %d commands, %d reports",
			len(f.Villages), len(f.Players), len(f.Commands), len(f.Reports))
	}
	for _, v := range f.Villages {
		roundTrip[Village](t, "village", v)
	}
	for _, p := range f.Players {
		roundTrip[Player](t, "player", p)
	}
	for id, b := range f.Brains {
		roundTrip[BotBrain](t, "brain "+id, b)
	}
	for _, c := range f.Commands {
		roundTrip[Command](t, "command", c)
	}
	for _, r := range f.Reports {
		roundTrip[Report](t, "report", r)
	}
	roundTrip[HumanState](t, "human", f.Human)
	for _, n := range f.News {
		roundTrip[NewsItem](t, "news", n)
	}
}

func TestIntelRoundTrips(t *testing.T) {
	raw, err := os.ReadFile("testdata/golden/shapes.json")
	if err != nil {
		t.Fatal(err)
	}
	var f shapesFixture
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatal(err)
	}
	if len(f.Intel) < 10 {
		t.Fatalf("no intel in the fixture: %s", f.Intel)
	}
	roundTrip[Intel](t, "intel", f.Intel)
}
