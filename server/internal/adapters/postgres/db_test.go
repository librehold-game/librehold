package postgres_test

import (
	"context"
	"testing"

	"librehold/internal/adapters/postgres"
	"librehold/internal/adapters/postgres/pgtest"
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
