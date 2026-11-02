package postgres_test

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"librehold/internal/adapters/postgres"
	"librehold/internal/adapters/postgres/pgtest"
	"librehold/internal/app"
	"librehold/internal/app/apptest"
	"librehold/internal/game"
)

// roundTrips counts the trips to the database: each query, and each batch as one.
type roundTrips struct{ n atomic.Int64 }

func (c *roundTrips) TraceQueryStart(ctx context.Context, _ *pgx.Conn, _ pgx.TraceQueryStartData) context.Context {
	c.n.Add(1)
	return ctx
}
func (c *roundTrips) TraceQueryEnd(context.Context, *pgx.Conn, pgx.TraceQueryEndData) {}
func (c *roundTrips) TraceBatchStart(ctx context.Context, _ *pgx.Conn, _ pgx.TraceBatchStartData) context.Context {
	c.n.Add(1)
	return ctx
}
func (c *roundTrips) TraceBatchQuery(context.Context, *pgx.Conn, pgx.TraceBatchQueryData) {}
func (c *roundTrips) TraceBatchEnd(context.Context, *pgx.Conn, pgx.TraceBatchEndData)     {}

func countedWorlds(t *testing.T) (*postgres.Worlds, *roundTrips) {
	t.Helper()
	plain := worldsDB(t)
	cfg, err := pgxpool.ParseConfig(pgtest.URL(plain.Config().ConnConfig.Database))
	if err != nil {
		t.Fatal(err)
	}
	counter := &roundTrips{}
	cfg.ConnConfig.Tracer = counter
	pool, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return postgres.NewWorlds(pool), counter
}

// The farm list and the auto-farm read the map around a village in a few queries whatever the radius: a query
// per tile made one radius-50 tick some 10,000 trips to the database, all under the world's lock.
func TestFarmListTripsDoNotGrowWithTheRadius(t *testing.T) {
	s, counter := countedWorlds(t)
	ctx := context.Background()
	id, err := s.Create(ctx, apptest.NewWorldRow(3), func(w app.WorldTx) error { return game.NewWorld(w) })
	if err != nil {
		t.Fatal(err)
	}
	var home string
	err = s.Run(ctx, id, func(w app.WorldTx) error {
		v, err := game.JoinHuman(w, "h1", "Ala", 1, game.QuadrantRandom)
		if err != nil {
			return err
		}
		home = v.ID
		known := 0
		for _, t := range game.VillagesInRadius(w, v.X, v.Y, 12) {
			if t.OwnerID == "" && known < 8 {
				game.EnsureVillage(w, t.X, t.Y)
				w.PutIntel("h1", t.ID, &game.Intel{LastFarmedAt: new(int64)})
				known++
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	trips := func(radius float64) (int64, int) {
		var n int64
		var listed int
		err := s.Run(ctx, id, func(w app.WorldTx) error {
			if err := game.SetFarmFilters(w, "h1", game.FarmFiltersPatch{Radius: &radius}); err != nil {
				return err
			}
			before := counter.n.Load()
			listed = len(game.FarmTargets(w, "h1", home))
			n = counter.n.Load() - before
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
		return n, listed
	}
	near, nearListed := trips(5)
	far, farListed := trips(50)
	if farListed <= nearListed {
		t.Fatalf("setup: radius 50 lists %d targets, radius 5 %d", farListed, nearListed)
	}
	if far > near || far > 6 {
		t.Fatalf("the farm list took %d trips at radius 5 and %d at radius 50, want the same few", near, far)
	}
}
