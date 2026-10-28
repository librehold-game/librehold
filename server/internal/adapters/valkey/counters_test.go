package valkey_test

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	tcvalkey "github.com/testcontainers/testcontainers-go/modules/valkey"

	"librehold/internal/adapters/valkey"
	"librehold/internal/app"
	"librehold/internal/app/apptest"
)

var (
	url  string
	runs atomic.Int64
)

func TestMain(m *testing.M) {
	ctx := context.Background()
	c, err := tcvalkey.Run(ctx, "valkey/valkey:9.1.2")
	if err == nil {
		url, err = c.ConnectionString(ctx)
	}
	code := 1
	if err != nil {
		fmt.Fprintln(os.Stderr, "valkey:", err)
	} else {
		code = m.Run()
	}
	_ = c.Terminate(ctx)
	os.Exit(code)
}

func TestCounters(t *testing.T) {
	apptest.RunCountersContract(t, func(t *testing.T) (app.Counters, time.Duration, func(time.Duration)) {
		c, err := valkey.Open(context.Background(), url, "test"+strconv.FormatInt(runs.Add(1), 10)+":")
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(c.Close)
		return c, 400 * time.Millisecond, time.Sleep
	})
}

func TestOpenFailsWithoutServer(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := valkey.Open(ctx, "redis://127.0.0.1:1", "x:"); err == nil {
		t.Fatal("opened a store with no server behind it")
	}
}
