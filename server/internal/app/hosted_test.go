package app_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"librehold/internal/app"
	"librehold/internal/game"
)

var ola = &app.Account{ID: 2, Username: "ola", PlayerKey: "k2"}

func ptr[T any](v T) *T { return &v }

func hostForm(name string) app.HostForm {
	return app.HostForm{Name: name}
}

func (f *instantFixture) host(t *testing.T, acct *app.Account, form app.HostForm) *app.HostedCard {
	t.Helper()
	card, err := f.svc.CreatePlayerWorld(context.Background(), acct, form)
	if err != nil {
		t.Fatal(err)
	}
	return card
}

func TestGuestsCannotHost(t *testing.T) {
	f := newInstantFixture(t, app.Config{})
	guest := f.guest(t)
	if app.CanHost(guest) || !app.CanHost(ala) {
		t.Fatal("CanHost")
	}
	if _, err := f.svc.CreatePlayerWorld(context.Background(), guest, hostForm("Gra")); !errors.Is(err, app.ErrForbidden) {
		t.Fatalf("a guest hosted: %v", err)
	}
}

func TestHostFormClamps(t *testing.T) {
	limits := app.HostSettings{MaxSeats: 12, MaxBots: 2}
	cases := []struct {
		name  string
		form  app.HostForm
		check func(cfg game.WorldConfig) bool
	}{
		{"defaults", app.HostForm{}, func(c game.WorldConfig) bool {
			return c.Kind == game.KindPlayer && c.DurationMin == 60 && c.HumanSeats == 12 && c.BotCount == 2 && c.WorldSpeed == 100 &&
				c.UnitSpeed == 100 && c.StartPreset == "instant" && c.BarbarianDensity == "normal" && c.SightRadius == 5 && !c.NoSight &&
				!c.NoFieldBattles && c.BeginnerProtectionDays == 10.0/1440 && c.DominationShare == 0.6 && c.JoinWindowMin == 10 &&
				c.Difficulty == game.InstantWorldConfig.Difficulty && c.BotStance == game.InstantWorldConfig.BotStance &&
				c.MapSize == game.PlayerMapSize(14)
		}},
		{"duration 20", app.HostForm{DurationMin: 20}, func(c game.WorldConfig) bool { return c.DurationMin == 30 }},
		{"duration 130", app.HostForm{DurationMin: 130}, func(c game.WorldConfig) bool { return c.DurationMin == 120 }},
		{"duration 50", app.HostForm{DurationMin: 50}, func(c game.WorldConfig) bool { return c.DurationMin == 45 }},
		{"seats 1", app.HostForm{Seats: 1}, func(c game.WorldConfig) bool { return c.HumanSeats == 2 }},
		{"seats 99", app.HostForm{Seats: 99}, func(c game.WorldConfig) bool { return c.HumanSeats == 12 }},
		{"bots 1", app.HostForm{Bots: ptr(1)}, func(c game.WorldConfig) bool { return c.BotCount == 1 }},
		{"bots 9", app.HostForm{Bots: ptr(9)}, func(c game.WorldConfig) bool { return c.BotCount == 2 }},
		{"bots 0", app.HostForm{Bots: ptr(0)}, func(c game.WorldConfig) bool { return c.BotCount == 0 }},
		{"sight 0", app.HostForm{SightRadius: ptr(0)}, func(c game.WorldConfig) bool { return c.NoSight }},
		{"sight 12", app.HostForm{SightRadius: ptr(12)}, func(c game.WorldConfig) bool { return c.SightRadius == 10 && !c.NoSight }},
		{"domination 40", app.HostForm{DominationPct: ptr(40)}, func(c game.WorldConfig) bool { return c.DominationShare == 0.5 }},
		{"domination 95", app.HostForm{DominationPct: ptr(95)}, func(c game.WorldConfig) bool { return c.DominationShare == 0.9 }},
		{"domination off", app.HostForm{DominationPct: ptr(0)}, func(c game.WorldConfig) bool { return c.DominationShare == 0 }},
		{"late join 7", app.HostForm{LateJoinMin: ptr(7)}, func(c game.WorldConfig) bool { return c.JoinWindowMin == 5 }},
		{"late join 0", app.HostForm{LateJoinMin: ptr(0)}, func(c game.WorldConfig) bool { return c.JoinWindowMin == 0 }},
		{"late join past the end", app.HostForm{DurationMin: 30, LateJoinMin: ptr(60)}, func(c game.WorldConfig) bool { return c.JoinWindowMin == 30 }},
		{"slow speeds", app.HostForm{WorldSpeed: 10, UnitSpeed: 10}, func(c game.WorldConfig) bool { return c.WorldSpeed == 50 && c.UnitSpeed == 50 }},
		{"fast speeds", app.HostForm{WorldSpeed: 500, UnitSpeed: 500}, func(c game.WorldConfig) bool { return c.WorldSpeed == 200 && c.UnitSpeed == 200 }},
		{"many barbarians", app.HostForm{Barbarians: "many"}, func(c game.WorldConfig) bool { return c.BarbarianDensity == "dense" }},
		{"few barbarians", app.HostForm{Barbarians: "few"}, func(c game.WorldConfig) bool { return c.BarbarianDensity == "sparse" }},
		{"fresh start", app.HostForm{StartVillage: "fresh"}, func(c game.WorldConfig) bool { return c.StartPreset == "" }},
		{"no field battles", app.HostForm{FieldBattles: ptr(false)}, func(c game.WorldConfig) bool { return c.NoFieldBattles }},
		{"protection 30", app.HostForm{ProtectionMin: ptr(30)}, func(c game.WorldConfig) bool { return c.BeginnerProtectionDays == 15.0/1440 }},
		{"unknown difficulty", app.HostForm{Difficulty: "godlike", BotStance: "chaotic"}, func(c game.WorldConfig) bool {
			return c.Difficulty == game.InstantWorldConfig.Difficulty && c.BotStance == game.InstantWorldConfig.BotStance
		}},
		{"hard bots", app.HostForm{Difficulty: game.Hard, BotStance: game.StanceAllAgainstHumans}, func(c game.WorldConfig) bool {
			return c.Difficulty == game.Hard && c.BotStance == game.StanceAllAgainstHumans
		}},
	}
	for _, c := range cases {
		if cfg := app.PlayerWorldConfig(c.form, limits); !c.check(cfg) {
			t.Errorf("%s: %+v", c.name, cfg)
		}
	}
}

func TestHostSettingsApplyToNewWorldsOnly(t *testing.T) {
	f := newInstantFixture(t, app.Config{})
	ctx := context.Background()
	before := f.host(t, ala, hostForm("Before"))
	if _, err := f.svc.SetHostSettings(ctx, ola, app.HostSettings{MaxSeats: 4, MaxBots: 1}); !errors.Is(err, app.ErrForbidden) {
		t.Fatalf("a player changed the limits: %v", err)
	}
	for _, bad := range []app.HostSettings{{MaxSeats: 1, MaxBots: 1}, {MaxSeats: 31, MaxBots: 1}, {MaxSeats: 4, MaxBots: 31}, {MaxSeats: 4, MaxBots: -1}} {
		if _, err := f.svc.SetHostSettings(ctx, admin, bad); err == nil {
			t.Fatalf("limits %+v accepted", bad)
		}
	}
	if got, err := f.svc.SetHostSettings(ctx, admin, app.HostSettings{MaxSeats: 4, MaxBots: 1}); err != nil || got.MaxSeats != 4 {
		t.Fatalf("set %+v %v", got, err)
	}
	if got, _ := f.svc.HostSettings(ctx); got != (app.HostSettings{MaxSeats: 4, MaxBots: 1}) {
		t.Fatalf("read back %+v", got)
	}
	after := f.host(t, ola, app.HostForm{Name: "After", Seats: 10, Bots: ptr(3)})
	if after.Seats != 4 || after.Bots != 1 {
		t.Fatalf("new world %d seats, %d bots", after.Seats, after.Bots)
	}
	if again, _ := f.svc.Hosted(ctx, ala); again == nil || again.WorldID != before.WorldID || again.Seats != 10 {
		t.Fatalf("existing world %+v", again)
	}
}

func TestCreateTwiceKeepsOneHostedWorld(t *testing.T) {
	f := newInstantFixture(t, app.Config{})
	ctx := context.Background()
	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i := range errs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, errs[i] = f.svc.CreatePlayerWorld(ctx, ala, hostForm("Tab"))
		}()
	}
	wg.Wait()
	var invalid *app.ErrValidation
	if (errs[0] == nil) == (errs[1] == nil) || !errors.As(errors.Join(errs...), &invalid) || invalid.Code != "alreadyHosting" {
		t.Fatalf("two tabs: %v", errs)
	}
	card, err := f.svc.Hosted(ctx, ala)
	if err != nil || card == nil {
		t.Fatalf("hosted %+v %v", card, err)
	}
	if err := f.worlds.Run(ctx, card.WorldID, func(w app.WorldTx) error { game.EndByHost(w); return nil }); err != nil {
		t.Fatal(err)
	}
	if next := f.host(t, ala, hostForm("Next")); next.WorldID == card.WorldID {
		t.Fatal("the ended world came back")
	}
}

func TestHostNameRules(t *testing.T) {
	f := newInstantFixture(t, app.Config{})
	for _, bad := range []string{"", "   ", "<script>", "a name that is far too long for a world, really"} {
		var invalid *app.ErrValidation
		if _, err := f.svc.CreatePlayerWorld(context.Background(), ala, hostForm(bad)); !errors.As(err, &invalid) || invalid.Code != "nameFormat" {
			t.Fatalf("name %q: %v", bad, err)
		}
	}
	if card := f.host(t, ala, hostForm("  Friday night (#3) & co!  ")); card.Name != "Friday night (#3) & co!" {
		t.Fatalf("name %q", card.Name)
	}
}

func TestServerFullAtTheCap(t *testing.T) {
	f := newInstantFixture(t, app.Config{PlayerMaxWorlds: 2})
	f.host(t, ala, hostForm("One"))
	f.host(t, ola, hostForm("Two"))
	if _, err := f.svc.CreatePlayerWorld(context.Background(), &app.Account{ID: 3, Username: "ewa"}, hostForm("Three")); !errors.Is(err, app.ErrServerFull) {
		t.Fatalf("over the cap: %v", err)
	}
}

func TestPlayerWorldsTakeNoInstantSlots(t *testing.T) {
	f := newInstantFixture(t, app.Config{InstantMaxWorlds: 1})
	card := f.host(t, ala, hostForm("Ours"))
	if r := f.play(t, f.guest(t)); r.WorldID == 0 || r.WorldID == card.WorldID {
		t.Fatalf("play %+v", r)
	}
}

func TestPlayerWorldStartsWaiting(t *testing.T) {
	f := newInstantFixture(t, app.Config{})
	ctx := context.Background()
	card := f.host(t, ala, app.HostForm{Name: "Ours", Bots: ptr(2)})
	if card.Status != "waiting" || card.Humans != 0 || card.Bots != 2 || card.Seats != 10 || card.ResultsCode != "" || len(card.Players) != 0 {
		t.Fatalf("card %+v", card)
	}
	row, err := f.worlds.Get(ctx, card.WorldID)
	if err != nil {
		t.Fatal(err)
	}
	if row.PausedAtMs == nil || *row.PausedAtMs != row.StartedAtMs || row.GameNumber != 0 || row.Config.Kind != game.KindPlayer {
		t.Fatalf("row %+v", row)
	}
	h := row.Host
	if h == nil || h.HostID != ala.ID || h.HostName != "ala" || h.StartedAtMs != nil || len(h.InviteCode) != 20 || len(h.ResultsCode) != 20 ||
		h.InviteCode == h.ResultsCode || card.InviteCode != h.InviteCode {
		t.Fatalf("host %+v", h)
	}
	if app.WorldNow(row, f.clock.NowMs()+60_000) != 0 {
		t.Fatal("a waiting world's clock runs")
	}
	err = f.worlds.Read(ctx, card.WorldID, func(w app.WorldTx) error {
		bots := 0
		for _, p := range w.Players() {
			if !p.IsHuman && len(p.VillageIDs) == 1 {
				bots++
			}
		}
		if bots != 2 {
			t.Fatalf("%d bots placed", bots)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if mine, _ := f.svc.Hosted(ctx, ala); mine == nil || mine.WorldID != card.WorldID {
		t.Fatalf("hosted %+v", mine)
	}
	if theirs, _ := f.svc.Hosted(ctx, ola); theirs != nil {
		t.Fatalf("ola hosts %+v", theirs)
	}
}

func TestLobbyKeepsPlayerWorldsOut(t *testing.T) {
	f := newInstantFixture(t, app.Config{})
	ctx := context.Background()
	card := f.host(t, ala, hostForm("Ours"))
	view, err := f.svc.Lobby(ctx, ala)
	if err != nil {
		t.Fatal(err)
	}
	for _, w := range append(view.Mine, view.Open...) {
		if w.ID == card.WorldID {
			t.Fatal("a player world in the lobby lists")
		}
	}
	if view.Hosted == nil || view.Hosted.WorldID != card.WorldID || !view.Host.CanHost || view.Host.MaxSeats != 10 || view.Host.Full {
		t.Fatalf("lobby %+v %+v", view.Hosted, view.Host)
	}
	if guest, _ := f.svc.Lobby(ctx, &app.Account{ID: 99, Username: "Guest_99", IsGuest: true}); guest.Host.CanHost || guest.Hosted != nil {
		t.Fatalf("guest lobby %+v", guest.Host)
	}
}

func TestHostedFormClampsSightRadiusToLimits(t *testing.T) {
	limits := app.HostSettings{MaxSeats: 12, MaxBots: 2}
	over := int(game.WorldLimits.SightRadius.Max) + 1
	if c := app.PlayerWorldConfig(app.HostForm{SightRadius: &over}, limits); c.SightRadius != game.WorldLimits.SightRadius.Max || c.NoSight {
		t.Fatalf("%d → %v (no sight %v)", over, c.SightRadius, c.NoSight)
	}
	if c := app.PlayerWorldConfig(app.HostForm{SightRadius: ptr(0)}, limits); !c.NoSight {
		t.Fatal("0 must mean no sight")
	}
}
