// Package postgres is the Postgres adapter: repositories, the world unit of work, locks and notifications.
package postgres

import (
	"context"
	"embed"
	"fmt"
	"io/fs"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

func Open(ctx context.Context, url string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return pool, nil
}

// Migrate applies the embedded migrations that have not run yet.
func Migrate(ctx context.Context, pool *pgxpool.Pool) error {
	return migrateTo(ctx, pool, 0)
}

// migrateDown undoes migrations back to version, or all of them for 0. Only the tests use it, to prove a rollback
// leaves the world's resources intact.
func migrateDown(ctx context.Context, pool *pgxpool.Pool, version int64) error {
	return migrateWith(ctx, pool, version, true)
}

// migrateTo applies migrations up to version, or all of them for 0.
func migrateTo(ctx context.Context, pool *pgxpool.Pool, version int64) error {
	return migrateWith(ctx, pool, version, false)
}

func migrateWith(ctx context.Context, pool *pgxpool.Pool, version int64, down bool) error {
	dir, err := fs.Sub(migrationFiles, "migrations")
	if err != nil {
		return err
	}
	db := stdlib.OpenDBFromPool(pool)
	defer db.Close()
	provider, err := goose.NewProvider(goose.DialectPostgres, db, dir)
	if err != nil {
		return fmt.Errorf("migrations: %w", err)
	}
	switch {
	case down && version == 0:
		_, err = provider.Down(ctx)
	case down:
		_, err = provider.DownTo(ctx, version)
	case version == 0:
		_, err = provider.Up(ctx)
	default:
		_, err = provider.UpTo(ctx, version)
	}
	if err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	return nil
}
