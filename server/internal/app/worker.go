package app

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"
)

const (
	workerBatch = 500
	retryAfter  = time.Second
)

// workers is the running set of per-world event workers.
type workers struct {
	ctx    context.Context
	wg     sync.WaitGroup
	mu     sync.Mutex
	wakeOf map[int64]chan struct{}
}

// forget drops a deleted world's worker.
func (ws *workers) forget(id int64) {
	ws.mu.Lock()
	delete(ws.wakeOf, id)
	ws.mu.Unlock()
}

func (ws *workers) wake(id int64) {
	ws.mu.Lock()
	ch := ws.wakeOf[id]
	ws.mu.Unlock()
	if ch != nil {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
}

// RunWorkers runs one event worker per world, including worlds created later, until ctx ends.
func (s *Service) RunWorkers(ctx context.Context, wake EventWakeups) error {
	wakeups, err := wake.Subscribe(ctx)
	if err != nil {
		return err
	}
	ws := &workers{ctx: ctx, wakeOf: map[int64]chan struct{}{}}
	s.mu.Lock()
	s.workers = ws
	s.mu.Unlock()
	worlds, err := s.worlds.List(ctx)
	if err != nil {
		return err
	}
	for _, w := range worlds {
		s.startWorker(ws, w.ID)
	}
	for {
		select {
		case <-ctx.Done():
			s.mu.Lock()
			s.workers = nil
			s.mu.Unlock()
			ws.wg.Wait()
			return nil
		case id, ok := <-wakeups:
			if ok {
				ws.wake(id)
			}
		}
	}
}

// notifyWorldCreated starts the worker of a new world when workers run.
func (s *Service) notifyWorldCreated(id int64) {
	s.mu.Lock()
	ws := s.workers
	s.mu.Unlock()
	if ws != nil {
		s.startWorker(ws, id)
	}
}

// wakeWorker makes a running worker of this process recheck its world now.
func (s *Service) wakeWorker(id int64) {
	s.mu.Lock()
	ws := s.workers
	s.mu.Unlock()
	if ws != nil {
		ws.wake(id)
	}
}

func (s *Service) startWorker(ws *workers, id int64) {
	ws.mu.Lock()
	if _, ok := ws.wakeOf[id]; ok {
		ws.mu.Unlock()
		return
	}
	ch := make(chan struct{}, 1)
	ws.wakeOf[id] = ch
	ws.mu.Unlock()
	ws.wg.Go(func() { s.workWorld(ws.ctx, ws, id, ch) })
}

// workWorld sleeps until the world's next event is due in wall time (at most WorkerPoll, which also covers
// lost wakeups and clocks that jump), then processes due events in batches.
func (s *Service) workWorld(ctx context.Context, ws *workers, id int64, wake <-chan struct{}) {
	for ctx.Err() == nil {
		wait, err := s.untilDue(ctx, id)
		if err == nil && wait <= 0 {
			err = s.drain(ctx, id)
			if err == nil {
				continue
			}
		}
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			if errors.Is(err, ErrNotFound) {
				ws.forget(id)
				return
			}
			slog.Error("event worker", "world", id, "err", err)
			wait = retryAfter
		}
		timer := time.NewTimer(min(wait, s.cfg.WorkerPoll))
		select {
		case <-ctx.Done():
		case <-wake:
		case <-timer.C:
		}
		timer.Stop()
	}
}

// untilDue is how long until the world's next event is due; WorkerPoll when there is none or it is paused.
func (s *Service) untilDue(ctx context.Context, id int64) (time.Duration, error) {
	row, err := s.worlds.Get(ctx, id)
	if err != nil {
		return 0, err
	}
	at, ok, err := s.worlds.NextDue(ctx, id)
	if err != nil {
		return 0, err
	}
	if !ok || row.PausedAtMs != nil || row.State.EndedAt != nil {
		return s.cfg.WorkerPoll, nil
	}
	return time.Duration(row.StartedAtMs+row.PausedTotalMs+at-s.clock.NowMs()) * time.Millisecond, nil
}

func (s *Service) drain(ctx context.Context, id int64) error {
	for {
		_, more, err := s.ProcessDue(ctx, id, workerBatch)
		if err != nil || !more {
			return err
		}
	}
}
