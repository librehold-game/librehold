// Command server is the game server: the API, live updates, the web builds and the event workers.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
	_ "time/tzdata"

	"librehold/internal/adapters/httpapi"
	"librehold/internal/adapters/memory"
	"librehold/internal/adapters/postgres"
	"librehold/internal/adapters/security"
	"librehold/internal/adapters/valkey"
	"librehold/internal/adapters/web"
	"librehold/internal/adapters/ws"
	"librehold/internal/app"
	"librehold/internal/config"
)

type wallClock struct{}

func (wallClock) NowMs() int64 { return time.Now().UnixMilli() }

// scaledClock runs scale times faster than the wall clock from epoch on (end-to-end tests).
type scaledClock struct {
	epoch int64
	scale float64
}

func (c scaledClock) NowMs() int64 {
	return c.epoch + int64(float64(time.Now().UnixMilli()-c.epoch)*c.scale)
}

func main() {
	if err := run(); err != nil {
		slog.Error("server", "err", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	pool, err := postgres.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()
	if err := postgres.Migrate(ctx, pool); err != nil {
		return err
	}
	var clock app.Clock = wallClock{}
	appCfg := app.Config{AdminUsername: cfg.AdminUsername, AdminPassword: cfg.AdminPassword, RegistrationsPerHour: cfg.RegistrationsPerHour,
		GuestsPerHour: cfg.GuestsPerHour, WorldRequestsPerMinute: cfg.WorldRequestsPerMinute, InstantMaxWorlds: cfg.InstantMaxWorlds, InstantKeep: cfg.InstantKeep,
		InstantSweepEvery: cfg.InstantSweepEvery, PlayerMaxWorlds: cfg.PlayerMaxWorlds, PlayerSweepEvery: cfg.PlayerSweepEvery}
	if appCfg.HeadText, err = web.LoadHeadText(cfg.WebDir); err != nil {
		slog.Warn("page titles of games, players and worlds fall back to their keys", "err", err)
	}
	if cfg.TimeScale != 1 {
		clock = scaledClock{epoch: cfg.TimeEpochMs, scale: cfg.TimeScale}
		// Worker sleeps are measured on the scaled clock; short polls keep events on time.
		appCfg.WorkerPoll = 50 * time.Millisecond
		slog.Warn("time scale for tests", "scale", cfg.TimeScale)
	}
	var counters app.Counters = memory.NewCounters(clock)
	if cfg.ValkeyURL != "" {
		vc, err := valkey.Open(ctx, cfg.ValkeyURL, "lh:limit:")
		if err != nil {
			return fmt.Errorf("valkey: %w", err)
		}
		defer vc.Close()
		counters = vc
	} else {
		slog.Warn("LH_VALKEY_URL is not set: the limits on logins, accounts and reports count in this process only")
	}
	svc := app.NewService(postgres.NewAccounts(pool), postgres.NewWorlds(pool), counters, clock, security.NewArgon2(),
		security.NewTokens(), appCfg)
	if err := svc.BootstrapAdmin(ctx); err != nil {
		return err
	}

	background, cancel := context.WithCancel(context.Background())
	defer cancel()
	var wg sync.WaitGroup
	listener := postgres.NewListener(pool)
	wg.Go(func() { listener.Run(background) })
	hub := ws.NewHub(svc, listener.Updates(), clock)
	wg.Go(func() {
		if err := hub.Run(background); err != nil && background.Err() == nil {
			slog.Error("live updates", "err", err)
		}
	})
	wg.Go(func() {
		if err := svc.RunWorkers(background, listener.Wakeups()); err != nil && background.Err() == nil {
			slog.Error("event workers", "err", err)
		}
	})
	wg.Go(func() {
		if err := svc.RunInstant(background); err != nil && background.Err() == nil {
			slog.Error("instant worlds", "err", err)
		}
	})
	wg.Go(func() {
		if err := svc.RunPlayerWorlds(background); err != nil && background.Err() == nil {
			slog.Error("player worlds", "err", err)
		}
	})

	mux := http.NewServeMux()
	mux.Handle("/api/v1/live", hub)
	mux.Handle("/api/", httpapi.New(svc, httpapi.Options{CookieSecure: cfg.CookieSecure, TrustedProxies: cfg.TrustedProxies}))
	mux.Handle("GET /robots.txt", web.Robots(cfg.PublicURL))
	mux.Handle("GET /sitemap.xml", web.NewSitemap(cfg.WebDir, svc, cfg.PublicURL))
	mux.Handle("/", web.New(cfg.WebDir, func(r *http.Request) web.AuthKind {
		c, err := r.Cookie("lh_session")
		if err != nil {
			return web.AuthNone
		}
		acct, err := svc.Authenticate(r.Context(), c.Value)
		switch {
		case err != nil:
			return web.AuthNone
		case acct.IsGuest:
			return web.AuthGuest
		}
		return web.AuthRegistered
	}, svc, cfg.PublicURL))
	srv := &http.Server{Addr: cfg.HTTPAddr, Handler: web.SecurityHeaders(mux, cfg.CookieSecure, cfg.PublicURL),
		ReadHeaderTimeout: 10 * time.Second, IdleTimeout: 2 * time.Minute}
	srv.RegisterOnShutdown(cancel)

	errc := make(chan error, 1)
	go func() {
		slog.Info("listening " + cfg.HTTPAddr)
		if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			errc <- err
		}
		close(errc)
	}()
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
	}
	slog.Info("shutting down")
	shutdown, done := context.WithTimeout(context.Background(), 20*time.Second)
	defer done()
	err = srv.Shutdown(shutdown)
	cancel()
	wg.Wait()
	return err
}
