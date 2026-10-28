package gen

import "librehold/internal/game"

// BalanceJSON is src/data/balance.json: the world-invariant tables the browser reads.
func BalanceJSON() ([]byte, error) { return canonical(game.BalanceExport()) }
