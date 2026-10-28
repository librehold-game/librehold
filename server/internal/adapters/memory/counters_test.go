package memory_test

import (
	"sync/atomic"
	"testing"
	"time"

	"librehold/internal/adapters/memory"
	"librehold/internal/app"
	"librehold/internal/app/apptest"
)

type clock struct{ ms atomic.Int64 }

func (c *clock) NowMs() int64 { return c.ms.Load() }

func TestCounters(t *testing.T) {
	apptest.RunCountersContract(t, func(t *testing.T) (app.Counters, time.Duration, func(time.Duration)) {
		c := &clock{}
		c.ms.Store(1_000_000)
		return memory.NewCounters(c), time.Minute, func(d time.Duration) { c.ms.Add(d.Milliseconds()) }
	})
}
