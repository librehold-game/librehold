package game_test

import (
	"encoding/json"
	"errors"
	"math"
	"os"
	"slices"
	"testing"

	"librehold/internal/game"
	"librehold/internal/game/gametest"
)

// nearestUnstoredBarbarian is a generated barbarian tile not yet stored.
func nearestUnstoredBarbarian(t *testing.T, w *gametest.MemWorld, v *game.Village, minDist float64) (int, int) {
	t.Helper()
	bx, by, bestD := 0, 0, math.Inf(1)
	for x := v.X - 15; x <= v.X+15; x++ {
		for y := v.Y - 15; y <= v.Y+15; y++ {
			d := game.Distance(v.X, v.Y, x, y)
			if d < minDist || !game.TileHasVillage(w.Seed(), w.Config(), x, y) || w.Village(game.VillageIDFor(x, y)) != nil {
				continue
			}
			if d < bestD {
				bx, by, bestD = x, y, d
			}
		}
	}
	if math.IsInf(bestD, 1) {
		t.Fatal("no barbarian nearby")
	}
	return bx, by
}

func mustSend(t *testing.T, w game.World, v *game.Village, x, y int, u game.UnitCounts, typ string) *game.Command {
	t.Helper()
	c, err := game.SendCommand(w, game.PlayerID(v.OwnerID), v.ID, x, y, u, typ, game.SendOpts{})
	if err != nil {
		t.Fatalf("SendCommand: %v", err)
	}
	return c
}

func sendKinds(w game.World, v *game.Village, x, y int, u game.UnitCounts, typ string) []string {
	return blockerKinds(game.CheckSend(w, game.PlayerID(v.OwnerID), v.ID, x, y, u, typ).Blockers)
}

func TestTravel(t *testing.T) {
	t.Run("formula: distance × slowest × 60 000 / unitSpeed — world speed does not shorten the march", func(t *testing.T) {
		if game.Distance(0, 0, 3, 4) != 5 || game.SlowestSpeed(game.UnitCounts{game.USpear: 1, game.ULightCav: 5}) != 18 ||
			game.SlowestSpeed(game.UnitCounts{game.ULightCav: 5}) != 10 {
			t.Fatal("distance / slowest")
		}
		if game.TravelTimeMs(5, 18, 1) != 90*game.MsPerMinute || game.TravelTimeMs(5, 18, 2) != 45*game.MsPerMinute ||
			game.TravelTimeMs(5, 18, 4) != 22*game.MsPerMinute+30_000 {
			t.Fatal("travel")
		}
	})

	t.Run("the march depends only on unit speed, not on world speed", func(t *testing.T) {
		w, v := gametest.NewTestWorld(5)
		w.Config().WorldSpeed = 8
		x, y := gametest.NearestBarbarianXY(w, v)
		v.Units[game.USpear] = 10
		c := mustSend(t, w, v, x, y, game.UnitCounts{game.USpear: 10}, "attack")
		if c.ArrivesAt-w.Now() != game.TravelTimeMs(game.Distance(v.X, v.Y, x, y), 18, 1) {
			t.Fatal("world speed changed the march")
		}
	})
}

func TestCommands(t *testing.T) {
	t.Run("sending subtracts units and validates", func(t *testing.T) {
		w, v := gametest.NewTestWorld(12)
		v.Units[game.USpear], v.Units[game.UAxe] = 50, 10
		tx, ty := nearestUnstoredBarbarian(t, w, v, 1)
		if !slices.Contains(sendKinds(w, v, tx, ty, game.UnitCounts{game.USpear: 60}, "attack"), "notEnough") ||
			!slices.Contains(sendKinds(w, v, v.X, v.Y, game.UnitCounts{game.USpear: 1}, "attack"), "self") ||
			!slices.Contains(sendKinds(w, v, tx, ty, game.UnitCounts{}, "attack"), "noUnits") ||
			!slices.Contains(sendKinds(w, v, tx, ty, game.UnitCounts{game.USpear: 1}, "support"), "supportTargetBarbarian") {
			t.Fatal("validation")
		}
		c := mustSend(t, w, v, tx, ty, game.UnitCounts{game.USpear: 20, game.UAxe: 5}, "attack")
		if v.Units[game.USpear] != 30 || v.Units[game.UAxe] != 5 || w.Village(c.TargetID) == nil {
			t.Fatal("units not taken / target not stored")
		}
		if c.ArrivesAt-c.DepartedAt != game.TravelTimeMs(game.Distance(v.X, v.Y, tx, ty), 18, 1) {
			t.Fatal("travel")
		}
		if game.UnitsAway(w, v.ID)[game.USpear] != 20 || game.PopulationUsed(w, v) != game.VillagePopulationUsed(v)+25 {
			t.Fatal("away / population")
		}
		if w.Human("h1").Stats.AttacksSent != 1 {
			t.Fatal("stats")
		}
	})

	t.Run("a target without a village is rejected, scouting with scouts only is allowed", func(t *testing.T) {
		w, v := gametest.NewTestWorld(12)
		v.Units[game.UScout] = 5
		ex, ey, found := 0, 0, false
		for x := v.X - 5; x <= v.X+5 && !found; x++ {
			for y := v.Y - 5; y <= v.Y+5 && !found; y++ {
				if !game.TileHasVillage(w.Seed(), w.Config(), x, y) && !(x == v.X && y == v.Y) {
					ex, ey, found = x, y, true
				}
			}
		}
		if !slices.Contains(sendKinds(w, v, ex, ey, game.UnitCounts{game.UScout: 1}, "attack"), "noTarget") {
			t.Fatal("noTarget")
		}
		tx, ty := nearestUnstoredBarbarian(t, w, v, 1)
		c := game.CheckSend(w, "h1", v.ID, tx, ty, game.UnitCounts{game.UScout: 2}, "attack")
		if len(c.Blockers) != 0 || !c.IsScoutMission {
			t.Fatalf("%+v", c)
		}
	})

	t.Run("cancel only within 10 % of the march time; troops take as long to return as they marched", func(t *testing.T) {
		w, v := gametest.NewTestWorld(12)
		v.Units[game.USpear] = 10
		tx, ty := nearestUnstoredBarbarian(t, w, v, 5)
		c := mustSend(t, w, v, tx, ty, game.UnitCounts{game.USpear: 10}, "attack")
		travel := c.ArrivesAt - c.DepartedAt
		advance(t, w, int64(math.Floor(float64(travel)*0.05)))
		if !game.CanCancelCommand(w, c) {
			t.Fatal("cannot cancel")
		}
		if err := game.CancelCommand(w, "h1", c.ID); err != nil {
			t.Fatal(err)
		}
		if c.Type != "return" || c.TargetID != v.ID || c.ArrivesAt-c.DepartedAt != int64(math.Floor(float64(travel)*0.05)) {
			t.Fatalf("%+v", c)
		}
		advanceTouch(t, w, v, c.ArrivesAt)
		if v.Units[game.USpear] != 10 {
			t.Fatal("not home")
		}
		c2 := mustSend(t, w, v, tx, ty, game.UnitCounts{game.USpear: 10}, "attack")
		advance(t, w, c2.DepartedAt+int64(math.Ceil(float64(travel)*0.2)))
		if game.CanCancelCommand(w, c2) || game.CancelCommand(w, "h1", c2.ID) == nil {
			t.Fatal("late cancel allowed")
		}
	})

	t.Run("support for an own village lands in supportFrom and counts towards home population; withdrawal returns", func(t *testing.T) {
		w, home := gametest.NewTestWorld(12)
		home.Units[game.USpear] = 30
		second := game.CreateVillage(home.X+3, home.Y+4, "Second", "", w.Now())
		w.PutVillage(second)
		game.SetOwner(w, second, "h1")
		c := mustSend(t, w, home, second.X, second.Y, game.UnitCounts{game.USpear: 12}, "support")
		advance(t, w, c.ArrivesAt)
		if second.SupportFrom[home.ID] != (game.UnitCounts{game.USpear: 12}) || second.Units[game.USpear] != 0 {
			t.Fatalf("support %+v", second.SupportFrom)
		}
		if game.UnitsAway(w, home.ID)[game.USpear] != 12 || game.PopulationUsed(w, home) != game.VillagePopulationUsed(home)+12 {
			t.Fatal("away / population")
		}
		part := game.UnitCounts{game.USpear: 5}
		ret, err := game.WithdrawSupport(w, "h1", second.ID, home.ID, &part)
		if err != nil || second.SupportFrom[home.ID] != (game.UnitCounts{game.USpear: 7}) {
			t.Fatalf("err %v support %+v", err, second.SupportFrom)
		}
		advanceTouch(t, w, home, ret.ArrivesAt)
		if home.Units[game.USpear] != 18+5 {
			t.Fatalf("home %d", home.Units[game.USpear])
		}
		if _, err := game.WithdrawSupport(w, "h1", second.ID, home.ID, nil); err != nil {
			t.Fatal(err)
		}
		if _, ok := second.SupportFrom[home.ID]; ok {
			t.Fatal("support left behind")
		}
	})

	t.Run("cancelled command: no arrival, the return fires once", func(t *testing.T) {
		w, v := gametest.NewTestWorld(1)
		v.Units[game.UAxe] = 10
		x, y := gametest.NearestBarbarianXY(w, v)
		c := mustSend(t, w, v, x, y, game.UnitCounts{game.UAxe: 10}, "attack")
		advance(t, w, w.Now()+game.MsPerMinute)
		if err := game.CancelCommand(w, "h1", c.ID); err != nil {
			t.Fatal(err)
		}
		advanceTouch(t, w, v, w.Now()+game.MsPerDay)
		if len(w.ReportsOf("h1")) != 0 || v.Units[game.UAxe] != 10 || len(w.Commands()) != 0 {
			t.Fatalf("reports %d axe %d commands %d", len(w.ReportsOf("h1")), v.Units[game.UAxe], len(w.Commands()))
		}
	})
}

// twoHumans: h1 (spears at home) and h2, each with a start village.
func twoHumans(t *testing.T) (*gametest.MemWorld, *game.Village, *game.Village) {
	t.Helper()
	w, v1 := gametest.NewTestWorld(12)
	v2, err := game.JoinHuman(w, "h2", "Friend", 2, game.QuadrantRandom)
	if err != nil {
		t.Fatal(err)
	}
	v1.Units[game.USpear] = 30
	return w, v1, v2
}

func TestSupportToFriendVillage(t *testing.T) {
	w, v1, v2 := twoHumans(t)
	c := mustSend(t, w, v1, v2.X, v2.Y, game.UnitCounts{game.USpear: 12}, "support")
	advance(t, w, c.ArrivesAt)
	if v2.SupportFrom[v1.ID] != (game.UnitCounts{game.USpear: 12}) {
		t.Fatalf("support %+v", v2.SupportFrom)
	}
	if w.Human("h1").Stats.SupportsSent != 1 {
		t.Fatal("stats")
	}
}

func TestHostSendsBackSupport(t *testing.T) {
	w, v1, v2 := twoHumans(t)
	c := mustSend(t, w, v1, v2.X, v2.Y, game.UnitCounts{game.USpear: 12}, "support")
	advance(t, w, c.ArrivesAt)
	ret, err := game.WithdrawSupport(w, "h2", v2.ID, v1.ID, nil)
	if err != nil || ret == nil || ret.Type != "return" || ret.TargetID != v1.ID || ret.Units != (game.UnitCounts{game.USpear: 12}) {
		t.Fatalf("ret %+v err %v", ret, err)
	}
	if _, ok := v2.SupportFrom[v1.ID]; ok {
		t.Fatal("support still stationed")
	}
	advanceTouch(t, w, v1, ret.ArrivesAt)
	if v1.Units[game.USpear] != 30 {
		t.Fatalf("home %d", v1.Units[game.USpear])
	}
}

func TestStrangerCannotWithdraw(t *testing.T) {
	w, v1, v2 := twoHumans(t)
	c := mustSend(t, w, v1, v2.X, v2.Y, game.UnitCounts{game.USpear: 12}, "support")
	advance(t, w, c.ArrivesAt)
	if _, err := game.JoinHuman(w, "h3", "Stranger", 3, game.QuadrantRandom); err != nil {
		t.Fatal(err)
	}
	_, err := game.WithdrawSupport(w, "h3", v2.ID, v1.ID, nil)
	var be *game.BlockedError
	if !errors.As(err, &be) || !be.HasKind("notOwner") || v2.SupportFrom[v1.ID][game.USpear] != 12 {
		t.Fatalf("err %v", err)
	}
}

func TestSendFromForeignVillageRejected(t *testing.T) {
	w, v1, v2 := twoHumans(t)
	_, err := game.SendCommand(w, "h2", v1.ID, v2.X, v2.Y, game.UnitCounts{game.USpear: 5}, "support", game.SendOpts{})
	var be *game.BlockedError
	if !errors.As(err, &be) || !be.HasKind("notOwner") || v1.Units[game.USpear] != 30 {
		t.Fatalf("err %v", err)
	}
}

func TestCancelForeignCommandRejected(t *testing.T) {
	w, v1, v2 := twoHumans(t)
	c := mustSend(t, w, v1, v2.X, v2.Y, game.UnitCounts{game.USpear: 5}, "support")
	err := game.CancelCommand(w, "h2", c.ID)
	var be *game.BlockedError
	if !errors.As(err, &be) || !be.HasKind("notOwner") || c.Type != "support" {
		t.Fatalf("err %v", err)
	}
}

func TestTravelGolden(t *testing.T) {
	raw, err := os.ReadFile("testdata/golden/travel.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		UnitSpeed float64     `json:"unitSpeed"`
		Unit      game.UnitID `json:"unit"`
		Dist      float64     `json:"dist"`
		Ms        int64       `json:"ms"`
	}
	if err := json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	if len(cases) == 0 {
		t.Fatal("empty fixture")
	}
	for i, c := range cases {
		cases[i].Ms = game.TravelTimeMs(c.Dist, game.UnitDefs[c.Unit].Speed, c.UnitSpeed)
	}
	game.Golden(t, "travel", cases)
}
