package postgres_test

import (
	"bytes"
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"librehold/internal/adapters/memory"
	"librehold/internal/adapters/postgres"
	"librehold/internal/adapters/postgres/pgtest"
	"librehold/internal/app"
	"librehold/internal/app/apptest"
	"librehold/internal/game"
)

// worldsDB is a fresh database with accounts 1–20, which players.account_id references.
func worldsDB(t *testing.T) *pgxpool.Pool {
	pool := pgtest.New(t)
	_, err := pool.Exec(context.Background(), `insert into accounts (id, username, username_lower, password_hash)
		select g, 'konto' || g, 'konto' || g, '' from generate_series(1, 20) g`)
	if err != nil {
		t.Fatal(err)
	}
	return pool
}

func backend(t *testing.T) apptest.WorldBackend {
	pool := worldsDB(t)
	l := postgres.NewListener(pool)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { l.Run(ctx); close(done) }()
	t.Cleanup(func() { cancel(); <-done })
	<-l.Ready()
	return apptest.WorldBackend{Store: postgres.NewWorlds(pool), Updates: l.Updates(), Wakeups: l.Wakeups()}
}

func TestWorldStore(t *testing.T) {
	apptest.RunWorldStoreContract(t, backend)
}

func TestDomainScenarioSameOnBothAdapters(t *testing.T) {
	mem := apptest.DomainScenario(t, memory.NewWorlds())
	pg := apptest.DomainScenario(t, postgres.NewWorlds(worldsDB(t)))
	if len(mem) != 8 {
		t.Fatalf("snapshot kinds %d", len(mem))
	}
	for kind, want := range map[string]string{"players": `"isHuman":true`, "villages": `"ownerId":"h2"`,
		"reports": `"type":"attack"`, "day1/commands": `"type":"support"`} {
		if !bytes.Contains(mem[kind], []byte(want)) {
			t.Errorf("%s: the scenario lost its %s: %.500s", kind, want, mem[kind])
		}
	}
	for kind := range mem {
		if !bytes.Equal(mem[kind], pg[kind]) {
			t.Errorf("%s differ:\nmemory:   %.2000s\npostgres: %.2000s", kind, mem[kind], pg[kind])
		}
	}
}

// writeCounter counts the statements that write rows, single and batched.
type writeCounter struct {
	mu     sync.Mutex
	writes []string
}

func (c *writeCounter) add(sql string) {
	s := strings.ToLower(strings.TrimSpace(sql))
	if strings.HasPrefix(s, "insert") || strings.HasPrefix(s, "update") || strings.HasPrefix(s, "delete") {
		c.mu.Lock()
		c.writes = append(c.writes, s)
		c.mu.Unlock()
	}
}

func (c *writeCounter) take() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	w := c.writes
	c.writes = nil
	return w
}

func (c *writeCounter) TraceQueryStart(ctx context.Context, _ *pgx.Conn, d pgx.TraceQueryStartData) context.Context {
	c.add(d.SQL)
	return ctx
}
func (c *writeCounter) TraceQueryEnd(context.Context, *pgx.Conn, pgx.TraceQueryEndData) {}
func (c *writeCounter) TraceBatchStart(ctx context.Context, _ *pgx.Conn, _ pgx.TraceBatchStartData) context.Context {
	return ctx
}
func (c *writeCounter) TraceBatchQuery(_ context.Context, _ *pgx.Conn, d pgx.TraceBatchQueryData) {
	c.add(d.SQL)
}
func (c *writeCounter) TraceBatchEnd(context.Context, *pgx.Conn, pgx.TraceBatchEndData) {}

func TestUnchangedEntitiesNotWritten(t *testing.T) {
	plain := worldsDB(t)
	cfg, err := pgxpool.ParseConfig(pgtest.URL(plain.Config().ConnConfig.Database))
	if err != nil {
		t.Fatal(err)
	}
	counter := &writeCounter{}
	cfg.ConnConfig.Tracer = counter
	pool, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	s := postgres.NewWorlds(pool)
	ctx := context.Background()
	id, err := s.Create(ctx, apptest.NewWorldRow(3), func(w app.WorldTx) error { return game.NewWorld(w) })
	if err != nil {
		t.Fatal(err)
	}
	counter.take()

	var vid string
	err = s.Run(ctx, id, func(w app.WorldTx) error {
		for _, p := range w.Players() {
			w.Brain(p.ID)
			for _, v := range p.VillageIDs {
				w.Village(v)
				vid = v
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, sql := range counter.take() {
		if !strings.HasPrefix(sql, "update worlds") {
			t.Errorf("an unchanged run wrote: %s", sql)
		}
	}

	err = s.Run(ctx, id, func(w app.WorldTx) error {
		for _, p := range w.Players() {
			for _, v := range p.VillageIDs {
				w.Village(v)
			}
		}
		w.Village(vid).Name = "Renamed"
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	villageWrites := 0
	for _, sql := range counter.take() {
		if strings.Contains(sql, "villages") {
			villageWrites++
		} else if !strings.HasPrefix(sql, "update worlds") && !strings.Contains(sql, "village_support") {
			t.Errorf("unexpected write: %s", sql)
		}
	}
	if villageWrites != 1 {
		t.Fatalf("%d village writes, want 1", villageWrites)
	}
}

// A host who deletes their account leaves their world to its players: it stays, only the host link goes.
func TestDeletedHostKeepsTheWorld(t *testing.T) {
	ctx := context.Background()
	pool := worldsDB(t)
	worlds := postgres.NewWorlds(pool)
	row := apptest.NewWorldRow(1)
	row.Host = &app.HostedWorld{HostID: 3, InviteCode: "invite-code", ResultsCode: "results-code", HostName: "konto3"}
	id, err := worlds.Create(ctx, row, func(w app.WorldTx) error { return game.NewWorld(w) })
	if err != nil {
		t.Fatal(err)
	}
	if err := postgres.NewAccounts(pool).DeleteAccount(ctx, 3); err != nil {
		t.Fatal(err)
	}
	got, err := worlds.Get(ctx, id)
	if err != nil {
		t.Fatalf("the world went with its host: %v", err)
	}
	if got.Host == nil || got.Host.HostID != 0 || got.Host.InviteCode != "invite-code" {
		t.Fatalf("host after the deletion %+v", got.Host)
	}
}

// Review finding: LockAccount holds a connection while fn needs more; with a small pool and many callers the
// holders must not use up the pool.
func TestLockAccountNeverStarvesTheirOwnQueries(t *testing.T) {
	base := pgtest.New(t)
	cfg := base.Config().Copy()
	cfg.MaxConns = 4
	pool, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	s := postgres.NewWorlds(pool)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	var wg sync.WaitGroup
	for i := range 16 {
		wg.Go(func() {
			err := s.LockAccount(ctx, int64(i+1), func() error {
				_, err := s.List(ctx)
				return err
			})
			if err != nil {
				t.Errorf("caller %d: %v", i, err)
			}
		})
	}
	wg.Wait()
}
