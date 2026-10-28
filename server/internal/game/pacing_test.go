package game_test

import (
	"math"
	"sync"
	"testing"

	"librehold/internal/game"
	"librehold/internal/game/gametest"
)

// The game's pace on the default world settings, measured by the reference player (gametest.RunReferencePlayer,
// the same one `go run ./cmd/sim -player` prints): from 1 village to 10 without a blocker or a wall of boredom.
//
// The test is sensitive on purpose: a balance change that lengthens the road to the tenth village or introduces a
// stall should break it. If the change is intended, adjust the thresholds here and record the new measurement in
// docs/balance.md. Thresholds = the TypeScript measurement of 2026-10-18 × 1.2, rounded up to a full hour or day.

const pacingDays = 24

var (
	pacingOnce sync.Once
	pacingRun  gametest.PlayerRun
	pacingErr  error
)

func pacing(t *testing.T) gametest.PlayerRun {
	t.Helper()
	if testing.Short() {
		t.Skip("long simulation")
	}
	pacingOnce.Do(func() {
		pacingRun, pacingErr = gametest.RunReferencePlayer(gametest.PlayerRunOptions{Seed: 1, Days: pacingDays})
	})
	if pacingErr != nil {
		t.Fatal(pacingErr)
	}
	return pacingRun
}

func milestoneDay(r gametest.PlayerRun, m string) float64 {
	ms, ok := r.Milestones[m]
	if !ok || ms < 0 {
		return math.Inf(1)
	}
	return float64(ms) / float64(game.MsPerDay)
}

func TestReferencePlayerReportsEveryMilestoneKey(t *testing.T) {
	r := pacing(t)
	if len(r.Milestones) != len(gametest.Milestones) {
		t.Fatalf("%d milestones, want %d", len(r.Milestones), len(gametest.Milestones))
	}
	for _, m := range gametest.Milestones {
		t.Logf("%-22s %6.2f d", m, milestoneDay(r, m))
	}
}

func TestPacing(t *testing.T) {
	r := pacing(t)
	t.Run("first raid and first successful farm within the first two hours", func(t *testing.T) {
		for _, m := range []string{"first attack", "first successful raid"} {
			if at := r.Milestones[m]; at < 0 || at > 2*game.MsPerHour {
				t.Errorf("%s at %d ms", m, at)
			}
		}
	})
	t.Run("1,000 points in five days", func(t *testing.T) {
		if d := milestoneDay(r, "1,000 pts"); d > 5 {
			t.Errorf("day %.2f", d)
		}
	})
	t.Run("first noble in the second week, not the third month", func(t *testing.T) {
		if d := milestoneDay(r, "first hetman"); d > 15 {
			t.Errorf("day %.2f", d)
		}
	})
	t.Run("second village right after the noble", func(t *testing.T) {
		if noble, second := milestoneDay(r, "first hetman"), milestoneDay(r, "2nd village"); second > noble+3 {
			t.Errorf("noble day %.2f, second village day %.2f", noble, second)
		}
	})
	t.Run("tenth village within 24 days", func(t *testing.T) {
		if d := milestoneDay(r, "10th village"); d > pacingDays {
			t.Errorf("day %.2f", d)
		}
	})
	t.Run("further villages come gradually, not all at once", func(t *testing.T) {
		if third, tenth := milestoneDay(r, "3rd village"), milestoneDay(r, "10th village"); !(tenth > third) {
			t.Errorf("third day %.2f, tenth day %.2f", third, tenth)
		}
	})
	t.Run("no three days in a row without progress", func(t *testing.T) {
		for i := 3; i < len(r.Days); i++ {
			if now, before := r.Days[i], r.Days[i-3]; now.Points <= before.Points {
				t.Errorf("days %d–%d: %d → %d points", before.Day, now.Day, before.Points, now.Points)
			}
		}
	})
	t.Run("the haul keeps growing throughout the run", func(t *testing.T) {
		if len(r.Days) == 0 {
			t.Fatal("no day snapshots")
		}
		half, last := r.Days[len(r.Days)/2], r.Days[len(r.Days)-1]
		if !(half.Looted > 0 && last.Looted > half.Looted) {
			t.Errorf("looted %v at day %d, %v at day %d", half.Looted, half.Day, last.Looted, last.Day)
		}
	})
}
