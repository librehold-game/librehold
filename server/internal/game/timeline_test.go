package game_test

import (
	"fmt"
	"testing"

	"librehold/internal/game"
	"librehold/internal/game/gametest"
)

// shortWorld is an instant world without beginner protection, with human h1.
func shortWorld(t *testing.T) (*gametest.MemWorld, *game.Village) {
	t.Helper()
	cfg := game.InstantWorldConfig
	cfg.BeginnerProtectionDays = 0
	w := gametest.NewMemWorld(7, cfg, 0)
	if err := game.NewWorld(w); err != nil {
		t.Fatal(err)
	}
	v, err := game.JoinHuman(w, "h1", "Ala", 1, game.QuadrantRandom)
	if err != nil {
		t.Fatal(err)
	}
	return w, v
}

func ofKind(entries []game.TimelineEntry, kind string) []game.TimelineEntry {
	var out []game.TimelineEntry
	for _, e := range entries {
		if e.Kind == kind {
			out = append(out, e)
		}
	}
	return out
}

// byAttacker keeps the battles one player fought; the bots of an instant world raid while time passes.
func byAttacker(entries []game.TimelineEntry, p game.PlayerRef) []game.TimelineEntry {
	var out []game.TimelineEntry
	for _, e := range ofKind(entries, "battle") {
		if e.Attacker == p {
			out = append(out, e)
		}
	}
	return out
}

func TestTimelineRecordsSpawnedVillages(t *testing.T) {
	cfg := game.InstantWorldConfig
	w := gametest.NewMemWorld(7, cfg, 0)
	if err := game.NewWorld(w); err != nil {
		t.Fatal(err)
	}
	villages := ofKind(w.TakeTimeline(), "village")
	if len(villages) != cfg.BotCount {
		t.Fatalf("%d village entries for %d bots", len(villages), cfg.BotCount)
	}
	for _, e := range villages {
		v := w.Village(game.VillageIDFor(int(e.X), int(e.Y)))
		if e.At != 0 || e.Owner == "" || v == nil || v.OwnerID != e.Owner {
			t.Fatalf("entry %+v, village %+v", e, v)
		}
	}
	home, err := game.JoinHuman(w, "h1", "Ala", 1, game.QuadrantRandom)
	if err != nil {
		t.Fatal(err)
	}
	joined := ofKind(w.TakeTimeline(), "village")
	if len(joined) != 1 || joined[0].Owner != "h1" || int(joined[0].X) != home.X || int(joined[0].Y) != home.Y {
		t.Fatalf("after joining %+v", joined)
	}
}

func TestTimelineRecordsEveryOwnerChange(t *testing.T) {
	w, v := shortWorld(t)
	target := ensureNearestBarbarian(w, v)
	disarm(w, target.ID)
	target.Loyalty = 1
	w.TakeTimeline()
	attackAndLand(t, w, v, target.X, target.Y, game.UnitCounts{game.UAxe: 50, game.UNoble: 1})
	owners := ofKind(w.TakeTimeline(), "owner")
	if len(owners) != 1 || owners[0].Owner != "h1" || int(owners[0].X) != target.X || int(owners[0].Y) != target.Y {
		t.Fatalf("conquest %+v", owners)
	}
	game.TransferVillage(w, target, "", "")
	owners = ofKind(w.TakeTimeline(), "owner")
	if len(owners) != 1 || owners[0].Owner != "" || owners[0].At != w.Now() {
		t.Fatalf("abandonment %+v", owners)
	}
}

func TestTimelineRecordsBattlesNotScouting(t *testing.T) {
	w, v := shortWorld(t)
	target := ensureNearestBarbarian(w, v)
	defenders := game.DefenderUnitsOf(target).Total()
	w.TakeTimeline()
	attackAndLand(t, w, v, target.X, target.Y, game.UnitCounts{game.UAxe: 60})
	battles := byAttacker(w.TakeTimeline(), "h1")
	if len(battles) != 1 {
		t.Fatalf("battles %+v", battles)
	}
	b := battles[0]
	if b.FromX != v.X || b.FromY != v.Y || int(b.X) != target.X || int(b.Y) != target.Y || b.Attacker != "h1" || b.Defender != "" ||
		b.Troops != 60+defenders {
		t.Fatalf("battle %+v (from %d|%d, defenders %d)", b, v.X, v.Y, defenders)
	}
	if err := gametest.ScoutOnce(w, v, target); err != nil {
		t.Fatal(err)
	}
	if battles := byAttacker(w.TakeTimeline(), "h1"); len(battles) != 0 {
		t.Fatalf("scouting made battles %+v", battles)
	}
}

func TestTimelineRecordsFieldBattlesAtTheMeetingPoint(t *testing.T) {
	cfg := game.InstantWorldConfig
	cfg.BeginnerProtectionDays = 0
	w, a := gametest.NewWorldWithHuman(31, cfg)
	gametest.NewHumanPlayer(w, "h2")
	b := gametest.AddStartVillage(w, "h2", a.X+8, a.Y)
	a.Units[game.UAxe] = 10
	b.Units[game.UAxe] = 100
	attack, err := game.SendCommand(w, "h2", b.ID, a.X, a.Y, game.UnitCounts{game.UAxe: 100}, "attack", game.SendOpts{})
	if err != nil {
		t.Fatal(err)
	}
	w.TakeTimeline()
	c, err := game.SendIntercept(w, "h1", a.ID, attack.ID, game.UnitCounts{game.UAxe: 10}, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := gametest.Advance(w, c.ArrivesAt); err != nil {
		t.Fatal(err)
	}
	battles := byAttacker(w.TakeTimeline(), "h1")
	if len(battles) != 1 {
		t.Fatalf("battles %+v", battles)
	}
	f := battles[0]
	if f.X != c.Point.X || f.Y != c.Point.Y || f.FromX != a.X || f.FromY != a.Y || f.Attacker != "h1" || f.Defender != "h2" || f.Troops != 110 {
		t.Fatalf("field battle %+v at %+v", f, *c.Point)
	}
}

func TestLongWorldsRecordNoTimeline(t *testing.T) {
	w, v := gametest.NewTestWorld(7)
	target := ensureNearestBarbarian(w, v)
	attackAndLand(t, w, v, target.X, target.Y, game.UnitCounts{game.UAxe: 60})
	if got := w.TakeTimeline(); len(got) != 0 {
		t.Fatalf("a long world kept %+v", got)
	}
}

func TestBattleSizeClass(t *testing.T) {
	for troops, want := range map[int]int{0: 1, 99: 1, 100: 2, 999: 2, 1000: 3, 4999: 3, 5000: 4} {
		if got := game.BattleSizeClass(troops); got != want {
			t.Errorf("%d troops: class %d, want %d", troops, got, want)
		}
	}
}

func TestPackTimelineNumbersOwnersAndListsBarbarians(t *testing.T) {
	cfg := game.InstantWorldConfig
	players := []*game.Player{{ID: "bot_1", Color: "#112233"}, {ID: "h1", Color: "#445566", IsHuman: true}}
	entries := []game.TimelineEntry{
		{At: 0, Kind: "village", X: 4, Y: 5, Owner: "bot_1"},
		{At: 10, Kind: "village", X: 7, Y: 8, Owner: "h1"},
		{At: 20, Kind: "battle", X: 4, Y: 5, FromX: 7, FromY: 8, Attacker: "h1", Defender: "bot_1", Troops: 1500},
		{At: 30, Kind: "owner", X: 4, Y: 5, Owner: "h1"},
		{At: 40, Kind: "owner", X: 4, Y: 5},
	}
	f := game.PackTimeline(7, &cfg, 50, players, entries)
	if f.Version != 1 || f.MapSize != cfg.MapSize || f.EndAt != 50 {
		t.Fatalf("header %+v", f)
	}
	if len(f.Owners) != 2 || f.Owners[0] != (game.TimelineOwner{PlayerID: "bot_1", Color: "#112233", Bot: true}) ||
		f.Owners[1] != (game.TimelineOwner{PlayerID: "h1", Color: "#445566"}) {
		t.Fatalf("owners %+v", f.Owners)
	}
	want := [][]float64{{0, 0, 4, 5, 1}, {10, 0, 7, 8, 2}, {20, 2, 7, 8, 4, 5, 2, 1, 3}, {30, 1, 4, 5, 2}, {40, 1, 4, 5, 0}}
	if len(f.Events) != len(want) {
		t.Fatalf("events %v", f.Events)
	}
	for i := range want {
		if fmt.Sprint(f.Events[i]) != fmt.Sprint(want[i]) {
			t.Fatalf("event %d: %v, want %v", i, f.Events[i], want[i])
		}
	}
	n := 0
	for x := range cfg.MapSize {
		for y := range cfg.MapSize {
			if game.TileHasVillage(7, &cfg, x, y) {
				n++
			}
		}
	}
	if len(f.Barbarians) != n || n == 0 {
		t.Fatalf("%d barbarians, want %d", len(f.Barbarians), n)
	}
}
