package app

import (
	"context"
	"crypto/rand"
	"encoding/base32"
	"errors"
	"math"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"

	"librehold/internal/game"
)

// Player worlds: a registered player hosts a small private game for friends, within the admin's limits.

const (
	playerMaxWorlds  = 16 // half the largest count the load test held up with (32 on an M1 Pro, 2026-10-21)
	hostPauseMaxMs   = 10 * 60_000
	hostPauseTotalMs = 30 * 60_000
	hostSeatsMin     = 2
	hostSeatsMax     = 30
	hostBotsMax      = 30
)

var ErrServerFull = errors.New("server full")

// CanHost says who may host a player world: any registered account.
func CanHost(acct *Account) bool { return !acct.IsGuest }

// HostForm is what the host chooses; every value is clamped by PlayerWorldConfig, nil or zero meaning the default.
type HostForm struct {
	Name           string          `json:"name"`
	DurationMin    int             `json:"durationMin"`
	Seats          int             `json:"seats"`
	Bots           *int            `json:"bots"`
	WorldSpeed     float64         `json:"worldSpeed"`
	UnitSpeed      float64         `json:"unitSpeed"`
	StartVillage   string          `json:"startVillage"` // grown | fresh
	Barbarians     string          `json:"barbarians"`   // few | normal | many
	Difficulty     game.Difficulty `json:"difficulty"`
	BotStance      game.BotStance  `json:"botStance"`
	FieldBattles   *bool           `json:"fieldBattles"`
	SightRadius    *int            `json:"sightRadius"`
	ProtectionMin  *int            `json:"protectionMin"`
	DominationPct  *int            `json:"dominationPct"` // 0 = off
	LateJoinMin    *int            `json:"lateJoinMin"`   // 0 = until start only
	RegisteredOnly bool            `json:"registeredOnly"`
}

type HostLimits struct {
	CanHost  bool `json:"canHost"`
	MaxSeats int  `json:"maxSeats"`
	MaxBots  int  `json:"maxBots"`
	Full     bool `json:"full"`
}

type HostedPlayer struct {
	ID      game.PlayerID `json:"id"`
	Name    string        `json:"name"`
	IsGuest bool          `json:"isGuest"`
}

// HostedCard is the host's view of their world.
type HostedCard struct {
	WorldID        int64            `json:"worldId"`
	Name           string           `json:"name"`
	Status         string           `json:"status"` // waiting | running | paused | ended
	Humans         int              `json:"humans"`
	Seats          int              `json:"seats"`
	Bots           int              `json:"bots"`
	InviteCode     string           `json:"inviteCode"`
	ResultsCode    string           `json:"resultsCode,omitempty"` // once ended
	RegisteredOnly bool             `json:"registeredOnly"`
	PauseEndsAtMs  int64            `json:"pauseEndsAt,omitempty"` // wall ms, while paused
	PauseLeftMs    int64            `json:"pauseLeftMs"`           // of the 30-minute budget
	Players        []HostedPlayer   `json:"players"`               // active humans
	Settings       game.WorldConfig `json:"settings"`
}

var barbarianDensity = map[string]game.BarbarianDensity{"few": "sparse", "normal": "normal", "many": "dense"}

var (
	difficulties = []game.Difficulty{game.Easy, game.Normal, game.Hard, game.Brutal}
	stances      = []game.BotStance{game.StancePeacefulRetaliate, game.StancePeacefulReputation, game.StanceNormal, game.StanceAllAgainstHumans}
)

func clampInt(v, lo, hi int) int { return min(hi, max(lo, v)) }

// orDefault is *v, or def when the host left it out.
func orDefault(v *int, def int) int {
	if v == nil {
		return def
	}
	return *v
}

// PlayerWorldConfig is the instant preset with the host's choices, each held to its range and the admin's limits.
func PlayerWorldConfig(f HostForm, s HostSettings) game.WorldConfig {
	cfg := game.InstantWorldConfig
	cfg.Kind = game.KindPlayer
	duration := 60
	if f.DurationMin > 0 {
		duration = int(math.Round(float64(f.DurationMin)/15)) * 15
	}
	cfg.DurationMin = clampInt(duration, 30, 120)
	seats := s.MaxSeats
	if f.Seats > 0 {
		seats = f.Seats
	}
	cfg.HumanSeats = clampInt(seats, hostSeatsMin, max(hostSeatsMin, s.MaxSeats))
	cfg.BotCount = clampInt(orDefault(f.Bots, min(3, s.MaxBots)), 0, s.MaxBots)
	speed := func(v float64) float64 {
		if v <= 0 {
			return 100
		}
		return math.Min(200, math.Max(50, v))
	}
	cfg.WorldSpeed, cfg.UnitSpeed = speed(f.WorldSpeed), speed(f.UnitSpeed)
	if f.StartVillage == "fresh" {
		cfg.StartPreset = ""
	}
	cfg.BarbarianDensity = "normal"
	if d, ok := barbarianDensity[f.Barbarians]; ok {
		cfg.BarbarianDensity = d
	}
	if slices.Contains(difficulties, f.Difficulty) {
		cfg.Difficulty = f.Difficulty
	}
	if slices.Contains(stances, f.BotStance) {
		cfg.BotStance = f.BotStance
	}
	cfg.NoFieldBattles = f.FieldBattles != nil && !*f.FieldBattles
	sight := clampInt(orDefault(f.SightRadius, 5), int(game.WorldLimits.SightRadius.Min), int(game.WorldLimits.SightRadius.Max))
	cfg.SightRadius, cfg.NoSight = float64(sight), sight == 0
	cfg.BeginnerProtectionDays = float64(clampInt(orDefault(f.ProtectionMin, 10), 0, 15)) / 1440
	cfg.DominationShare = 0
	if pct := orDefault(f.DominationPct, 60); pct > 0 {
		cfg.DominationShare = float64(clampInt(pct, 50, 90)) / 100
	}
	cfg.JoinWindowMin = 0
	if late := orDefault(f.LateJoinMin, 10); late > 0 {
		cfg.JoinWindowMin = min(clampInt(late/5*5, 5, 60), cfg.DurationMin)
	}
	cfg.MapSize = game.PlayerMapSize(cfg.HumanSeats + cfg.BotCount)
	return cfg
}

// hostNamePattern is the username characters plus spaces and a little punctuation.
var hostNamePattern = regexp.MustCompile(`^[A-Za-z0-9_\- '.,!?#&()]+$`)

func checkHostName(name string) error {
	if n := utf8.RuneCountInString(name); n < 1 || n > 40 || !hostNamePattern.MatchString(name) {
		return &ErrValidation{Field: "name", Code: "nameFormat"}
	}
	return nil
}

// newCode is 12 random bytes as 20 lowercase base32 letters and digits, never all digits, so a /games/{ref} is a
// world id exactly when it parses as a number.
func newCode() string {
	enc := base32.StdEncoding.WithPadding(base32.NoPadding)
	for {
		b := make([]byte, 12)
		_, _ = rand.Read(b)
		code := strings.ToLower(enc.EncodeToString(b))
		if strings.Trim(code, "0123456789") != "" {
			return code
		}
	}
}

func (s *Service) HostSettings(ctx context.Context) (HostSettings, error) {
	return s.worlds.HostSettings(ctx)
}

// SetHostSettings changes the admin's limits for player worlds created from now on.
func (s *Service) SetHostSettings(ctx context.Context, actor *Account, in HostSettings) (HostSettings, error) {
	if err := requireAdmin(actor); err != nil {
		return HostSettings{}, err
	}
	if in.MaxSeats < hostSeatsMin || in.MaxSeats > hostSeatsMax {
		return HostSettings{}, &ErrValidation{Field: "maxSeats", Code: "outOfRange"}
	}
	if in.MaxBots < 0 || in.MaxBots > hostBotsMax {
		return HostSettings{}, &ErrValidation{Field: "maxBots", Code: "outOfRange"}
	}
	if err := s.worlds.SaveHostSettings(ctx, in); err != nil {
		return HostSettings{}, err
	}
	return in, nil
}

func hostedBy(w WorldSummary, acct *Account) bool { return w.Host != nil && w.Host.HostID == acct.ID }

func livePlayerWorld(w WorldSummary) bool {
	return w.Config.Kind == game.KindPlayer && w.State.EndedAt == nil
}

// CreatePlayerWorld opens the host's world, waiting (paused at world time 0) until they start it; one at a time.
func (s *Service) CreatePlayerWorld(ctx context.Context, acct *Account, f HostForm) (*HostedCard, error) {
	if !CanHost(acct) {
		return nil, ErrForbidden
	}
	s.hostMu.Lock()
	defer s.hostMu.Unlock()
	worlds, err := s.worlds.List(ctx)
	if err != nil {
		return nil, err
	}
	live := 0
	for _, w := range worlds {
		if !livePlayerWorld(w) {
			continue
		}
		if hostedBy(w, acct) {
			return nil, &ErrValidation{Field: "world", Code: "alreadyHosting"}
		}
		live++
	}
	name := strings.TrimSpace(f.Name)
	if err := checkHostName(name); err != nil {
		return nil, err
	}
	if live >= s.cfg.PlayerMaxWorlds {
		return nil, ErrServerFull
	}
	limits, err := s.worlds.HostSettings(ctx)
	if err != nil {
		return nil, err
	}
	now := s.clock.NowMs()
	row := WorldRow{Name: name, Status: "open", Seed: newSeed(), Secret: newSecret(), Config: PlayerWorldConfig(f, limits), StartedAtMs: now, PausedAtMs: &now,
		Host: &HostedWorld{HostID: acct.ID, InviteCode: newCode(), ResultsCode: newCode(), HostName: acct.Username, RegisteredOnly: f.RegisteredOnly}}
	id, err := s.worlds.Create(ctx, row, newWorld)
	if err != nil {
		return nil, err
	}
	s.notifyWorldCreated(id)
	return s.hostedCard(ctx, id)
}

// Hosted is the account's hosted world until it is purged: the one still going, else the latest ended; nil for none.
func (s *Service) Hosted(ctx context.Context, acct *Account) (*HostedCard, error) {
	worlds, err := s.worlds.List(ctx)
	if err != nil {
		return nil, err
	}
	var best *WorldSummary
	for i, w := range worlds {
		if !hostedBy(w, acct) {
			continue
		}
		if best == nil || livePlayerWorld(w) || !livePlayerWorld(*best) && w.ID > best.ID {
			best = &worlds[i]
		}
	}
	if best == nil {
		return nil, nil
	}
	return s.hostedCard(ctx, best.ID)
}

// hostLimits is what the lobby's host form may offer the account.
func (s *Service) hostLimits(ctx context.Context, acct *Account, worlds []WorldSummary) (HostLimits, error) {
	limits, err := s.worlds.HostSettings(ctx)
	if err != nil {
		return HostLimits{}, err
	}
	live := 0
	for _, w := range worlds {
		if livePlayerWorld(w) {
			live++
		}
	}
	return HostLimits{CanHost: CanHost(acct), MaxSeats: limits.MaxSeats, MaxBots: limits.MaxBots, Full: live >= s.cfg.PlayerMaxWorlds}, nil
}

func (s *Service) hostedCard(ctx context.Context, id int64) (*HostedCard, error) {
	var card *HostedCard
	err := s.worlds.Read(ctx, id, func(w WorldTx) error {
		row := w.Row()
		h := row.Host
		if h == nil {
			return ErrNotFound
		}
		card = &HostedCard{WorldID: row.ID, Name: row.Name, Seats: row.Config.HumanSeats, Bots: row.Config.BotCount,
			InviteCode: h.InviteCode, RegisteredOnly: h.RegisteredOnly, PauseLeftMs: max(0, hostPauseTotalMs-h.HostPausedMs),
			Players: []HostedPlayer{}, Settings: row.Config}
		switch {
		case row.State.EndedAt != nil:
			card.Status, card.ResultsCode = "ended", h.ResultsCode
		case h.StartedAtMs == nil:
			card.Status = "waiting"
		case row.PausedAtMs != nil:
			card.Status = "paused"
			card.PauseEndsAtMs = hostPauseEndsAt(row)
		default:
			card.Status = "running"
		}
		for _, p := range w.Players() {
			if p.IsHuman && p.Status == game.StatusActive {
				card.Players = append(card.Players, HostedPlayer{ID: p.ID, Name: p.Name, IsGuest: p.IsGuest})
			}
		}
		card.Humans = activeHumans(w)
		return nil
	})
	return card, err
}
