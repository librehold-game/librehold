package app

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"math"
	"slices"
	"strings"
	"unicode/utf8"

	"librehold/internal/game"
)

func requireAdmin(actor *Account) error {
	if actor == nil || !actor.IsAdmin {
		return ErrForbidden
	}
	return nil
}

func (s *Service) Accounts(ctx context.Context, actor *Account) ([]Account, error) {
	if err := requireAdmin(actor); err != nil {
		return nil, err
	}
	return s.accounts.List(ctx)
}

// ResetPassword sets a new password and logs the account out everywhere.
func (s *Service) ResetPassword(ctx context.Context, actor *Account, accountID int64, newPassword string) error {
	if err := requireAdmin(actor); err != nil {
		return err
	}
	if utf8.RuneCountInString(newPassword) < minPassword {
		return &ErrValidation{Field: "password", Code: "passwordShort"}
	}
	hash, err := s.hasher.Hash(ctx, newPassword)
	if err != nil {
		return err
	}
	if err := s.accounts.SetPasswordHash(ctx, accountID, hash); err != nil {
		return err
	}
	return s.accounts.DeleteSessionsOf(ctx, accountID)
}

func clamp(v float64, r game.Range) float64 { return math.Max(r.Min, math.Min(r.Max, v)) }

func clampModifiers(m game.SideModifiers) game.SideModifiers {
	r := game.WorldLimits.Modifier
	return game.SideModifiers{Resources: clamp(m.Resources, r), Recruitment: clamp(m.Recruitment, r), Building: clamp(m.Building, r)}
}

// ClampWorldConfig keeps every numeric setting within game.WorldLimits; a map size off the list becomes the
// nearest allowed one.
func ClampWorldConfig(cfg game.WorldConfig) game.WorldConfig {
	l := game.WorldLimits
	cfg.WorldSpeed = clamp(cfg.WorldSpeed, l.WorldSpeed)
	cfg.UnitSpeed = clamp(cfg.UnitSpeed, l.UnitSpeed)
	if !slices.Contains(l.MapSizes, cfg.MapSize) {
		best := l.MapSizes[0]
		for _, size := range l.MapSizes {
			if abs(size-cfg.MapSize) < abs(best-cfg.MapSize) {
				best = size
			}
		}
		cfg.MapSize = best
	}
	cfg.HumanModifiers = clampModifiers(cfg.HumanModifiers)
	cfg.BotModifiers = clampModifiers(cfg.BotModifiers)
	cfg.BotCount = int(clamp(float64(cfg.BotCount), l.BotCount))
	cfg.RampUpDays = clamp(cfg.RampUpDays, l.RampUpDays)
	cfg.BeginnerProtectionDays = clamp(cfg.BeginnerProtectionDays, l.BeginnerProtectionDays)
	cfg.SightRadius = clamp(cfg.SightRadius, l.SightRadius)
	return cfg
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// newSeed draws a world's map. It is no secret: the map gives it away. What players must not foresee rolls from
// newSecret.
func newSeed() uint32 {
	var b [4]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	return binary.LittleEndian.Uint32(b[:])
}

// newSecret is a world's key of game.HiddenSeed.
func newSecret() []byte {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return b
}

func (s *Service) CreateWorld(ctx context.Context, actor *Account, name string, cfg game.WorldConfig) (int64, error) {
	if err := requireAdmin(actor); err != nil {
		return 0, err
	}
	name = strings.TrimSpace(name)
	if name == "" || utf8.RuneCountInString(name) > 40 {
		return 0, &ErrValidation{Field: "name", Code: "nameFormat"}
	}
	now := s.clock.NowMs()
	row := WorldRow{Name: name, Status: "open", Seed: newSeed(), Secret: newSecret(), Config: ClampWorldConfig(cfg), StartedAtMs: now}
	id, err := s.worlds.Create(ctx, row, newWorld)
	if err != nil {
		return 0, err
	}
	s.notifyWorldCreated(id)
	return id, nil
}

func (s *Service) Worlds(ctx context.Context, actor *Account) ([]WorldSummary, error) {
	if err := requireAdmin(actor); err != nil {
		return nil, err
	}
	return s.worlds.List(ctx)
}

// WorldSettingsPatch is a partial WorldConfig by JSON field name.
type WorldSettingsPatch map[string]json.RawMessage

var changeableSettings = []string{"worldSpeed", "unitSpeed", "humanModifiers", "botModifiers", "battleLosses",
	"beginnerProtectionDays", "lastVillage", "difficulty", "botStance", "noFieldBattles"}

// UpdateWorldSettings changes the settings that may change in a running world. Every stored village is
// settled at the old pace first; timers already running keep their end times.
func (s *Service) UpdateWorldSettings(ctx context.Context, actor *Account, worldID int64, patch WorldSettingsPatch) error {
	if err := requireAdmin(actor); err != nil {
		return err
	}
	keys := make([]string, 0, len(patch))
	for k := range patch {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	for _, k := range keys {
		if !slices.Contains(changeableSettings, k) {
			return &ErrValidation{Field: k, Code: "fixedAtCreation"}
		}
	}
	return s.worlds.Run(ctx, worldID, func(w WorldTx) error {
		return applySettings(w, WorldNow(w.Row(), s.clock.NowMs()), patch)
	})
}

// applySettings changes the world's settings at world time now, every village settled under the old ones first.
func applySettings(w WorldTx, now int64, patch WorldSettingsPatch) error {
	cfg, err := applyPatch(w.Row().Config, patch)
	if err != nil {
		return err
	}
	w.SetNow(now)
	if _, err := processDue(w, now, 0); err != nil {
		return err
	}
	w.SetNow(now)
	size := w.Config().MapSize
	for _, ref := range w.VillagesIn(0, 0, size-1, size-1) {
		game.Touch(w, w.Village(ref.ID))
	}
	w.Row().Config = cfg
	w.LogAction(actorAdmin, false, "settings", patch)
	return nil
}

func applyPatch(cfg game.WorldConfig, patch WorldSettingsPatch) (game.WorldConfig, error) {
	var fields map[string]json.RawMessage
	b, err := json.Marshal(cfg)
	if err != nil {
		return cfg, err
	}
	if err := json.Unmarshal(b, &fields); err != nil {
		return cfg, err
	}
	for k, v := range patch {
		fields[k] = v
	}
	b, err = json.Marshal(fields)
	if err != nil {
		return cfg, err
	}
	var out game.WorldConfig
	if err := json.Unmarshal(b, &out); err != nil {
		return cfg, &ErrValidation{Field: "settings", Code: "invalid"}
	}
	return ClampWorldConfig(out), nil
}

func (s *Service) SetWorldStatus(ctx context.Context, actor *Account, worldID int64, status string) error {
	if err := requireAdmin(actor); err != nil {
		return err
	}
	if status != "open" && status != "closed" {
		return &ErrValidation{Field: "status", Code: "statusInvalid"}
	}
	return s.worlds.Run(ctx, worldID, func(w WorldTx) error {
		w.Row().Status = status
		return nil
	})
}

func (s *Service) PauseWorld(ctx context.Context, actor *Account, worldID int64) error {
	if err := requireAdmin(actor); err != nil {
		return err
	}
	return s.worlds.Run(ctx, worldID, func(w WorldTx) error {
		if w.Row().PausedAtMs == nil {
			at := s.clock.NowMs()
			w.SetNow(WorldNow(w.Row(), at))
			w.Row().PausedAtMs = &at
			w.LogAction(actorAdmin, false, "pause", struct{}{})
		}
		return nil
	})
}

// ResumeWorld adds the paused span to the world's paused total, so world time continues where it stopped.
func (s *Service) ResumeWorld(ctx context.Context, actor *Account, worldID int64) error {
	if err := requireAdmin(actor); err != nil {
		return err
	}
	err := s.worlds.Run(ctx, worldID, func(w WorldTx) error {
		row := w.Row()
		if row.PausedAtMs != nil {
			row.PausedTotalMs += s.clock.NowMs() - *row.PausedAtMs
			row.PausedAtMs = nil
			logResume(w, s.clock.NowMs(), actorAdmin)
		}
		return nil
	})
	if err == nil {
		s.wakeWorker(worldID)
	}
	return err
}
