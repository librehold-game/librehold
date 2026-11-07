package game_test

import (
	"encoding/json"
	"testing"

	"librehold/internal/game"
)

// The merchant curve: one per level up to 11, then quadratic, matching Tribal Wars exactly.
func TestMerchantsForLevel(t *testing.T) {
	// The published Tribal Wars market table, levels 1-25.
	tw := []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 14, 19, 26, 35, 46, 59, 74, 91, 110, 131, 154, 179, 206, 235}
	for i, want := range tw {
		level := i + 1
		if got := game.MerchantsForLevel(level); got != want {
			t.Errorf("level %d: got %d merchants, Tribal Wars gives %d", level, got, want)
		}
	}
	// A market gives nothing before it is built.
	if got := game.MerchantsForLevel(0); got != 0 {
		t.Errorf("level 0: got %d, want 0", got)
	}
}

// The curve has to keep the early game identical: up to the breakpoint it must be a plain read of the level.
func TestMerchantsLinearUntilTheBreakpoint(t *testing.T) {
	for level := 0; level <= game.TradeBalance.MerchantCurveAt; level++ {
		if got := game.MerchantsForLevel(level); got != level {
			t.Errorf("level %d: got %d, want one merchant per level (%d)", level, got, level)
		}
	}
	// And it must actually accelerate past it, or the top levels stay worthless.
	at := game.TradeBalance.MerchantCurveAt
	if game.MerchantsForLevel(at+1) <= at+1 {
		t.Fatalf("level %d should give more than %d merchants", at+1, at+1)
	}
}

// The point of the curve: a maxed market must move more than a *level 25* storehouse holds of one resource, so trading
// stays useful in the late game instead of being capped at pocket change. A market maxes at 25 while the storehouse
// goes to 30, so the level-25 storehouse is the fair yardstick.
func TestMaxedMarketOutshipsAStorehouse(t *testing.T) {
	level := game.BuildingDefs[game.BMarket].MaxLevel
	merchants := game.MerchantsForLevel(level)
	capacity := float64(merchants) * game.CommandBalance.MerchantCapacity
	storehouse := game.StorageCapacity(level, nil) // the same level, not the storehouse's own (higher) cap
	if capacity <= storehouse {
		t.Errorf("a level %d market ships %v, no more than one level-%d storehouse of %v — trading would be pointless",
			level, capacity, level, storehouse)
	}
	linear := float64(level) * game.CommandBalance.MerchantCapacity
	t.Logf("level %d market: %d merchants, %v resources (the old linear curve gave %v; a level-%d storehouse holds %v)",
		level, merchants, capacity, linear, level, storehouse)
	if capacity < 1.5*storehouse {
		t.Errorf("the curve only reaches %.2fx a level-%d storehouse; the late game still hits the merchant cap",
			capacity/storehouse, level)
	}
}

// TestTradingDefaultsToOffForStoredWorlds pins the migration contract: a world saved before the setting existed has no
// tradingEnabled key, and json.Unmarshal leaves the zero value, which must mean off so no running world changes
// behaviour. New worlds come from DefaultWorldConfig, which turns it on.
func TestTradingDefaultsToOffForStoredWorlds(t *testing.T) {
	// A config exactly as an older world stored it: no tradingEnabled key at all.
	stored := `{"worldSpeed":8,"unitSpeed":1,"mapSize":200,"botCount":100}`
	var cfg game.WorldConfig
	if err := json.Unmarshal([]byte(stored), &cfg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if cfg.TradingEnabled {
		t.Fatal("a world stored without the key must read as trading off")
	}
	if !game.DefaultWorldConfig.TradingEnabled {
		t.Fatal("a new world must start with trading on")
	}
	// The key survives a round trip, so the admin can switch it on and it stays on.
	cfg.TradingEnabled = true
	round, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var back game.WorldConfig
	if err := json.Unmarshal(round, &back); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !back.TradingEnabled {
		t.Fatal("tradingEnabled must survive a store round trip, including false→/true")
	}
}
