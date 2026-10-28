// Command export writes the instant archive, its leaderboards and every action log to files, for offline work
// (admin only; there is no public API for the logs).
//
//	go run ./cmd/export -db postgres://librehold:librehold@localhost:5433/librehold -out ./export
package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"librehold/internal/adapters/postgres"
)

func main() {
	db := flag.String("db", os.Getenv("LH_DATABASE_URL"), "database URL (default $LH_DATABASE_URL)")
	out := flag.String("out", "export", "directory to write into")
	flag.Parse()
	if *db == "" {
		fmt.Fprintln(os.Stderr, "export: -db or LH_DATABASE_URL is required")
		os.Exit(2)
	}
	ctx := context.Background()
	pool, err := postgres.Open(ctx, *db)
	if err != nil {
		fmt.Fprintln(os.Stderr, "export:", err)
		os.Exit(1)
	}
	defer pool.Close()
	if err := postgres.Export(ctx, pool, *out); err != nil {
		fmt.Fprintln(os.Stderr, "export:", err)
		os.Exit(1)
	}
	fmt.Println("export: written to", *out)
}
