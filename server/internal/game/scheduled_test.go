package game_test

import (
	"errors"
	"slices"
	"testing"

	"librehold/internal/game"
	"librehold/internal/game/gametest"
)

func mustSchedule(t *testing.T, w game.World, actor game.PlayerID, in game.ScheduleSendInput) *game.ScheduledSend {
	t.Helper()
	s, err := game.ScheduleSend(w, actor, in)
	if err != nil {
		t.Fatalf("ScheduleSend: %v", err)
	}
	return s
}

func lastNewsFor(w *gametest.MemWorld, id game.PlayerID) *game.NewsItem {
	for _, n := range w.News(0) {
		if n.For == id {
			return n
		}
	}
	return nil
}

func TestScheduledSends(t *testing.T) {
	t.Run("“arrive at” lands to the millisecond, even when the player is offline", func(t *testing.T) {
		w, v := gametest.NewTestWorld(41)
		v.Units[game.UAxe] = 10
		x, y := gametest.NearestBarbarianXY(w, v)
		at := w.Now() + 3*game.MsPerHour + 300
		mustSchedule(t, w, "h1", game.ScheduleSendInput{OriginID: v.ID, X: x, Y: y, Units: game.UnitCounts{game.UAxe: 10}, Type: "attack", Mode: "arriveAt", At: at})
		advance(t, w, w.Now()+game.MsPerDay)
		if w.ReportsOf("h1")[0].Time != at {
			t.Fatalf("landed at %d, want %d", w.ReportsOf("h1")[0].Time, at)
		}
	})

	t.Run("four attacks 100 ms apart resolve in that order", func(t *testing.T) {
		w, a := gametest.NewTestWorld(42)
		a.Units[game.UAxe] = 40
		b := game.CreateVillage(a.X+1, a.Y+3, "Second", "", w.Now())
		b.Buildings = a.Buildings
		game.RecomputePoints(b)
		w.PutVillage(b)
		game.SetOwner(w, b, "h1")
		b.Units[game.UAxe] = 40
		x, y := gametest.NearestBarbarianXY(w, a)
		T := w.Now() + 4*game.MsPerHour
		order := []*game.Village{a, b, a, b}
		for i, v := range order {
			mustSchedule(t, w, "h1", game.ScheduleSendInput{OriginID: v.ID, X: x, Y: y, Units: game.UnitCounts{game.UAxe: 10}, Type: "attack", Mode: "arriveAt", At: T + int64(i)*100})
		}
		advance(t, w, w.Now()+game.MsPerDay)
		var hits [][2]any
		for _, r := range w.ReportsOf("h1") {
			if r.Type == "attack" {
				hits = append(hits, [2]any{r.Time, r.OriginID})
			}
		}
		for i, v := range order {
			if i >= len(hits) || hits[i] != [2]any{T + int64(i)*100, v.ID} {
				t.Fatalf("hits %v", hits)
			}
		}
	})

	t.Run("failed send: the troops are gone — a notice instead of an exception", func(t *testing.T) {
		w, v := gametest.NewTestWorld(43)
		v.Units[game.UAxe] = 10
		x, y := gametest.NearestBarbarianXY(w, v)
		mustSchedule(t, w, "h1", game.ScheduleSendInput{OriginID: v.ID, X: x, Y: y, Units: game.UnitCounts{game.UAxe: 10}, Type: "attack", Mode: "sendAt", At: w.Now() + game.MsPerHour})
		v.Units[game.UAxe] = 0
		advance(t, w, w.Now()+2*game.MsPerHour)
		if n := lastNewsFor(w, "h1"); len(w.Commands()) != 0 || n == nil || n.Key != "news.scheduledFailed" || !n.Alert {
			t.Fatalf("news %+v", n)
		}
	})

	t.Run("target changed owner before sending: the support goes to the new owner", func(t *testing.T) {
		w, v := gametest.NewTestWorld(44)
		v.Units[game.USpear] = 10
		bv := gametest.MakeBotVillage(w, v)
		gametest.NewBotPlayer(w, "bot_other")
		game.SetOwner(w, bv, "h1")
		mustSchedule(t, w, "h1", game.ScheduleSendInput{OriginID: v.ID, X: bv.X, Y: bv.Y, Units: game.UnitCounts{game.USpear: 10}, Type: "support", Mode: "sendAt", At: w.Now() + game.MsPerHour})
		game.TransferVillage(w, bv, "bot_other", "")
		advanceTouch(t, w, v, w.Now()+2*game.MsPerHour)
		if len(w.Commands())+len(bv.SupportFrom) == 0 || v.Units[game.USpear] != 0 {
			t.Fatal("support to another player's village did not leave")
		}
	})

	t.Run("cancelling a scheduled send ahead of time", func(t *testing.T) {
		w, v := gametest.NewTestWorld(45)
		v.Units[game.UAxe] = 10
		x, y := gametest.NearestBarbarianXY(w, v)
		s := mustSchedule(t, w, "h1", game.ScheduleSendInput{OriginID: v.ID, X: x, Y: y, Units: game.UnitCounts{game.UAxe: 10}, Type: "attack", Mode: "sendAt", At: w.Now() + game.MsPerHour})
		if err := game.CancelScheduled(w, "h1", s.ID); err != nil {
			t.Fatal(err)
		}
		advance(t, w, w.Now()+2*game.MsPerHour)
		if len(w.Commands()) != 0 || len(w.ScheduledSendsOf("h1")) != 0 {
			t.Fatal("still scheduled")
		}
	})

	t.Run("too late: the troops would already have to be on the way", func(t *testing.T) {
		w, v := gametest.NewTestWorld(46)
		v.Units[game.UAxe] = 10
		x, y := gametest.NearestBarbarianXY(w, v)
		_, err := game.ScheduleSend(w, "h1", game.ScheduleSendInput{OriginID: v.ID, X: x, Y: y, Units: game.UnitCounts{game.UAxe: 10}, Type: "attack", Mode: "arriveAt", At: w.Now() + 1000})
		var re *game.RuleError
		if !errors.As(err, &re) || re.Code != "tooLate" || re.Params["ms"] == nil {
			t.Fatalf("err %v", err)
		}
	})

	t.Run("a scheduled attack on a village that has meanwhile become yours does not go out — with a notice", func(t *testing.T) {
		w, v := gametest.NewTestWorld(45)
		v.Units[game.UAxe] = 10
		w.Player("h1").ProtectedUntil = 0
		bv := gametest.MakeBotVillage(w, v)
		mustSchedule(t, w, "h1", game.ScheduleSendInput{OriginID: v.ID, X: bv.X, Y: bv.Y, Units: game.UnitCounts{game.UAxe: 10}, Type: "attack", Mode: "sendAt", At: w.Now() + game.MsPerHour})
		game.TransferVillage(w, bv, "h1", v.ID)
		advanceTouch(t, w, v, w.Now()+2*game.MsPerHour)
		if n := lastNewsFor(w, "h1"); len(w.Commands()) != 0 || v.Units[game.UAxe] != 10 || n == nil || n.Key != "news.scheduledFailed" {
			t.Fatalf("commands %d news %+v", len(w.Commands()), n)
		}
	})

	t.Run("an attack on its way to a village that has meanwhile become yours stays there as support", func(t *testing.T) {
		w, v := gametest.NewTestWorld(46)
		v.Units[game.UAxe] = 10
		bv := gametest.MakeBotVillage(w, v)
		w.Player("h1").ProtectedUntil = 0
		mustSend(t, w, v, bv.X, bv.Y, game.UnitCounts{game.UAxe: 10}, "attack")
		game.TransferVillage(w, bv, "h1", v.ID)
		advance(t, w, w.Now()+game.MsPerDay)
		if bv.SupportFrom[v.ID][game.UAxe] != 10 || len(w.ReportsOf("h1")) != 0 {
			t.Fatalf("support %+v reports %d", bv.SupportFrom, len(w.ReportsOf("h1")))
		}
	})
}

func TestScheduleSendFromForeignVillageRejected(t *testing.T) {
	w, v := gametest.NewTestWorld(41)
	gametest.NewHumanPlayer(w, "h2")
	v.Units[game.UAxe] = 10
	x, y := gametest.NearestBarbarianXY(w, v)
	var be *game.BlockedError
	if _, err := game.ScheduleSend(w, "h2", game.ScheduleSendInput{OriginID: v.ID, X: x, Y: y, Units: game.UnitCounts{game.UAxe: 10}, Type: "attack", Mode: "sendAt", At: w.Now() + game.MsPerHour}); !errors.As(err, &be) || !be.HasKind("notOwner") {
		t.Fatalf("err %v", err)
	}
	s := mustSchedule(t, w, "h1", game.ScheduleSendInput{OriginID: v.ID, X: x, Y: y, Units: game.UnitCounts{game.UAxe: 10}, Type: "attack", Mode: "sendAt", At: w.Now() + game.MsPerHour})
	if err := game.CancelScheduled(w, "h2", s.ID); !errors.As(err, &be) || !be.HasKind("notOwner") {
		t.Fatalf("cancel err %v", err)
	}
}

// duelWorld has one quitting bot next door with 4000 axes and 5 nobles, no protection.
func duelWorld(t *testing.T) (*gametest.MemWorld, *game.Village, *game.Village) {
	t.Helper()
	cfg := gametest.TestConfig()
	cfg.BotCount, cfg.BeginnerProtectionDays, cfg.LastVillage = 1, 0, "canBeLost"
	w, home := gametest.NewWorldWithHuman(51, cfg)
	w.Player("bot_0").Status = game.StatusQuit
	bv := game.PlayerVillages(w, "bot_0")[0]
	bv.Units[game.UAxe], bv.Units[game.UNoble], bv.Units[game.UScout] = 4000, 5, 5
	bv.Units[game.ULightCav], bv.Units[game.URam], bv.Units[game.USpear] = 5, 5, 5
	return w, home, bv
}

func botAttack(t *testing.T, w *gametest.MemWorld, bv, target *game.Village, u game.UnitCounts) *game.Command {
	t.Helper()
	c, err := game.SendCommand(w, "bot_0", bv.ID, target.X, target.Y, u, "attack", game.SendOpts{})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestDefenceTools(t *testing.T) {
	t.Run("labels match the slowest unit", func(t *testing.T) {
		w, home, bv := duelWorld(t)
		for _, c := range []struct {
			u     game.UnitCounts
			label game.IncomingLabel
		}{
			{game.UnitCounts{game.UScout: 1}, "scout"}, {game.UnitCounts{game.ULightCav: 1}, "cavalry"},
			{game.UnitCounts{game.UAxe: 1}, "infantry"}, {game.UnitCounts{game.URam: 1}, "siege"},
			{game.UnitCounts{game.UNoble: 1, game.UAxe: 50}, "noble"},
		} {
			if got := game.LabelIncoming(w, botAttack(t, w, bv, home, c.u)); got != c.label {
				t.Errorf("%v → %s, want %s", c.u, got, c.label)
			}
		}
	})

	t.Run("dodge: the troops survive and return after the hit", func(t *testing.T) {
		w, home, bv := duelWorld(t)
		home.Units[game.USpear] = 500
		c := botAttack(t, w, bv, home, game.UnitCounts{game.UAxe: 3000})
		if _, err := game.PlanDodge(w, "h1", c.ID); err != nil {
			t.Fatal(err)
		}
		advanceTouch(t, w, home, c.ArrivesAt+5000)
		if home.Units[game.USpear] != 500 || lastReport(w, "h1").Defender.Units.Total() != 0 {
			t.Fatalf("spear %d defender %+v", home.Units[game.USpear], lastReport(w, "h1").Defender.Units)
		}
	})

	t.Run("snipe: the support lands between the 1st and 2nd noble", func(t *testing.T) {
		w, target, bv := duelWorld(t)
		helper := game.CreateVillage(target.X+2, target.Y-1, "Relief", "", w.Now())
		helper.Buildings = target.Buildings
		game.RecomputePoints(helper)
		w.PutVillage(helper)
		game.SetOwner(w, helper, "h1")
		helper.Units[game.USpear] = 3000
		T := w.Now() + 6*game.MsPerHour
		mustSchedule(t, w, "bot_0", game.ScheduleSendInput{OriginID: bv.ID, X: target.X, Y: target.Y, Units: game.UnitCounts{game.UAxe: 3000}, Type: "attack", Mode: "arriveAt", At: T})
		for i := int64(1); i <= 4; i++ {
			mustSchedule(t, w, "bot_0", game.ScheduleSendInput{OriginID: bv.ID, X: target.X, Y: target.Y, Units: game.UnitCounts{game.UNoble: 1}, Type: "attack", Mode: "arriveAt", At: T + i*100})
		}
		last := int64(0)
		for _, s := range w.ScheduledSendsOf("bot_0") {
			last = max(last, s.SendAt)
		}
		advance(t, w, last+1)
		trains := game.IncomingTrains(w, target.ID)
		if len(trains) != 1 || len(trains[0].Commands) != 4 {
			t.Fatalf("trains %+v", trains)
		}
		ids := []string{trains[0].Commands[0].ID, trains[0].Commands[1].ID}
		if _, err := game.PlanSnipe(w, "h1", ids, helper.ID, game.UnitCounts{game.USpear: 3000}); err != nil {
			t.Fatal(err)
		}
		advance(t, w, T+1000)
		if target.OwnerID != "h1" || target.Loyalty < 100-35 {
			t.Fatalf("owner %q loyalty %v", target.OwnerID, target.Loyalty)
		}
	})

	t.Run("dodge and snipe only for own villages", func(t *testing.T) {
		w, home, bv := duelWorld(t)
		gametest.NewHumanPlayer(w, "h2")
		home.Units[game.USpear] = 50
		c := botAttack(t, w, bv, home, game.UnitCounts{game.UAxe: 100})
		var be *game.BlockedError
		if _, err := game.PlanDodge(w, "h2", c.ID); !errors.As(err, &be) || !be.HasKind("notOwner") {
			t.Fatalf("dodge err %v", err)
		}
		if _, err := game.PlanSnipe(w, "h2", []string{c.ID, c.ID}, home.ID, game.UnitCounts{game.USpear: 1}); !errors.As(err, &be) {
			t.Fatalf("snipe err %v", err)
		}
		if slices.ContainsFunc(w.News(0), func(n *game.NewsItem) bool { return n.For == "h2" }) {
			t.Fatal("unexpected news")
		}
	})
}
