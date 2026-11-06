package postgres_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"librehold/internal/adapters/postgres"
	"librehold/internal/adapters/postgres/pgtest"
	"librehold/internal/game"
)

func TestMain(m *testing.M) { pgtest.Main(m) }

var schemaTables = []string{
	"accounts", "sessions", "worlds", "players", "bot_brains", "human_states", "villages",
	"village_support", "commands", "events", "reports", "intel", "scheduled_sends", "news", "conquests", "public_events",
	"problem_reports", "timeline", "timeline_packed", "host_settings",
}

func TestMigrateCreatesSchema(t *testing.T) {
	ctx := context.Background()
	pool := pgtest.Empty(t)
	if err := postgres.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	for _, table := range schemaTables {
		var exists bool
		if err := pool.QueryRow(ctx, `select exists (select from information_schema.tables where table_name = $1)`, table).Scan(&exists); err != nil {
			t.Fatal(err)
		}
		if !exists {
			t.Errorf("table %s missing", table)
		}
	}
	var codes bool
	if err := pool.QueryRow(ctx, `select exists (select from information_schema.tables where table_name = 'access_codes')`).Scan(&codes); err != nil || codes {
		t.Fatalf("access_codes still exists (%v)", err)
	}
	var before int
	if err := pool.QueryRow(ctx, `select count(*) from goose_db_version`).Scan(&before); err != nil {
		t.Fatal(err)
	}
	if err := postgres.Migrate(ctx, pool); err != nil {
		t.Fatalf("second migrate: %v", err)
	}
	var after int
	if err := pool.QueryRow(ctx, `select count(*) from goose_db_version`).Scan(&after); err != nil {
		t.Fatal(err)
	}
	if after != before {
		t.Fatalf("second migrate changed the version table: %d → %d", before, after)
	}
}

func TestMigration0003WipesWorldsAndUnlinksDeletedAccounts(t *testing.T) {
	ctx := context.Background()
	pool := pgtest.Empty(t)
	if err := postgres.MigrateTo(ctx, pool, 2); err != nil {
		t.Fatal(err)
	}
	var acct int64
	if err := pool.QueryRow(ctx, `insert into accounts (username, username_lower, password_hash) values ('Ala', 'ala', 'h') returning id`).Scan(&acct); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `insert into worlds (name, status, seed, config, started_at_ms) values ('stary', 'open', 1, '{}', 0)`); err != nil {
		t.Fatal(err)
	}
	if err := postgres.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	var worlds int
	if err := pool.QueryRow(ctx, `select count(*) from worlds`).Scan(&worlds); err != nil || worlds != 0 {
		t.Fatalf("worlds left: %d (%v)", worlds, err)
	}
	var locale, key string
	if err := pool.QueryRow(ctx, `select locale, player_key from accounts where id = $1`, acct).Scan(&locale, &key); err != nil || locale != "en" || len(key) != 32 {
		t.Fatalf("existing account %q %q %v", locale, key, err)
	}
	var wid int64
	if err := pool.QueryRow(ctx, `insert into worlds (name, status, seed, secret, config, started_at_ms) values ('nowy', 'open', 1, sha256('x'), '{}', 0) returning id`).Scan(&wid); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `insert into players (world_id, id, account_id, name, is_human, status, data) values ($1, 'h1', $2, 'Ala', true, 'active', '{}')`, wid, acct); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `delete from accounts where id = $1`, acct); err != nil {
		t.Fatalf("deleting an account with a player: %v", err)
	}
	var linked *int64
	if err := pool.QueryRow(ctx, `select account_id from players where world_id = $1`, wid).Scan(&linked); err != nil || linked != nil {
		t.Fatalf("player still linked: %v %v", linked, err)
	}
}

func TestPgtestGivesMigratedIsolatedDatabases(t *testing.T) {
	ctx := context.Background()
	a, b := pgtest.New(t), pgtest.New(t)
	if _, err := a.Exec(ctx, `insert into accounts (username, username_lower, password_hash) values ('x', 'x', 'h')`); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := b.QueryRow(ctx, `select count(*) from accounts`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("databases not isolated: %d %v", n, err)
	}
}

// Armies already on the road get the box of their leg, as the store writes it for new ones.
func TestMigration0016BoxesTheLegsOfArmiesOnTheRoad(t *testing.T) {
	ctx := context.Background()
	pool := pgtest.Empty(t)
	if err := postgres.MigrateTo(ctx, pool, 15); err != nil {
		t.Fatal(err)
	}
	var wid int64
	if err := pool.QueryRow(ctx, `insert into worlds (name, status, seed, secret, config, started_at_ms) values ('w', 'open', 1, sha256('x'), '{}', 0) returning id`).Scan(&wid); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct{ id, typ, origin, target, data string }{
		{"c_attack", "attack", "v:10:4", "v:3:2", `{}`},
		{"c_field", "return", "", "v:3:4", `{"point": {"x": 5.5, "y": 6}}`},
		{"c_intercept", "intercept", "v:3:4", "", `{"point": {"x": 7, "y": 8}}`},
		{"c_odd", "attack", "vil_fresh", "v:3:4", `{}`},
	} {
		if _, err := pool.Exec(ctx, `insert into commands (world_id, id, type, origin_id, target_id, home_id, arrives_at, data)
			values ($1, $2, $3, $4, $5, $4, 0, $6)`, wid, c.id, c.typ, c.origin, c.target, c.data); err != nil {
			t.Fatal(err)
		}
	}
	if err := postgres.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"c_attack": "(10,4),(3,2)", "c_field": "(5.5,6),(3,4)", "c_intercept": "(7,8),(3,4)", "c_odd": "<nil>"}
	for id, box := range want {
		var path *string
		if err := pool.QueryRow(ctx, `select path::text from commands where id = $1`, id).Scan(&path); err != nil {
			t.Fatal(err)
		}
		got := "<nil>"
		if path != nil {
			got = *path
		}
		if got != box {
			t.Errorf("%s: path %s, want %s", id, got, box)
		}
	}
}

// The trading tables and settings columns arrive as a plain additive migration: a deployment already at 0016 boots
// with its worlds untouched. The world config is not rewritten, so no new key appears — which is what makes
// TradingEnabled (absent = off) the right polarity for existing worlds.
func TestMigration0017AddsTradingWithoutTouchingStoredWorlds(t *testing.T) {
	ctx := context.Background()
	pool := pgtest.Empty(t)
	if err := postgres.MigrateTo(ctx, pool, 16); err != nil {
		t.Fatal(err)
	}
	var wid int64
	if err := pool.QueryRow(ctx, `insert into worlds (name, status, seed, secret, config, started_at_ms)
		values ('old', 'open', 1, sha256('x'), '{"worldSpeed":8,"mapSize":200}', 0) returning id`).Scan(&wid); err != nil {
		t.Fatal(err)
	}
	if err := postgres.Migrate(ctx, pool); err != nil {
		t.Fatalf("upgrade: %v", err)
	}
	var raw []byte
	if err := pool.QueryRow(ctx, `select config from worlds where id = $1`, wid).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	var cfg map[string]any
	if err := json.Unmarshal(raw, &cfg); err != nil {
		t.Fatal(err)
	}
	if _, ok := cfg["tradingEnabled"]; ok {
		t.Fatalf("the migration must not add tradingEnabled, got %v", cfg["tradingEnabled"])
	}
	// The new table exists, is empty, and belongs to the world by foreign key.
	var n int
	if err := pool.QueryRow(ctx, `select count(*) from trade_offers where world_id = $1`, wid).Scan(&n); err != nil {
		t.Fatalf("trade_offers missing after the upgrade: %v", err)
	}
	if n != 0 {
		t.Fatalf("expected no offers, got %d", n)
	}
}

// Rolling back 0017 must not destroy resources: posting an offer reserves the give side out of its village, so a
// plain "drop table" would take those resources with it and nobody would be refunded. The refund is capped at the
// storehouse, like every runtime refund, and the excess is lost.
func TestMigration0017DownRefundsOpenOffers(t *testing.T) {
	ctx := context.Background()
	pool := pgtest.Empty(t)
	if err := postgres.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	var wid int64
	if err := pool.QueryRow(ctx, `insert into worlds (name, status, seed, secret, config, started_at_ms)
		values ('w', 'open', 1, sha256('x'), '{}', 0) returning id`).Scan(&wid); err != nil {
		t.Fatal(err)
	}
	// Every village sits at storehouse 20, so the cap is the same for all of them.
	const storage = 20
	stored := game.StorageCapacity(storage, nil)
	coin := 0
	add := func(id string, wood, clay, iron float64) {
		t.Helper()
		coin++
		if _, err := pool.Exec(ctx, `insert into villages (world_id, id, x, y, name, points, owner_id, data)
			values ($1, $2, $3, $3, 'v', 0, 'h1', jsonb_build_object(
			  'resources', jsonb_build_object('wood', $4::float8, 'clay', $5::float8, 'iron', $6::float8),
			  'buildings', jsonb_build_object('storage', $7::int)))`, wid, id, coin, wood, clay, iron, storage); err != nil {
			t.Fatal(err)
		}
	}
	add("v:1:1", 100, 0, 7)
	add("v:2:2", 500, 0, 0)
	add("v:3:3", stored+5000, 0, 0) // loot left it above capacity
	post := func(id, origin string, wood, iron float64) {
		t.Helper()
		if _, err := pool.Exec(ctx, `insert into trade_offers (world_id, id, from_id, expires_at, data)
			values ($1, $2, 'h1', 0, jsonb_build_object('originId', $3::text,
			  'give', jsonb_build_object('wood', $4::float8, 'clay', 0::float8, 'iron', $5::float8)))`,
			wid, id, origin, wood, iron); err != nil {
			t.Fatal(err)
		}
	}
	post("to_a", "v:1:1", 30, 0)
	post("to_b", "v:1:1", 20, 5)
	post("to_c", "v:2:2", 100, 0)
	post("to_d", "v:3:3", 5000, 0)  // no room: clamped, the rest lost
	post("to_gone", "v:9:9", 50, 0) // the village was razed; this share has nowhere to go

	if err := postgres.MigrateDown(ctx, pool, 16); err != nil {
		t.Fatalf("rollback: %v", err)
	}

	get := func(id string) (wood, iron float64) {
		t.Helper()
		if err := pool.QueryRow(ctx, `select (data->'resources'->>'wood')::float8, (data->'resources'->>'iron')::float8
			from villages where world_id = $1 and id = $2`, wid, id).Scan(&wood, &iron); err != nil {
			t.Fatal(err)
		}
		return wood, iron
	}
	// v:1:1 was under capacity, so all 50 wood and the 5 iron come home.
	if wood, _ := get("v:1:1"); wood != 150 {
		t.Fatalf("v:1:1 wood: want 150, got %v", wood)
	}
	if _, iron := get("v:1:1"); iron != 12 {
		t.Fatalf("v:1:1 iron: want 12, got %v", iron)
	}
	if wood, _ := get("v:2:2"); wood != 600 {
		t.Fatalf("v:2:2 wood: want 600, got %v", wood)
	}
	// v:3:3 was already over capacity, so it is clamped down to the storehouse and the excess is lost.
	if wood, _ := get("v:3:3"); wood != stored {
		t.Fatalf("v:3:3 wood: want the capacity %v, got %v", stored, wood)
	}
	// The table is gone, so the game boots without it.
	var n int
	if err := pool.QueryRow(ctx, `select count(*) from information_schema.tables where table_name = 'trade_offers'`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("trade_offers still present: %d %v", n, err)
	}
}

// The migration cannot call game.StorageCapacity, so it reproduces the formula in SQL. This pins the copy to the real
// function at every level and both bonus states, so a balance change cannot silently make the rollback refund the
// wrong amount.
func TestMigration0017CapacityMatchesTheGame(t *testing.T) {
	ctx := context.Background()
	pool := pgtest.Empty(t)
	if err := postgres.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	var wid int64
	if err := pool.QueryRow(ctx, `insert into worlds (name, status, seed, secret, config, started_at_ms)
		values ('w', 'open', 1, sha256('x'), '{}', 0) returning id`).Scan(&wid); err != nil {
		t.Fatal(err)
	}
	storages := []int{0, 1, 2, 5, 11, 20, 25, 30}
	coin := 0
	for _, withBonus := range []bool{false, true} {
		for _, lvl := range storages {
			coin++
			id := fmt.Sprintf("v:%d:%t", lvl, withBonus)
			data := map[string]any{
				"resources": map[string]float64{"wood": 0, "clay": 0, "iron": 0},
				"buildings": map[string]int{"storage": lvl},
			}
			if withBonus {
				data["bonus"] = map[string]string{"kind": "storage"}
			}
			raw, err := json.Marshal(data)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := pool.Exec(ctx, `insert into villages (world_id, id, x, y, name, points, owner_id, data)
				values ($1, $2, $3, 1, 'v', 0, 'h1', $4::jsonb)`, wid, id, coin, raw); err != nil {
				t.Fatal(err)
			}
			// A share too big to fit, so the refund lands exactly on the cap the migration computes.
			if _, err := pool.Exec(ctx, `insert into trade_offers (world_id, id, from_id, expires_at, data)
				values ($1, $2, 'h1', 0, jsonb_build_object('originId', $3::text,
				  'give', jsonb_build_object('wood', 1e9::float8, 'clay', 0::float8, 'iron', 0::float8)))`,
				wid, "o_"+id, id); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := postgres.MigrateDown(ctx, pool, 16); err != nil {
		t.Fatalf("rollback: %v", err)
	}
	for _, withBonus := range []bool{false, true} {
		for _, lvl := range storages {
			id := fmt.Sprintf("v:%d:%t", lvl, withBonus)
			var got float64
			if err := pool.QueryRow(ctx, `select (data->'resources'->>'wood')::float8 from villages where id = $1`, id).Scan(&got); err != nil {
				t.Fatal(err)
			}
			var vb *game.VillageBonus
			if withBonus {
				vb = &game.VillageBonus{Kind: "storage"}
			}
			if want := game.StorageCapacity(lvl, vb); got != want {
				t.Fatalf("storage %d bonus %t: migration cap %v, game says %v", lvl, withBonus, got, want)
			}
		}
	}
}
