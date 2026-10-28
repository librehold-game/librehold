package game_test

import (
	"errors"
	"math"
	"slices"
	"testing"

	"librehold/internal/game"
	"librehold/internal/game/gametest"
)

func twoVillages(t *testing.T, seed uint32) (*gametest.MemWorld, *game.Village, *game.Village) {
	t.Helper()
	w, a := gametest.NewTestWorld(seed)
	a.Buildings[game.BMarket], a.Buildings[game.BStorage] = 5, 15
	a.Resources = game.Resources{Wood: 9000, Clay: 9000, Iron: 9000}
	b := game.CreateVillage(a.X+3, a.Y+4, "B", "", w.Now())
	b.Buildings[game.BStorage], b.Buildings[game.BMarket] = 15, 2
	w.PutVillage(b)
	game.SetOwner(w, b, "h1")
	return w, a, b
}

func tradeKinds(w game.World, actor game.PlayerID, from, to string, amount game.Resources) []string {
	return blockerKinds(game.CheckTrade(w, actor, from, to, amount).Blockers)
}

func TestMarket(t *testing.T) {
	t.Run("sending: merchants carry 1000 each, 6 min/tile, merchants return", func(t *testing.T) {
		w, a, b := twoVillages(t, 5)
		check := game.CheckTrade(w, "h1", a.ID, b.ID, game.Resources{Wood: 2500, Iron: 100})
		if len(check.Blockers) != 0 || check.Merchants != 3 || check.TravelMs != 5*6*game.MsPerMinute {
			t.Fatalf("%+v", check)
		}
		c, err := game.SendTrade(w, "h1", a.ID, b.ID, game.Resources{Wood: 2500, Iron: 100})
		if err != nil {
			t.Fatal(err)
		}
		if a.Resources.Wood != 6500 || game.MerchantsAvailable(w, a) != 2 {
			t.Fatalf("wood %v merchants %d", a.Resources.Wood, game.MerchantsAvailable(w, a))
		}
		if !slices.Contains(tradeKinds(w, "h1", a.ID, b.ID, game.Resources{Wood: 3000}), "merchants") {
			t.Fatal("merchants blocker")
		}
		advanceTouch(t, w, b, c.ArrivesAt)
		if b.Resources.Wood < 2500 || b.Resources.Wood >= 2510 || game.MerchantsAvailable(w, a) != 2 {
			t.Fatalf("delivered %v merchants %d", b.Resources.Wood, game.MerchantsAvailable(w, a))
		}
		var back *game.Command
		for _, x := range w.Commands() {
			if x.Type == "trade" && x.Returning {
				back = x
			}
		}
		if back == nil {
			t.Fatal("merchants not returning")
		}
		advance(t, w, back.ArrivesAt)
		if game.MerchantsAvailable(w, a) != 5 || len(w.Commands()) != 0 {
			t.Fatalf("merchants %d commands %d", game.MerchantsAvailable(w, a), len(w.Commands()))
		}
		if w.Human("h1").Stats.TradesSent != 1 {
			t.Fatal("stats")
		}
	})

	t.Run("blockers: no market, no village, no resources", func(t *testing.T) {
		w, a, b := twoVillages(t, 5)
		a.Buildings[game.BMarket] = 0
		if !slices.Contains(tradeKinds(w, "h1", a.ID, b.ID, game.Resources{Wood: 10}), "noMarket") {
			t.Fatal("noMarket")
		}
		a.Buildings[game.BMarket] = 1
		if !slices.Contains(tradeKinds(w, "h1", a.ID, "v:0:0", game.Resources{Wood: 10}), "noTarget") ||
			!slices.Contains(tradeKinds(w, "h1", a.ID, b.ID, game.Resources{Wood: 99999}), "resources") ||
			!slices.Contains(tradeKinds(w, "h1", a.ID, a.ID, game.Resources{Wood: 10}), "self") {
			t.Fatal("blockers")
		}
	})

	t.Run("surplus and distribution", func(t *testing.T) {
		_, a, b := twoVillages(t, 5)
		a.Resources = game.Resources{Wood: 20000, Clay: 100, Iron: 21000}
		capacity := 1000 * math.Pow(1.2294, 14)
		if s := game.Surplus(a, 0.8); s.Wood != math.Floor(20000-capacity*0.8) || s.Clay != 0 {
			t.Fatalf("surplus %+v", s)
		}
		b.Resources = game.Resources{Clay: 500, Iron: 21000}
		if got := game.BalanceAmount(a, b); got != (game.Resources{Wood: 10000}) {
			t.Fatalf("balance %+v", got)
		}
	})

	t.Run("travelling trader: an offer every 8 h, deterministic, once per window", func(t *testing.T) {
		w, v := gametest.NewTestWorld(3)
		v.Buildings[game.BMarket], v.Buildings[game.BStorage] = 1, 15
		tr := &w.Human("h1").Trader
		if tr.Offer != game.TraderOfferAt(w, 0) || tr.Offer.Give == tr.Offer.Get || tr.Offer.GetAmount != math.Round(tr.Offer.GiveAmount*0.8) {
			t.Fatalf("offer %+v", tr.Offer)
		}
		v.Resources = game.Resources{Wood: 10000, Clay: 10000, Iron: 10000}
		if !game.CanAcceptTrader(w, "h1", v) {
			t.Fatal("cannot accept")
		}
		if err := game.AcceptTrader(w, "h1", v.ID); err != nil {
			t.Fatal(err)
		}
		if v.Resources.Get(tr.Offer.Give) != 10000-tr.Offer.GiveAmount || game.CanAcceptTrader(w, "h1", v) {
			t.Fatal("accept")
		}
		advance(t, w, 8*game.MsPerHour)
		tr = &w.Human("h1").Trader
		if tr.OfferIndex != 1 || tr.Accepted || tr.Offer != game.TraderOfferAt(w, 1) {
			t.Fatalf("after 8 h %+v", tr)
		}
		advance(t, w, 3*game.MsPerDay)
		if w.Human("h1").Trader.OfferIndex != 9 {
			t.Fatalf("index %d", w.Human("h1").Trader.OfferIndex)
		}
	})
}

func TestTradeToFriend(t *testing.T) {
	w, a, _ := twoVillages(t, 5)
	friend, err := game.JoinHuman(w, "h2", "Friend", 2, game.QuadrantRandom)
	if err != nil {
		t.Fatal(err)
	}
	before := friend.Resources.Wood
	c, err := game.SendTrade(w, "h1", a.ID, friend.ID, game.Resources{Wood: 1000})
	if err != nil {
		t.Fatal(err)
	}
	advanceTouch(t, w, friend, c.ArrivesAt)
	if friend.Resources.Wood <= before {
		t.Fatalf("friend wood %v → %v", before, friend.Resources.Wood)
	}
	var be *game.BlockedError
	if _, err := game.SendTrade(w, "h2", a.ID, friend.ID, game.Resources{Wood: 10}); !errors.As(err, &be) || !be.HasKind("notOwner") {
		t.Fatalf("foreign trade err %v", err)
	}
}

func TestTraderIsPerHuman(t *testing.T) {
	w, v := gametest.NewTestWorld(3)
	v.Buildings[game.BMarket] = 1
	v.Resources = game.Resources{Wood: 10000, Clay: 10000, Iron: 10000}
	v2, err := game.JoinHuman(w, "h2", "Friend", 2, game.QuadrantRandom)
	if err != nil {
		t.Fatal(err)
	}
	v2.Buildings[game.BMarket] = 1
	v2.Resources = game.Resources{Wood: 10000, Clay: 10000, Iron: 10000}
	if err := game.AcceptTrader(w, "h1", v.ID); err != nil {
		t.Fatal(err)
	}
	if !game.CanAcceptTrader(w, "h2", v2) || w.Human("h2").Trader.Accepted {
		t.Fatal("h1's acceptance used h2's offer")
	}
	if !w.IsScheduled("trader:h1") || !w.IsScheduled("trader:h2") {
		t.Fatal("trader events not per human")
	}
	var be *game.BlockedError
	if err := game.AcceptTrader(w, "h2", v.ID); !errors.As(err, &be) || !be.HasKind("notOwner") {
		t.Fatalf("foreign accept err %v", err)
	}
}
