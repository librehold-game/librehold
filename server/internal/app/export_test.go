package app

import (
	"context"

	"librehold/internal/game"
)

func (s *Service) InWorld(ctx context.Context, acct *Account, worldID int64, fn func(w WorldTx, me game.PlayerID) error) error {
	return s.inWorld(ctx, acct, worldID, fn)
}

var ReplayEntry = replayEntry

// PublicSettingsOf exposes the world page's settings projection to the tests.
func PublicSettingsOf(cfg game.WorldConfig) PublicSettings { return publicSettingsOf(&cfg) }
