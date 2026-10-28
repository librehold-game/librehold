package security

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestArgonRunsAtMostItsSlotsAtOnce(t *testing.T) {
	var running, peak atomic.Int32
	h := newArgon2(2, func(_, _ []byte, _, _ uint32, _ uint8, n uint32) []byte {
		now := running.Add(1)
		for old := peak.Load(); now > old && !peak.CompareAndSwap(old, now); old = peak.Load() {
		}
		time.Sleep(20 * time.Millisecond)
		running.Add(-1)
		return make([]byte, n)
	})
	hash, err := h.Hash(context.Background(), "password")
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for i := range 8 {
		wg.Go(func() {
			if i%2 == 0 {
				_, _ = h.Hash(context.Background(), "password")
			} else {
				_, _ = h.Verify(context.Background(), hash, "password")
			}
		})
	}
	wg.Wait()
	if got := peak.Load(); got != 2 {
		t.Fatalf("%d hashes ran at once, want 2", got)
	}
}

func TestArgonWaiterGivesUpWithItsRequest(t *testing.T) {
	release := make(chan struct{})
	h := newArgon2(1, func(_, _ []byte, _, _ uint32, _ uint8, n uint32) []byte {
		<-release
		return make([]byte, n)
	})
	defer close(release)
	go func() { _, _ = h.Hash(context.Background(), "busy") }()
	for len(h.slots) == 0 {
		time.Sleep(time.Millisecond)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if _, err := h.Hash(ctx, "waiting"); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("hash waiting for a slot: %v", err)
	}
	if _, err := h.Verify(ctx, "$argon2id$v=19$m=8,t=1,p=1$c2FsdHNhbHQ$a2V5", "waiting"); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("verify waiting for a slot: %v", err)
	}
}
