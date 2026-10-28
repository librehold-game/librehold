package gametest

import (
	"fmt"

	"librehold/internal/game"
)

// Advance plays the world forward to `until`: every due event in (at, seq) order, each at its own time,
// followed by quest settling for the humans it touched.
func Advance(w *MemWorld, until int64) error {
	for guard := 0; ; guard++ {
		if guard > 1_000_000 {
			return fmt.Errorf("advance: too many events in one batch")
		}
		key, at, ok := w.PopDue(until)
		if !ok {
			break
		}
		w.SetNow(at)
		if err := game.Dispatch(w, key); err != nil {
			return fmt.Errorf("%s at %d: %w", key, at, err)
		}
		for _, h := range w.TouchedHumans() {
			game.SettleQuests(w, h)
		}
	}
	if until > w.Now() {
		w.SetNow(until)
	}
	return nil
}
