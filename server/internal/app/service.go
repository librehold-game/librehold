package app

import (
	"errors"
	"sync"
	"time"
)

// Clock is the only way the app reads wall time.
type Clock interface{ NowMs() int64 }

type Config struct {
	SessionTTL    time.Duration // 30 days unless set
	AdminUsername string
	AdminPassword string
	WorkerPoll    time.Duration // longest sleep of an event worker; 30 s unless set
	// RegistrationsPerHour is how many accounts one client address may open within an hour; 3 unless set.
	RegistrationsPerHour int
	// GuestsPerHour is how many guest accounts one client address may open within an hour; 10 unless set.
	GuestsPerHour int
	// WorldRequestsPerMinute is how many state reads and orders one account may send within a minute, over all its
	// worlds; 300 unless set. Each of them catches its world up and holds it while it works.
	WorldRequestsPerMinute int
	// InstantMaxWorlds is how many instant worlds may run at once; 4 unless set.
	InstantMaxWorlds int
	// InstantKeep is how long an ended instant world stays readable before it is deleted; 24 h unless set.
	InstantKeep time.Duration
	// InstantSweepEvery is how often the instant worlds are tended (ready world, purges); 1 min unless set.
	InstantSweepEvery time.Duration
	// PlayerMaxWorlds is how many player-hosted worlds may wait or run at once; playerMaxWorlds unless set.
	PlayerMaxWorlds int
	// PlayerSweepEvery is how often host pauses are closed at their cap and abandoned worlds deleted; 10 s unless set.
	PlayerSweepEvery time.Duration
	// HeadText is the client's head copy (head-text.json), for titles of pages the server fills.
	HeadText HeadText
}

type Service struct {
	accounts AccountStore
	worlds   WorldStore
	clock    Clock
	hasher   PasswordHasher
	tokens   TokenSource
	cfg      Config

	mu           sync.Mutex
	workers      *workers
	instantMu    sync.Mutex
	presenceMu   sync.Mutex
	bootMs       int64           // wall ms this process started; nobody can have been seen before it
	presentAt    map[int64]int64 // wall ms of the last presence write per world, in this process
	hostMu       sync.Mutex
	stats        *statsCache
	liveGames    map[int64]liveGame
	live         ttlCache[struct{}, LiveStrip]
	publicWorlds ttlCache[struct{}, PublicWorldList]
	publicWorld  ttlCache[int64, *PublicWorld]
	publicMap    ttlCache[int64, *PublicMap]

	counters      Counters
	registrations limit
	guests        limit
	pace          limit
	dummyOnce     sync.Once
	dummy         string
}

func NewService(accounts AccountStore, worlds WorldStore, counters Counters, clock Clock, hasher PasswordHasher, tokens TokenSource,
	cfg Config) *Service {
	if cfg.SessionTTL == 0 {
		cfg.SessionTTL = 30 * 24 * time.Hour
	}
	if cfg.WorkerPoll == 0 {
		cfg.WorkerPoll = 30 * time.Second
	}
	if cfg.RegistrationsPerHour == 0 {
		cfg.RegistrationsPerHour = registrationsLimit
	}
	if cfg.GuestsPerHour == 0 {
		cfg.GuestsPerHour = guestsLimit
	}
	if cfg.WorldRequestsPerMinute == 0 {
		cfg.WorldRequestsPerMinute = worldRequestsLimit
	}
	if cfg.InstantMaxWorlds == 0 {
		cfg.InstantMaxWorlds = instantMaxWorlds
	}
	if cfg.InstantKeep == 0 {
		cfg.InstantKeep = instantKeep
	}
	if cfg.InstantSweepEvery == 0 {
		cfg.InstantSweepEvery = instantSweepEvery
	}
	if cfg.PlayerMaxWorlds == 0 {
		cfg.PlayerMaxWorlds = playerMaxWorlds
	}
	if cfg.PlayerSweepEvery == 0 {
		cfg.PlayerSweepEvery = playerSweepEvery
	}
	return &Service{bootMs: clock.NowMs(), accounts: accounts, worlds: worlds, counters: counters, clock: clock, hasher: hasher, tokens: tokens, cfg: cfg,
		registrations: limit{"registration", cfg.RegistrationsPerHour, registrationWindow, true},
		guests:        limit{"guest", cfg.GuestsPerHour, guestWindow, true},
		pace:          limit{"pace", cfg.WorldRequestsPerMinute, time.Minute, false}}
}

var (
	ErrNotMember   = errors.New("not a member of this world")
	ErrWorldPaused = errors.New("world paused")
	ErrWorldEnded  = errors.New("world ended")
	ErrForbidden   = errors.New("forbidden")
)

// ErrValidation is a refused input: Field names it, Code says why.
type ErrValidation struct{ Field, Code string }

func (e *ErrValidation) Error() string { return "invalid " + e.Field + ": " + e.Code }
