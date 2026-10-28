package app

import (
	"context"

	"librehold/internal/game"
)

func (s *Service) InWorld(ctx context.Context, acct *Account, worldID int64, fn func(w WorldTx, me game.PlayerID) error) error {
	return s.inWorld(ctx, acct, worldID, fn)
}
