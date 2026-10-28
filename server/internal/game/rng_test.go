package game

import (
	"encoding/json"
	"os"
	"testing"
)

type rngFixture struct {
	Mulberry32 map[string][]float64 `json:"mulberry32"`
	HashSeed   []struct {
		Parts []any  `json:"parts"`
		Hash  uint32 `json:"hash"`
	} `json:"hashSeed"`
}

func loadRNGFixture(t *testing.T) rngFixture {
	t.Helper()
	raw, err := os.ReadFile("testdata/golden/rng.json")
	if err != nil {
		t.Fatal(err)
	}
	var f rngFixture
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatal(err)
	}
	return f
}

func TestRNGGolden(t *testing.T) {
	f := loadRNGFixture(t)
	seeds := map[string]uint32{"0": 0, "1": 1, "12345": 12345, "4294967295": 4294967295}
	if len(f.Mulberry32) != len(seeds) || len(f.HashSeed) == 0 {
		t.Fatal("fixture")
	}
	for name, seed := range seeds {
		r := Mulberry32(seed)
		f.Mulberry32[name] = make([]float64, 10)
		for i := range f.Mulberry32[name] {
			f.Mulberry32[name][i] = r()
		}
	}
	for i, c := range f.HashSeed {
		f.HashSeed[i].Hash = HashSeed(c.Parts...)
	}
	Golden(t, "rng", f)
}

func TestRandIntCoversRangeInclusive(t *testing.T) {
	r := Mulberry32(7)
	seen := map[int]bool{}
	for range 2000 {
		seen[RandInt(r, -2, 2)] = true
	}
	for v := -2; v <= 2; v++ {
		if !seen[v] {
			t.Errorf("RandInt never produced %d", v)
		}
	}
	if len(seen) != 5 {
		t.Errorf("RandInt produced values outside [-2,2]: %v", seen)
	}
}
