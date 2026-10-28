package app

import (
	"context"
	"errors"
	"time"

	"librehold/internal/game"
)

// ---- accounts and sessions ----

type Account struct {
	ID           int64
	Username     string
	PasswordHash string // empty for guests: no password ever verifies against it
	IsAdmin      bool
	CreatedAt    time.Time
	Locale       string // "en" or "pl"
	IsGuest      bool
	PlayerKey    string // random and stable; stands for the account in action logs and archives
	TutorialDone bool
	LastSeenAt   time.Time
}

// ProfilePatch changes only the fields that are set.
type ProfilePatch struct {
	Locale       *string
	TutorialDone *bool
	Username     *string
	PasswordHash *string
	IsGuest      *bool
}

type Session struct {
	TokenHash  []byte
	AccountID  int64
	ExpiresAt  time.Time
	LastSeenAt time.Time
}

var (
	ErrNotFound      = errors.New("not found")
	ErrUsernameTaken = errors.New("username taken")
)

type AccountStore interface {
	// Register inserts a new account; sets a.ID. ErrUsernameTaken when the name is taken, case-insensitively.
	Register(ctx context.Context, a *Account, at time.Time) error
	CreateAdmin(ctx context.Context, a *Account) error
	AdminExists(ctx context.Context) (bool, error)
	ByUsername(ctx context.Context, username string) (*Account, error) // case-insensitive
	ByID(ctx context.Context, id int64) (*Account, error)
	List(ctx context.Context) ([]Account, error)
	SetPasswordHash(ctx context.Context, id int64, hash string) error
	CreateSession(ctx context.Context, s Session) error
	Session(ctx context.Context, tokenHash []byte) (*Session, error)
	TouchSession(ctx context.Context, tokenHash []byte, lastSeen, expires time.Time) error
	DeleteSession(ctx context.Context, tokenHash []byte) error
	DeleteSessionsOf(ctx context.Context, accountID int64) error
	// UpdateProfile applies the set fields; ErrUsernameTaken for a taken name, ErrNotFound for a missing account.
	UpdateProfile(ctx context.Context, id int64, p ProfilePatch) error
	TouchAccount(ctx context.Context, id int64, at time.Time) error
	// DeleteAccount removes the account and its sessions; its players stay, unlinked.
	DeleteAccount(ctx context.Context, id int64) error
	// GuestsInactiveSince lists guests last seen before t, oldest first.
	GuestsInactiveSince(ctx context.Context, t time.Time, limit int) ([]int64, error)
	ProblemReportStore
}

type PasswordHasher interface {
	Hash(ctx context.Context, pw string) (string, error)
	Verify(ctx context.Context, hash, pw string) (bool, error)
}

type TokenSource interface {
	New() (plain string, hash []byte)
	Hash(plain string) []byte
}

// ---- limits ----

// Counters counts events per key in fixed windows that every server process shares: the limits on failed logins,
// new accounts and reports.
type Counters interface {
	// Add counts an event of key and returns how many its window holds, this one included. The first one opens a
	// window that closes window later; the rest do not extend it.
	Add(ctx context.Context, key string, window time.Duration) (int, error)
	// Undo takes one event back from key's open window; with none open it does nothing.
	Undo(ctx context.Context, key string) error
	// Clear closes key's window.
	Clear(ctx context.Context, key string) error
}

// ---- worlds ----

type WorldRow struct {
	ID            int64
	Name          string
	Status        string // open | closed
	Seed          uint32
	Secret        []byte // 32 random bytes, the key of game.HiddenSeed; never shown to anyone
	Config        game.WorldConfig
	StartedAtMs   int64 // wall ms at world time 0
	PausedAtMs    *int64
	PausedTotalMs int64
	State         game.WorldState // set when an instant world ends, with its running totals
	GameNumber    int             // an instant world's number from the global sequence; 0 for long worlds
	Host          *HostedWorld    // a player world's host; nil for every other world
}

// HostedWorld is what a player world keeps about its host; nil for every other world.
type HostedWorld struct {
	HostID         int64   `json:"-"` // column host_account_id
	InviteCode     string  `json:"-"` // column invite_code (unique)
	ResultsCode    string  `json:"-"` // column results_code (unique)
	HostName       string  `json:"hostName"`
	StartedAtMs    *int64  `json:"startedAt,omitempty"` // wall ms of Start; nil while waiting
	HostPausedMs   int64   `json:"hostPausedMs"`        // closed host pauses together
	Banned         []int64 `json:"banned,omitempty"`
	RegisteredOnly bool    `json:"registeredOnly,omitempty"`
}

// Clone is a copy that shares nothing with h.
func (h *HostedWorld) Clone() *HostedWorld {
	if h == nil {
		return nil
	}
	c := *h
	if h.StartedAtMs != nil {
		c.StartedAtMs = new(int64)
		*c.StartedAtMs = *h.StartedAtMs
	}
	if h.Banned != nil {
		c.Banned = append([]int64{}, h.Banned...)
	}
	return &c
}

// HostSettings are the admin's limits on player worlds.
type HostSettings struct {
	MaxSeats int `json:"maxSeats"`
	MaxBots  int `json:"maxBots"`
}

var DefaultHostSettings = HostSettings{MaxSeats: 10, MaxBots: 5}

type HostedStore interface {
	HostSettings(ctx context.Context) (HostSettings, error) // DefaultHostSettings until saved
	SaveHostSettings(ctx context.Context, s HostSettings) error
	WorldByInvite(ctx context.Context, code string) (*WorldRow, error)      // ErrNotFound
	WorldByResultsCode(ctx context.Context, code string) (*WorldRow, error) // ErrNotFound
}

type WorldSummary struct {
	WorldRow
	Humans, Bots int
}

type Membership struct {
	WorldID  int64
	PlayerID game.PlayerID
}

// WorldTx is the unit of work for one world inside one transaction holding the world's lock.
type WorldTx interface {
	game.World
	Row() *WorldRow // mutable; flushed on commit
	SetNow(t int64)
	PopDue(until int64) (key string, at int64, ok bool)
	TouchedHumans() []game.PlayerID
	IntelOf(player game.PlayerID) map[string]*game.Intel // by village id
	// MovingCommands are the attacks, supports, returns and intercepts not yet arrived at now, by ID (the god view).
	MovingCommands(now int64) []*game.Command
}

// Update tells live connections what to refetch. PlayerID "" is the whole world.
type Update struct {
	WorldID    int64         `json:"worldId"`
	PlayerID   game.PlayerID `json:"playerId,omitempty"`
	Topics     []string      `json:"topics"`
	VillageIDs []string      `json:"villageIds,omitempty"`
}

type WorldStore interface {
	// Create inserts the row and runs init in the same unit of work; returns the new world's id.
	Create(ctx context.Context, row WorldRow, init func(WorldTx) error) (int64, error)
	// Run locks the world, runs fn, flushes, notifies and commits. It rolls back on an error or on a
	// panic(*game.StoreError), which it returns as the error.
	Run(ctx context.Context, worldID int64, fn func(WorldTx) error) error
	// Read runs fn on committed state without the lock and never writes.
	Read(ctx context.Context, worldID int64, fn func(WorldTx) error) error
	List(ctx context.Context) ([]WorldSummary, error)
	Get(ctx context.Context, worldID int64) (*WorldRow, error)
	MembershipsOf(ctx context.Context, accountID int64) ([]Membership, error)
	NextDue(ctx context.Context, worldID int64) (at int64, ok bool, err error)
	// Delete removes the world and everything in it; ErrNotFound if there is none.
	Delete(ctx context.Context, worldID int64) error
	// NextGameNumber hands out the next instant game number; a number is never given twice.
	NextGameNumber(ctx context.Context) (int64, error)
	ArchiveStore
	ActionLogStore
	PublicEventStore
	TimelineStore
	HostedStore
}

type PublicEventQuery struct {
	WorldID   int64 // 0 = every world
	MinTroops int   // battles with fewer troops are left out; conquests always stay
	Limit     int   // 0 = no limit
}

type PublicEventStore interface {
	// PublicEvents is newest first: (At, WorldID, insertion) descending, with WorldName, WorldKind and GameNumber filled.
	PublicEvents(ctx context.Context, q PublicEventQuery) ([]game.PublicEvent, error)
	// LastBattleAt is the newest event of any kind and size in any world; ok=false when there is none.
	LastBattleAt(ctx context.Context) (at time.Time, ok bool, err error)
}

type Updates interface {
	Subscribe(ctx context.Context) (<-chan Update, error)
}

// EventWakeups delivers the ids of worlds whose events changed.
type EventWakeups interface {
	Subscribe(ctx context.Context) (<-chan int64, error)
}

// ClockOrigin is game.World.ClockOrigin for a world at world time now: the local wall ms in
// game.WorldTimezone at world time 0, with the zone offset taken at the current wall instant.
func ClockOrigin(row *WorldRow, now int64) int64 {
	origin := row.StartedAtMs + row.PausedTotalMs
	_, offset := time.UnixMilli(origin + now).In(worldZone()).Zone()
	return origin + int64(offset)*1000
}
