// Package pgtest runs one Postgres 18 container per test package and gives each test its own database.
package pgtest

import (
	"context"
	"fmt"
	"os"
	"sync/atomic"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"

	"librehold/internal/adapters/postgres"
)

const template = "lh_template"

var (
	adminURL string // the container's maintenance database
	baseURL  func(db string) string
	counter  atomic.Int64
)

// Main starts the container, migrates a template database, runs the package's tests and stops the container.
// Call it from TestMain.
func Main(m *testing.M) {
	ctx := context.Background()
	c, err := tcpostgres.Run(ctx, "postgres:18.6",
		tcpostgres.WithDatabase("postgres"), tcpostgres.WithUsername("librehold"), tcpostgres.WithPassword("librehold"),
		tcpostgres.BasicWaitStrategies())
	if err != nil {
		fmt.Fprintln(os.Stderr, "pgtest: start postgres:", err)
		os.Exit(1)
	}
	host, err := c.Host(ctx)
	if err == nil {
		var port string
		p, perr := c.MappedPort(ctx, "5432/tcp")
		port, err = p.Port(), perr
		baseURL = func(db string) string {
			return fmt.Sprintf("postgres://librehold:librehold@%s:%s/%s?sslmode=disable", host, port, db)
		}
		adminURL = baseURL("postgres")
	}
	if err == nil {
		err = createTemplate(ctx)
	}
	code := 1
	if err != nil {
		fmt.Fprintln(os.Stderr, "pgtest:", err)
	} else {
		code = m.Run()
	}
	_ = c.Terminate(ctx)
	os.Exit(code)
}

func createTemplate(ctx context.Context) error {
	admin, err := pgxpool.New(ctx, adminURL)
	if err != nil {
		return err
	}
	defer admin.Close()
	if _, err := admin.Exec(ctx, "create database "+template); err != nil {
		return err
	}
	pool, err := postgres.Open(ctx, baseURL(template))
	if err != nil {
		return err
	}
	defer pool.Close()
	return postgres.Migrate(ctx, pool)
}

func newDatabase(t *testing.T, from string) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()
	name := fmt.Sprintf("t_%d", counter.Add(1))
	admin, err := pgxpool.New(ctx, adminURL)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	stmt := "create database " + name
	if from != "" {
		stmt += " template " + from
	}
	if _, err := admin.Exec(ctx, stmt); err != nil {
		t.Fatal(err)
	}
	pool, err := postgres.Open(ctx, baseURL(name))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		pool.Close()
		if a, err := pgxpool.New(context.Background(), adminURL); err == nil {
			_, _ = a.Exec(context.Background(), "drop database if exists "+name+" with (force)")
			a.Close()
		}
	})
	return pool
}

// New returns a pool to a fresh, migrated database of its own.
func New(t *testing.T) *pgxpool.Pool { return newDatabase(t, template) }

// Clone returns a pool to a new database copied from the named one, which nobody may be connected to.
func Clone(t *testing.T, from string) *pgxpool.Pool { return newDatabase(t, from) }

// Empty returns a pool to a fresh database without the schema.
func Empty(t *testing.T) *pgxpool.Pool { return newDatabase(t, "") }

// URL is the connection string of the container's maintenance database (for tests needing their own pools).
func URL(db string) string { return baseURL(db) }
