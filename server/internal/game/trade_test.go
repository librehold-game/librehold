package game_test

import (
	"slices"
	"testing"

	"librehold/internal/game"
	"librehold/internal/game/gametest"
)

// tradeWorld builds a world where h1 has village a and human h2 has village b, both with markets.
func tradeWorld(t *testing.T, seed uint32) (*gametest.MemWorld, *game.Village, *game.Village) {
	t.Helper()
	w, a := gametest.NewTestWorld(seed)
	a.Buildings[game.BMarket], a.Buildings[game.BStorage] = 5, 15
	a.Resources = game.Resources{Wood: 9000, Clay: 9000, Iron: 9000}
	gametest.NewHumanPlayer(w, "h2")
	b := gametest.AddStartVillage(w, "h2", a.X+3, a.Y+4)
	b.Buildings[game.BMarket], b.Buildings[game.BStorage] = 5, 15
	b.Resources = game.Resources{Wood: 9000, Clay: 9000, Iron: 9000}
	return w, a, b
}

func offerKinds(t *testing.T, fn func() error) []string {
	t.Helper()
	be, ok := fn().(*game.BlockedError)
	if !ok {
		t.Fatalf("expected blocked error")
	}
	return blockerKinds(be.Blockers)
}

func TestTradeOfferLifecycle(t *testing.T) {
	t.Run("create: a post is visible to the poster and to a player it can reach", func(t *testing.T) {
		w, a, _ := tradeWorld(t, 5)
		o, err := game.CreateTradeOffer(w, "h1", game.TradeOfferInput{OriginID: a.ID, Give: game.Resources{Wood: 100}, Want: game.Resources{Iron: 100}})
		if err != nil {
			t.Fatal(err)
		}
		if got := game.VisibleOffers(w, "h2"); len(got) != 1 || got[0].ID != o.ID {
			t.Fatalf("the other player sees %+v", got)
		}
		if got := game.VisibleOffers(w, "h1"); len(got) != 1 {
			t.Fatal("own offer not visible")
		}
	})

	t.Run("create: blockers (no market, empty)", func(t *testing.T) {
		w, a, _ := tradeWorld(t, 5)
		a.Buildings[game.BMarket] = 0
		if !slices.Contains(offerKinds(t, func() error {
			_, e := game.CreateTradeOffer(w, "h1", game.TradeOfferInput{OriginID: a.ID, Give: game.Resources{Wood: 1}})
			return e
		}), "noMarket") {
			t.Fatal("noMarket")
		}
		a.Buildings[game.BMarket] = 5
		if !slices.Contains(offerKinds(t, func() error {
			_, e := game.CreateTradeOffer(w, "h1", game.TradeOfferInput{OriginID: a.ID})
			return e
		}), "empty") {
			t.Fatal("empty")
		}
		// Giving nothing is a request for a gift and reserves nothing, so it never reaches the board either.
		if !slices.Contains(offerKinds(t, func() error {
			_, e := game.CreateTradeOffer(w, "h1", game.TradeOfferInput{OriginID: a.ID, Want: game.Resources{Iron: 1}})
			return e
		}), "empty") {
			t.Fatal("empty (want only)")
		}
	})

	t.Run("create: the public board is capped per distinct offer, not per copy", func(t *testing.T) {
		w, a, _ := tradeWorld(t, 5)
		// Every post books its own merchants, copies included, so the market has to carry six deals plus ten copies
		// of one of them; the cap is what this test is about, not the merchant pool.
		a.Buildings[game.BMarket] = 20 // 110 merchants
		a.Resources = game.Resources{Wood: 1000, Clay: 1000, Iron: 1000}
		// h1 owns one village, so the cap is the per-village allotment of *different* deals.
		for i := 0; i < game.TradeBalance.PublicPerVillage; i++ {
			if _, err := game.CreateTradeOffer(w, "h1", game.TradeOfferInput{OriginID: a.ID, Give: game.Resources{Wood: 10 + float64(i)}, Want: game.Resources{Clay: 5}}); err != nil {
				t.Fatal(err)
			}
		}
		if !slices.Contains(offerKinds(t, func() error {
			_, e := game.CreateTradeOffer(w, "h1", game.TradeOfferInput{OriginID: a.ID, Give: game.Resources{Wood: 999}, Want: game.Resources{Clay: 5}})
			return e
		}), "boardFull") {
			t.Fatal("a new deal over the cap should be refused")
		}

		// Copies of a deal that is already out are free, however many: they are the same offer, split for several
		// takers. Ten copies of the first deal must all go up while the board is nominally full.
		for i := 0; i < 10; i++ {
			if _, err := game.CreateTradeOffer(w, "h1", game.TradeOfferInput{OriginID: a.ID, Give: game.Resources{Wood: 10}, Want: game.Resources{Clay: 5}}); err != nil {
				t.Fatalf("copy %d of a known deal was refused: %v", i, err)
			}
		}

		// Every copy reserves its share, so the village must hold the whole batch; posting more is refused rather
		// than driving the stock negative.
		w2, a2v, _ := tradeWorld(t, 5)
		a2v.Resources = game.Resources{Wood: 100}
		if !slices.Contains(offerKinds(t, func() error {
			_, e := game.CreateTradeOffer(w2, "h1", game.TradeOfferInput{OriginID: a2v.ID, Give: game.Resources{Wood: 500}, Want: game.Resources{Clay: 5}})
			return e
		}), "resources") {
			t.Fatal("posting more than the village holds should be refused")
		}
		if a2v.Resources.Wood < 0 {
			t.Fatalf("a refused post must not reserve anything, wood %v", a2v.Resources.Wood)
		}

		// A second village has its own allotment, but the first village's slots do not refill: the cap is per origin
		// village, because it is that village's merchants that deliver.
		a2 := gametest.AddStartVillage(w, "h1", a.X+7, a.Y+9)
		a2.Buildings[game.BMarket] = 5
		if _, err := game.CreateTradeOffer(w, "h1", game.TradeOfferInput{OriginID: a2.ID, Give: game.Resources{Wood: 5}, Want: game.Resources{Clay: 5}}); err != nil {
			t.Fatalf("a second village brings its own slots: %v", err)
		}
		if !slices.Contains(offerKinds(t, func() error {
			_, e := game.CreateTradeOffer(w, "h1", game.TradeOfferInput{OriginID: a.ID, Give: game.Resources{Wood: 777}, Want: game.Resources{Clay: 5}})
			return e
		}), "boardFull") {
			t.Fatal("owning a second village must not top up the first village's cap")
		}
	})

	t.Run("accept: settles both shipments and consumes the offer", func(t *testing.T) {
		w, a, b := tradeWorld(t, 5)
		o, err := game.CreateTradeOffer(w, "h1", game.TradeOfferInput{OriginID: a.ID, Give: game.Resources{Wood: 1000}, Want: game.Resources{Iron: 500}})
		if err != nil {
			t.Fatal(err)
		}
		if err := game.AcceptTradeOffer(w, "h2", o.ID, b.ID); err != nil {
			t.Fatal(err)
		}
		if a.Resources.Wood != 8000 || b.Resources.Iron != 8500 {
			t.Fatalf("a.wood %v b.iron %v", a.Resources.Wood, b.Resources.Iron)
		}
		var trades int
		for _, c := range w.Commands() {
			if c.Type == "trade" && !c.Returning {
				trades++
			}
		}
		if trades != 2 {
			t.Fatalf("trades %d", trades)
		}
		if w.TradeOffer(o.ID) != nil {
			t.Fatal("offer should be consumed")
		}
	})

	t.Run("accept: a post whose delivery limit cannot be met is blocked", func(t *testing.T) {
		w, a, _ := tradeWorld(t, 5)
		gametest.NewHumanPlayer(w, "h3")
		far := gametest.AddStartVillage(w, "h3", w.Config().MapSize-2, w.Config().MapSize-2)
		far.Buildings[game.BMarket], far.Buildings[game.BStorage] = 5, 15
		// Two fields away, the merchants need far longer than the hour the post allows.
		o, err := game.CreateTradeOffer(w, "h1", game.TradeOfferInput{OriginID: a.ID, Give: game.Resources{Wood: 100}, Want: game.Resources{Iron: 100}, MaxDeliveryMs: game.MsPerHour})
		if err != nil {
			t.Fatal(err)
		}
		if !slices.Contains(offerKinds(t, func() error {
			return game.AcceptTradeOffer(w, "h3", o.ID, far.ID)
		}), "outOfRange") {
			t.Fatal("outOfRange")
		}
	})

	t.Run("accept: a post without a delivery limit reaches a far village", func(t *testing.T) {
		w, a, _ := tradeWorld(t, 5)
		gametest.NewHumanPlayer(w, "h3")
		far := gametest.AddStartVillage(w, "h3", w.Config().MapSize-2, w.Config().MapSize-2)
		far.Buildings[game.BMarket], far.Buildings[game.BStorage] = 5, 15
		far.Resources = game.Resources{Wood: 9000, Clay: 9000, Iron: 9000}
		o, err := game.CreateTradeOffer(w, "h1", game.TradeOfferInput{OriginID: a.ID, Give: game.Resources{Wood: 100}, Want: game.Resources{Iron: 100}})
		if err != nil {
			t.Fatal(err)
		}
		if err := game.AcceptTradeOffer(w, "h3", o.ID, far.ID); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("cancel: only the poster may withdraw", func(t *testing.T) {
		w, a, _ := tradeWorld(t, 5)
		o, _ := game.CreateTradeOffer(w, "h1", game.TradeOfferInput{OriginID: a.ID, Give: game.Resources{Wood: 10}, Want: game.Resources{Iron: 10}})
		if err := game.CancelTradeOffer(w, "h2", o.ID); err == nil {
			t.Fatal("target cannot cancel")
		}
		if err := game.CancelTradeOffer(w, "h1", o.ID); err != nil {
			t.Fatal(err)
		}
		if w.TradeOffer(o.ID) != nil {
			t.Fatal("offer should be gone")
		}
	})

	t.Run("reservation: the give side is set aside, refunded on cancel and expiry", func(t *testing.T) {
		w, a, _ := tradeWorld(t, 5)

		o, err := game.CreateTradeOffer(w, "h1", game.TradeOfferInput{OriginID: a.ID, Give: game.Resources{Wood: 200}, Want: game.Resources{Iron: 50}})
		if err != nil {
			t.Fatal(err)
		}
		if a.Resources.Wood != 8800 {
			t.Fatalf("give side should be reserved, a.wood %v", a.Resources.Wood)
		}

		// A cancel hands the share back.
		if err := game.CancelTradeOffer(w, "h1", o.ID); err != nil {
			t.Fatal(err)
		}
		if a.Resources.Wood != 9000 {
			t.Fatalf("cancel should refund, a.wood %v", a.Resources.Wood)
		}

		// An expiry does the same; production adds to the village while time passes, so allow for it.
		o2, _ := game.CreateTradeOffer(w, "h1", game.TradeOfferInput{OriginID: a.ID, Give: game.Resources{Wood: 200}, Want: game.Resources{Iron: 50}})
		before := a.Resources.Wood
		advanceTouch(t, w, a, o2.ExpiresAt)
		if got := a.Resources.Wood - before; got < 180 { // the refund (200) plus whatever farms grew, never less
			t.Fatalf("expiry should refund, gained %v", got)
		}
	})

	t.Run("accept: the giver ships its reserved share without paying it again", func(t *testing.T) {
		w, a, b := tradeWorld(t, 5)
		o, err := game.CreateTradeOffer(w, "h1", game.TradeOfferInput{OriginID: a.ID, Give: game.Resources{Wood: 1000}, Want: game.Resources{Iron: 500}})
		if err != nil {
			t.Fatal(err)
		}
		if a.Resources.Wood != 8000 {
			t.Fatalf("give side should be reserved, a.wood %v", a.Resources.Wood)
		}
		if err := game.AcceptTradeOffer(w, "h2", o.ID, b.ID); err != nil {
			t.Fatal(err)
		}
		if a.Resources.Wood != 8000 || b.Resources.Iron != 8500 {
			t.Fatalf("a.wood %v b.iron %v", a.Resources.Wood, b.Resources.Iron)
		}
	})

	t.Run("create: blocked when trading is switched off in the world", func(t *testing.T) {
		w, a, _ := tradeWorld(t, 5)
		cfg := w.Config()
		cfg.TradingEnabled = false
		defer func() { cfg.TradingEnabled = true }()
		if !slices.Contains(offerKinds(t, func() error {
			_, e := game.CreateTradeOffer(w, "h1", game.TradeOfferInput{OriginID: a.ID, Give: game.Resources{Wood: 1}})
			return e
		}), "tradingOff") {
			t.Fatal("tradingOff")
		}
	})

	t.Run("trading off: every way in is refused, but withdrawing still works", func(t *testing.T) {
		w, a, b := tradeWorld(t, 5)
		cfg := w.Config()
		cfg.TradingEnabled = false
		// Post one offer while trading is still on, so there is something to withdraw afterwards.
		cfg.TradingEnabled = true
		o, err := game.CreateTradeOffer(w, "h1", game.TradeOfferInput{OriginID: a.ID, Give: game.Resources{Wood: 10}, Want: game.Resources{Iron: 10}})
		if err != nil {
			t.Fatalf("create: %v", err)
		}
		cfg.TradingEnabled = false
		defer func() { cfg.TradingEnabled = true }()

		// A transfer to another player is trading, so it is closed.
		if !slices.Contains(blockerKinds(game.CheckTrade(w, "h1", a.ID, b.ID, game.Resources{Wood: 5}).Blockers), "tradingOff") {
			t.Fatal("checkTrade should report tradingOff")
		}
		if _, err := game.SendTrade(w, "h1", a.ID, b.ID, game.Resources{Wood: 5}); err == nil {
			t.Fatal("SendTrade should refuse while trading is off")
		}
		// Moving resources between your own villages is not trading with anyone, so it keeps working: the market
		// still looks like it did before players could trade, and it still does what it did back then.
		mine := gametest.AddStartVillage(w, "h1", a.X+4, a.Y+1)
		mine.Buildings[game.BMarket] = 5
		if _, err := game.SendTrade(w, "h1", a.ID, mine.ID, game.Resources{Wood: 5}); err != nil {
			t.Fatalf("a transfer between my own villages should survive the switch: %v", err)
		}
		if err := game.AcceptTradeOffers(w, "h2", []string{o.ID}, b.ID); err == nil {
			t.Fatal("accepting an offer should refuse while trading is off")
		}
		if got := game.VisibleOffers(w, "h2"); len(got) != 0 {
			t.Fatalf("the market should be empty, got %d offers", len(got))
		}
		// Withdrawing is how a player gets their set-aside resources back after the host switches trading off.
		if err := game.CancelTradeOffer(w, "h1", o.ID); err != nil {
			t.Fatalf("withdraw must still work: %v", err)
		}
		if w.TradeOffer(o.ID) != nil {
			t.Fatal("the withdrawn offer should be gone")
		}
	})

	t.Run("expiry: an offer lapses when its time runs out", func(t *testing.T) {
		w, a, _ := tradeWorld(t, 5)
		o, _ := game.CreateTradeOffer(w, "h1", game.TradeOfferInput{OriginID: a.ID, Give: game.Resources{Wood: 10}, Want: game.Resources{Iron: 10}})
		advanceTouch(t, w, a, o.ExpiresAt)
		if w.TradeOffer(o.ID) != nil {
			t.Fatal("offer should have lapsed")
		}
		if got := game.VisibleOffers(w, "h2"); len(got) != 0 {
			t.Fatalf("lapsed offer still visible %+v", got)
		}
	})
}

// TestTradeOfferBatch covers taking several copies of the same board post in one deal.
func TestTradeOfferBatch(t *testing.T) {
	post := func(t *testing.T, w *gametest.MemWorld, a *game.Village, n int) []*game.TradeOffer {
		t.Helper()
		var out []*game.TradeOffer
		for i := 0; i < n; i++ {
			o, err := game.CreateTradeOffer(w, "h1", game.TradeOfferInput{OriginID: a.ID, Give: game.Resources{Wood: 1000}, Want: game.Resources{Iron: 500}})
			if err != nil {
				t.Fatal(err)
			}
			out = append(out, o)
		}
		return out
	}

	t.Run("accept: several copies settle as one haul each way", func(t *testing.T) {
		w, a, b := tradeWorld(t, 5)
		offers := post(t, w, a, 3)
		ids := []string{offers[0].ID, offers[1].ID, offers[2].ID}
		before := a.Resources.Wood
		if err := game.AcceptTradeOffers(w, "h2", ids, b.ID); err != nil {
			t.Fatal(err)
		}
		// h1 set aside 3×1000 wood when posting, so its stock did not move on accept.
		if a.Resources.Wood != before {
			t.Fatalf("giver's reserved share should not be paid twice, a.wood %v -> %v", before, a.Resources.Wood)
		}
		if b.Resources.Iron != 9000-1500 {
			t.Fatalf("taker should pay 3×500 iron, b.iron %v", b.Resources.Iron)
		}
		var hauls int
		for _, c := range w.Commands() {
			if c.Type == "trade" && !c.Returning {
				hauls++
			}
		}
		if hauls != 2 {
			t.Fatalf("the batch should book one haul each way, got %d", hauls)
		}
		for _, o := range offers {
			if w.TradeOffer(o.ID) != nil {
				t.Fatal("every accepted copy should be consumed")
			}
		}
	})

	t.Run("accept: the whole batch is refused when the taker cannot pay it", func(t *testing.T) {
		w, a, b := tradeWorld(t, 5)
		offers := post(t, w, a, 3)
		b.Resources.Iron = 800 // enough for one copy (500), not for three (1500)
		ids := []string{offers[0].ID, offers[1].ID, offers[2].ID}
		if !slices.Contains(offerKinds(t, func() error {
			return game.AcceptTradeOffers(w, "h2", ids, b.ID)
		}), "resources") {
			t.Fatal("resources")
		}
		// Nothing settles on a refused batch: no offer consumed and no stock moved.
		for _, o := range offers {
			if w.TradeOffer(o.ID) == nil {
				t.Fatal("a refused batch must not consume offers")
			}
		}
		if b.Resources.Iron != 800 {
			t.Fatalf("a refused batch must not charge the taker, b.iron %v", b.Resources.Iron)
		}
	})

	t.Run("accept: the batch needs free merchants for the whole haul", func(t *testing.T) {
		w, a, b := tradeWorld(t, 5)
		b.Buildings[game.BMarket] = 1 // one merchant = 1000 resources
		var ids []string
		for i := 0; i < 2; i++ { // 2×600 = 1200 iron owed, more than one merchant carries
			o, err := game.CreateTradeOffer(w, "h1", game.TradeOfferInput{OriginID: a.ID, Give: game.Resources{Wood: 1000}, Want: game.Resources{Iron: 600}})
			if err != nil {
				t.Fatal(err)
			}
			ids = append(ids, o.ID)
		}
		// Two copies need two merchants and the market only ever has one, so this is the permanent kind of refusal.
		if !slices.Contains(offerKinds(t, func() error {
			return game.AcceptTradeOffers(w, "h2", ids, b.ID)
		}), "marketTooSmall") {
			t.Fatal("marketTooSmall")
		}
		// One copy fits in the single merchant, so that still settles.
		if err := game.AcceptTradeOffers(w, "h2", ids[:1], b.ID); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("accept: a haul bigger than the whole market says so, instead of blaming busy merchants", func(t *testing.T) {
		w, a, b := tradeWorld(t, 5)
		a.Buildings[game.BMarket], b.Buildings[game.BMarket] = 5, 1 // the taker's market ships one merchant
		o, err := game.CreateTradeOffer(w, "h1", game.TradeOfferInput{OriginID: a.ID, Give: game.Resources{Wood: 1000}, Want: game.Resources{Iron: 1500}})
		if err != nil {
			t.Fatal(err)
		}
		// 1500 resources is two merchants; the market will never have more than one at level 1.
		if !slices.Contains(offerKinds(t, func() error {
			return game.AcceptTradeOffer(w, "h2", o.ID, b.ID)
		}), "marketTooSmall") {
			t.Fatal("marketTooSmall")
		}
	})

	t.Run("accept: busy merchants are reported as busy, not as a market that is too small", func(t *testing.T) {
		w, a, b := tradeWorld(t, 5)
		b.Buildings[game.BMarket], b.Buildings[game.BStorage] = 3, 15
		o, err := game.CreateTradeOffer(w, "h1", game.TradeOfferInput{OriginID: a.ID, Give: game.Resources{Wood: 1000}, Want: game.Resources{Iron: 2500}})
		if err != nil {
			t.Fatal(err)
		}
		// Three merchants at level 3, one already on the road: only two are free but three are needed.
		game.SendTrade(w, "h2", b.ID, a.ID, game.Resources{Wood: 400})
		if !slices.Contains(offerKinds(t, func() error {
			return game.AcceptTradeOffer(w, "h2", o.ID, b.ID)
		}), "merchants") {
			t.Fatal("merchants")
		}
	})

	t.Run("accept: mixed sellers or duplicates are refused", func(t *testing.T) {
		w, a, b := tradeWorld(t, 5)
		o, _ := game.CreateTradeOffer(w, "h1", game.TradeOfferInput{OriginID: a.ID, Give: game.Resources{Wood: 100}, Want: game.Resources{Iron: 100}})
		if err := game.AcceptTradeOffers(w, "h2", []string{o.ID, o.ID}, b.ID); err == nil {
			t.Fatal("the same offer twice should be refused")
		}
		if err := game.AcceptTradeOffers(w, "h2", []string{o.ID, "nope"}, b.ID); err == nil {
			t.Fatal("an unknown offer should be refused")
		}
		if w.TradeOffer(o.ID) == nil {
			t.Fatal("a refused batch must not consume the offer")
		}
	})
}

// TestAcceptedOfferCannotBeRecalled pins the settlement rule: both legs of an accepted offer are Settled, so neither
// side may turn its half back. Without it a taker could empty a bot's public post, cancel its own payment leg, and
// keep both the goods and the payment every cycle.
func TestAcceptedOfferCannotBeRecalled(t *testing.T) {
	w, a, b := tradeWorld(t, 5)
	o, err := game.CreateTradeOffer(w, "h1", game.TradeOfferInput{OriginID: a.ID, Give: game.Resources{Wood: 1000}, Want: game.Resources{Iron: 500}})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	game.TouchVillages(w, "h2")
	if err := game.AcceptTradeOffers(w, "h2", []string{o.ID}, b.ID); err != nil {
		t.Fatalf("accept: %v", err)
	}

	// Two shipment legs are on the road, one from each side, and both are settled.
	legs := append(w.CommandsHome(a.ID), w.CommandsHome(b.ID)...)
	var trades []*game.Command
	for _, c := range legs {
		if c.Type == "trade" {
			trades = append(trades, c)
		}
	}
	if len(trades) != 2 {
		t.Fatalf("want 2 settlement legs, got %d", len(trades))
	}
	for _, c := range trades {
		if !c.Settled {
			t.Fatalf("leg %s is not marked settled", c.ID)
		}
		if game.CanCancelCommand(w, c) {
			t.Fatalf("leg %s: CanCancelCommand should be false", c.ID)
		}
		// Whoever ships it is the one who could recall it, so try their side.
		owner := game.PlayerID("h1")
		if c.OriginID == b.ID {
			owner = "h2"
		}
		if err := game.CancelCommand(w, owner, c.ID); err == nil {
			t.Fatalf("leg %s: cancel should be refused", c.ID)
		}
	}
}

// TestClosingTheMarketCapsTheRefundAtStorage is the rule the admin switch promises: every offer is cleared and its
// resources go home, but never above the storehouse. Whatever does not fit is lost, the same way a full storehouse
// wastes production, rather than pushing the village over capacity where the surplus would sit outside the economy.
func TestClosingTheMarketCapsTheRefundAtStorage(t *testing.T) {
	t.Run("a village holding loot above its storehouse", func(t *testing.T) {
		w, a, _ := tradeWorld(t, 5)
		a.Buildings[game.BStorage] = 1
		capacity := game.VillageStorageCapacity(a)
		// Loot can leave a village above capacity, which is where a refund can overflow.
		a.Resources = game.Resources{Wood: capacity + 400, Clay: 0, Iron: 0}
		if _, err := game.CreateTradeOffer(w, "h1", game.TradeOfferInput{OriginID: a.ID, Give: game.Resources{Wood: 500}, Want: game.Resources{Iron: 1}}); err != nil {
			t.Fatalf("post: %v", err)
		}
		game.CloseMarket(w)
		if a.Resources.Wood != capacity {
			t.Fatalf("want the village filled to capacity %v, got %v", capacity, a.Resources.Wood)
		}
	})
	t.Run("a storehouse that shrank while the offer was up", func(t *testing.T) {
		w, a, _ := tradeWorld(t, 5)
		a.Buildings[game.BStorage] = 20
		big := game.VillageStorageCapacity(a)
		a.Resources = game.Resources{Wood: 1000, Clay: 0, Iron: 0}
		if _, err := game.CreateTradeOffer(w, "h1", game.TradeOfferInput{OriginID: a.ID, Give: game.Resources{Wood: 600}, Want: game.Resources{Iron: 1}}); err != nil {
			t.Fatalf("post: %v", err)
		}
		// Demolished down to nothing: the share can no longer fit.
		a.Buildings[game.BStorage] = 0
		small := game.VillageStorageCapacity(a)
		if small >= big {
			t.Fatalf("the storehouse should have shrunk: %v -> %v", big, small)
		}
		game.CloseMarket(w)
		if a.Resources.Wood != small {
			t.Fatalf("want the village capped at %v, got %v", small, a.Resources.Wood)
		}
		if a.Resources.Wood > small {
			t.Fatal("a refund must never push a village past its storehouse")
		}
	})
}

// TestCloseMarketGivesTheShareBackToItsOwnVillage keeps the refunds apart: each offer returns to the village that
// posted it, not to whichever village of the player happens to be touched.
func TestCloseMarketGivesTheShareBackToItsOwnVillage(t *testing.T) {
	w, a, b := tradeWorld(t, 5)
	a.Buildings[game.BStorage], b.Buildings[game.BStorage] = 20, 20
	a.Resources = game.Resources{Wood: 1000, Clay: 1000, Iron: 1000}
	b.Resources = game.Resources{Wood: 1000, Clay: 1000, Iron: 1000}
	for _, c := range []struct {
		owner string
		v     *game.Village
	}{{"h1", a}, {"h2", b}} {
		game.TouchVillages(w, c.owner)
		if _, err := game.CreateTradeOffer(w, c.owner, game.TradeOfferInput{OriginID: c.v.ID, Give: game.Resources{Wood: 300}, Want: game.Resources{Iron: 300}}); err != nil {
			t.Fatalf("%s post: %v", c.owner, err)
		}
	}
	game.CloseMarket(w)
	if a.Resources.Wood != 1000 || b.Resources.Wood != 1000 {
		t.Fatalf("each village should be whole again: a=%v b=%v", a.Resources.Wood, b.Resources.Wood)
	}
}

// TestAPlainMarketTradeIsStillRecallable guards the other side of the rule: only settlement legs are frozen, an
// ordinary trade keeps the cancel window players rely on.
func TestAPlainMarketTradeIsStillRecallable(t *testing.T) {
	w, a, b := tradeWorld(t, 5)
	c, err := game.SendTrade(w, "h1", a.ID, b.ID, game.Resources{Wood: 100})
	if err != nil {
		t.Fatalf("send: %v", err)
	}
	if c.Settled {
		t.Fatal("a plain market trade must not be marked settled")
	}
	if !game.CanCancelCommand(w, c) {
		t.Fatal("a plain market trade should still be recallable")
	}
	if err := game.CancelCommand(w, "h1", c.ID); err != nil {
		t.Fatalf("cancel: %v", err)
	}
}

// TestOneSidedOffersAreRefused closes the free route past the board cap: an offer with nothing on its give side
// reserves no resources, so before this check a player could post unlimited copies of a "give nothing, want 1000"
// deal while the cap saw every copy as a known post.
func TestOneSidedOffersAreRefused(t *testing.T) {
	w, a, _ := tradeWorld(t, 5)
	for _, in := range []game.TradeOfferInput{
		{OriginID: a.ID, Want: game.Resources{Wood: 1000}}, // gift request
		{OriginID: a.ID, Give: game.Resources{Wood: 1000}}, // free hand-out
	} {
		if !slices.Contains(offerKinds(t, func() error {
			_, e := game.CreateTradeOffer(w, "h1", in)
			return e
		}), "empty") {
			t.Fatalf("a one-sided offer should be refused as empty: %+v", in)
		}
	}
}

// TestDeliveryVariantsStillCountAgainstTheCap pins the shared deal identity: the delivery limit is part of a deal, so
// both the "copy of something already out" test and the cap count must use it, or a variant would be cap-free.
func TestDeliveryVariantsStillCountAgainstTheCap(t *testing.T) {
	w, a, _ := tradeWorld(t, 5)
	// One merchant per distinct post is booked on creation, so the market has to be able to carry the whole board.
	a.Buildings[game.BMarket] = game.TradeBalance.PublicPerVillage
	base := game.TradeOfferInput{OriginID: a.ID, Give: game.Resources{Wood: 100}, Want: game.Resources{Iron: 50}}
	// One variant per slot the village has: each is a distinct deal, so each fills one.
	for i := 0; i < game.TradeBalance.PublicPerVillage; i++ {
		in := base
		in.MaxDeliveryMs = int64(2+i) * 3_600_000
		if _, err := game.CreateTradeOffer(w, "h1", in); err != nil {
			t.Fatalf("post %d: %v", i, err)
		}
	}
	// The slots are full, so one more variant is refused rather than slipping past the cap as "a copy".
	in := base
	in.MaxDeliveryMs = int64(2+game.TradeBalance.PublicPerVillage) * 3_600_000
	if !slices.Contains(offerKinds(t, func() error {
		_, e := game.CreateTradeOffer(w, "h1", in)
		return e
	}), "boardFull") {
		t.Fatal("a variant past the cap should be refused")
	}
}

// TestClosingTheMarketRefundsEveryOpenOffer is what makes switching trading off safe on a live world: the panels are
// hidden while it is off, so any offer left on the board would hold its reserved resources out of reach forever.
func TestClosingTheMarketRefundsEveryOpenOffer(t *testing.T) {
	w, a, b := tradeWorld(t, 5)
	start := a.Resources.Wood
	for i := 0; i < 3; i++ {
		if _, err := game.CreateTradeOffer(w, "h1", game.TradeOfferInput{OriginID: a.ID, Give: game.Resources{Wood: 100}, Want: game.Resources{Iron: 50}}); err != nil {
			t.Fatalf("post %d: %v", i, err)
		}
	}
	if a.Resources.Wood != start-300 {
		t.Fatalf("the posts should have reserved 300 wood, got %v off", start-a.Resources.Wood)
	}
	// A second village keeps its own offer, so the sweep is not accidentally single-village.
	game.TouchVillages(w, "h2")
	if _, err := game.CreateTradeOffer(w, "h2", game.TradeOfferInput{OriginID: b.ID, Give: game.Resources{Clay: 40}, Want: game.Resources{Wood: 20}}); err != nil {
		t.Fatalf("h2 post: %v", err)
	}
	if n := game.CloseMarket(w); n != 4 {
		t.Fatalf("want 4 offers closed, got %d", n)
	}
	if n := len(w.AllTradeOffers()); n != 0 {
		t.Fatalf("the board should be empty, got %d", n)
	}
	// Every share came home.
	if a.Resources.Wood != start {
		t.Fatalf("h1 wood: want %v, got %v", start, a.Resources.Wood)
	}
	// The expiry timers are gone too, so nothing fires later.
	if err := gametest.Advance(w, w.Now()+2*24*int64(game.MsPerHour)); err != nil {
		t.Fatalf("advance: %v", err)
	}
	if n := len(w.AllTradeOffers()); n != 0 {
		t.Fatalf("a stale timer resurrected offers: %d", n)
	}
}

// TestPublicOfferVisibility covers a public post the viewer did not write: it shows up for a player whose villages
// the merchants could actually reach, and stays off the board of a player they could not.
func TestPublicOfferVisibility(t *testing.T) {
	t.Run("a public post is on the board for a player it can deliver to", func(t *testing.T) {
		w, a, _ := tradeWorld(t, 5)
		o, err := game.CreateTradeOffer(w, "h1", game.TradeOfferInput{OriginID: a.ID, Give: game.Resources{Wood: 100}, Want: game.Resources{Iron: 100}})
		if err != nil {
			t.Fatal(err)
		}
		got := game.VisibleOffers(w, "h2")
		if len(got) != 1 || got[0].ID != o.ID {
			t.Fatalf("h2 should see the public post, got %+v", got)
		}
		// The poster sees it as their own post.
		if got := game.VisibleOffers(w, "h1"); len(got) != 1 || got[0].ID != o.ID {
			t.Fatalf("h1 should see their own post, got %+v", got)
		}
	})

	t.Run("a public post out of a player's reach is not on their board", func(t *testing.T) {
		w, a, _ := tradeWorld(t, 5)
		// A limit the poster's village cannot meet for the other player: the merchants could not arrive in time.
		o, err := game.CreateTradeOffer(w, "h1", game.TradeOfferInput{OriginID: a.ID, MaxDeliveryMs: game.TradeBalance.MinDeliveryMs,
			Give: game.Resources{Wood: 100}, Want: game.Resources{Iron: 100}})
		if err != nil {
			t.Fatal(err)
		}
		// Move the viewer far outside the posted window.
		b := game.PlayerVillages(w, "h2")[0]
		b.X, b.Y = a.X+60, a.Y+60
		if got := game.VisibleOffers(w, "h2"); len(got) != 0 {
			t.Fatalf("an unreachable post should be hidden, got %+v", got)
		}
		if w.TradeOffer(o.ID) == nil {
			t.Fatal("hiding it must not remove it from the board")
		}
	})

	t.Run("a player with no villages sees no public posts", func(t *testing.T) {
		w, a, _ := tradeWorld(t, 5)
		if _, err := game.CreateTradeOffer(w, "h1", game.TradeOfferInput{OriginID: a.ID, Give: game.Resources{Wood: 100}, Want: game.Resources{Iron: 100}}); err != nil {
			t.Fatal(err)
		}
		if got := game.VisibleOffers(w, "nobody"); len(got) != 0 {
			t.Fatalf("an unknown player should see nothing, got %+v", got)
		}
	})
}

// TestRefundAfterVillageLost covers the set-aside share of an offer whose village was conquered while the post was up:
// it goes to another village the player still holds, and lapses when they hold none.
func TestRefundAfterVillageLost(t *testing.T) {
	t.Run("it comes home to another village the player still holds", func(t *testing.T) {
		w, a, _ := tradeWorld(t, 5)
		// A second village of the same player, and a little room to receive the share.
		home := gametest.AddStartVillage(w, "h1", a.X+6, a.Y+6)
		home.Buildings[game.BStorage] = 15
		home.Resources = game.Resources{Wood: 1000, Clay: 1000, Iron: 1000}
		before := a.Resources.Wood
		o, err := game.CreateTradeOffer(w, "h1", game.TradeOfferInput{OriginID: a.ID, Give: game.Resources{Wood: 500}, Want: game.Resources{Iron: 500}})
		if err != nil {
			t.Fatal(err)
		}
		if a.Resources.Wood != before-500 {
			t.Fatalf("the give side should be set aside, want %v got %v", before-500, a.Resources.Wood)
		}
		// The posting village is conquered while the offer is up.
		a.OwnerID = game.Ref("h2")
		was := home.Resources.Wood
		if err := game.CancelTradeOffer(w, "h1", o.ID); err != nil {
			t.Fatal(err)
		}
		if home.Resources.Wood <= was {
			t.Fatalf("the set-aside share should reach the player's remaining village, %v -> %v", was, home.Resources.Wood)
		}
		// It is capped by storage like any other arrival.
		if home.Resources.Wood > game.VillageStorageCapacity(home)+1 {
			t.Fatalf("the refund should respect storage: %v", home.Resources.Wood)
		}
	})

	t.Run("it lapses when the player holds no village at all", func(t *testing.T) {
		w, a, b := tradeWorld(t, 5)
		o, err := game.CreateTradeOffer(w, "h1", game.TradeOfferInput{OriginID: a.ID, Give: game.Resources{Wood: 500}, Want: game.Resources{Iron: 500}})
		if err != nil {
			t.Fatal(err)
		}
		a.OwnerID = game.Ref("h2")
		b.OwnerID = game.Ref("h2")
		if err := game.CancelTradeOffer(w, "h1", o.ID); err != nil {
			t.Fatalf("cancelling must not fail: %v", err)
		}
		if w.TradeOffer(o.ID) != nil {
			t.Fatal("the offer should be gone")
		}
	})
}

// TestSettledDealReports covers the "market" report each side of a settled deal receives: one report per deal, naming
// the other player, the village the goods come from and the travel time, with what each side gave and got.
func TestSettledDealReports(t *testing.T) {
	mk := func(t *testing.T) (*gametest.MemWorld, *game.Village, *game.Village) {
		w, a, b := tradeWorld(t, 5)
		// b belongs to h2; a to h1. Make both villagers market-holders with room to spare.
		a.Resources = game.Resources{Wood: 5000, Clay: 5000, Iron: 5000}
		b.Resources = game.Resources{Wood: 5000, Clay: 5000, Iron: 5000}
		return w, a, b
	}

	t.Run("the poster is told what was taken, by whom, and where it ships from", func(t *testing.T) {
		w, a, b := mk(t)
		o, err := game.CreateTradeOffer(w, "h1", game.TradeOfferInput{OriginID: a.ID, Give: game.Resources{Wood: 300}, Want: game.Resources{Iron: 100}})
		if err != nil {
			t.Fatal(err)
		}
		if err := game.AcceptTradeOffer(w, "h2", o.ID, b.ID); err != nil {
			t.Fatal(err)
		}
		reps := w.Reports("h1", w.Now()+1, 0)
		if len(reps) != 1 {
			t.Fatalf("the poster should get one report, got %d", len(reps))
		}
		r := reps[0]
		if r.Type != "market" || r.Market == nil {
			t.Fatalf("want a market report, got %q %+v", r.Type, r.Market)
		}
		m := r.Market
		if m.CounterpartyID != "h2" {
			t.Fatalf("counterparty should be the taker, got %q", m.CounterpartyID)
		}
		// The poster receives the taker's payment, shipped from the taker's village.
		if m.FromVillage.ID != b.ID {
			t.Fatalf("the incoming resources come from the taker's village, got %q want %q", m.FromVillage.ID, b.ID)
		}
		if m.Want.Iron != 100 || m.Give.Wood != 300 {
			t.Fatalf("give/want should be the reader's sides: %+v", m)
		}
		if m.Offers != 1 {
			t.Fatalf("one offer settled, got %d", m.Offers)
		}
		if m.TravelMs <= 0 {
			t.Fatalf("travel time should be known, got %d", m.TravelMs)
		}
	})

	t.Run("the taker is told the goods come from the poster's village, with its travel time", func(t *testing.T) {
		w, a, b := mk(t)
		o, err := game.CreateTradeOffer(w, "h1", game.TradeOfferInput{OriginID: a.ID, Give: game.Resources{Wood: 300}, Want: game.Resources{Iron: 100}})
		if err != nil {
			t.Fatal(err)
		}
		if err := game.AcceptTradeOffer(w, "h2", o.ID, b.ID); err != nil {
			t.Fatal(err)
		}
		reps := w.Reports("h2", w.Now()+1, 0)
		if len(reps) != 1 || reps[0].Market == nil {
			t.Fatalf("the taker should get one market report, got %+v", reps)
		}
		m := reps[0].Market
		if m.CounterpartyID != "h1" {
			t.Fatalf("counterparty should be the poster, got %q", m.CounterpartyID)
		}
		if m.FromVillage.ID != a.ID {
			t.Fatalf("the goods come from the poster's village, got %q want %q", m.FromVillage.ID, a.ID)
		}
		if m.Want.Wood != 300 || m.Give.Iron != 100 {
			t.Fatalf("the taker gives iron and receives wood: %+v", m)
		}
		wantMs := game.TradeTravelMs(a, b, w.Config().UnitSpeed)
		if m.TravelMs != wantMs {
			t.Fatalf("travel time %d, want %d", m.TravelMs, wantMs)
		}
	})

	t.Run("ten offers settled at once make one report, totalled", func(t *testing.T) {
		w, a, b := mk(t)
		// Ten copies each book their own merchant on posting, so the market has to hold all ten of them.
		a.Buildings[game.BMarket] = 10
		var ids []string
		for i := 0; i < 10; i++ {
			o, err := game.CreateTradeOffer(w, "h1", game.TradeOfferInput{OriginID: a.ID, Give: game.Resources{Wood: 100}, Want: game.Resources{Iron: 100}})
			if err != nil {
				t.Fatal(err)
			}
			ids = append(ids, o.ID)
		}
		if err := game.AcceptTradeOffers(w, "h2", ids, b.ID); err != nil {
			t.Fatal(err)
		}
		for _, who := range []game.PlayerID{"h1", "h2"} {
			reps := w.Reports(who, w.Now()+1, 0)
			market := 0
			for _, r := range reps {
				if r.Type == "market" {
					market++
				}
			}
			if market != 1 {
				t.Fatalf("%s: ten offers in one deal should make one report, got %d", who, market)
			}
			var m *game.MarketReport
			for _, r := range reps {
				if r.Type == "market" {
					m = r.Market
				}
			}
			if m.Offers != 10 {
				t.Fatalf("%s: the report should say ten offers, got %d", who, m.Offers)
			}
			// The reader's own sides, totalled across the ten offers: h1 gives wood and receives iron, h2 the reverse.
			giveWood, receiveIron := m.Give.Wood, m.Want.Iron
			if who == "h2" {
				giveWood, receiveIron = m.Give.Iron, m.Want.Wood
			}
			if giveWood != 1000 || receiveIron != 1000 {
				t.Fatalf("%s: totals should be summed: give %+v want %+v", who, m.Give, m.Want)
			}
		}
	})

	t.Run("a bot accepting a player's post still reports to the player", func(t *testing.T) {
		w, a, _ := tradeWorld(t, 5)
		gametest.NewBotPlayer(w, "bot_0")
		bot := gametest.AddStartVillage(w, "bot_0", a.X+2, a.Y+2)
		bot.Buildings[game.BMarket] = 5
		bot.Resources = game.Resources{Wood: 500, Clay: 5000, Iron: 9000}
		a.Resources = game.Resources{Wood: 9000, Clay: 5000, Iron: 500}
		o, err := game.CreateTradeOffer(w, "h1", game.TradeOfferInput{OriginID: a.ID, Give: game.Resources{Wood: 500}, Want: game.Resources{Iron: 500}})
		if err != nil {
			t.Fatal(err)
		}
		if err := game.AcceptTradeOffer(w, "bot_0", o.ID, bot.ID); err != nil {
			t.Fatal(err)
		}
		reps := w.Reports("h1", w.Now()+1, 0)
		if len(reps) != 1 || reps[0].Market == nil {
			t.Fatalf("the human should get a market report, got %+v", reps)
		}
		if !reps[0].Market.CounterpartyBot {
			t.Fatalf("the report should say a bot took it: %+v", reps[0].Market)
		}
	})
}

// A post books the merchants that will carry it, so an offer on the board can always be accepted: the taker must
// never be refused for the seller's busy merchants, which they can neither see nor fix.
func TestPostingAnOfferBooksItsMerchants(t *testing.T) {
	t.Run("posting needs free merchants, and refuses with numbers when it has none", func(t *testing.T) {
		w, a, b := tradeWorld(t, 9)
		a.Buildings[game.BMarket] = 2 // two merchants
		// Spend them on a shipment so none are free.
		if _, err := game.SendTrade(w, "h1", a.ID, b.ID, game.Resources{Wood: 2000}); err != nil {
			t.Fatal(err)
		}
		if got := game.MerchantsAvailable(w, a); got != 0 {
			t.Fatalf("%d merchants free, want 0", got)
		}
		be, ok := func() error {
			_, e := game.CreateTradeOffer(w, "h1", game.TradeOfferInput{OriginID: a.ID,
				Give: game.Resources{Wood: 1000}, Want: game.Resources{Clay: 1000}})
			return e
		}().(*game.BlockedError)
		if !ok {
			t.Fatal("an offer with no free merchants went up")
		}
		b0 := be.Blockers[0]
		if b0["kind"] != "merchants" || b0["needed"] != 1 || b0["available"] != 0 {
			t.Fatalf("blocker %+v, want merchants 1 of 0", b0)
		}
	})

	t.Run("a booked offer takes its merchants out of the free pool", func(t *testing.T) {
		w, a, _ := tradeWorld(t, 9)
		a.Buildings[game.BMarket] = 4
		before := game.MerchantsAvailable(w, a)
		if _, err := game.CreateTradeOffer(w, "h1", game.TradeOfferInput{OriginID: a.ID,
			Give: game.Resources{Wood: 2000}, Want: game.Resources{Clay: 2000}}); err != nil {
			t.Fatal(err)
		}
		if after := game.MerchantsAvailable(w, a); after != before-2 {
			t.Fatalf("%d free after posting 2000 (2 merchants), was %d", after, before)
		}
	})

	t.Run("a copy of a deal books its own merchants", func(t *testing.T) {
		// Each copy a taker picks up is its own haul, so a shared booking would promise more than the market carries.
		w, a, _ := tradeWorld(t, 9)
		a.Buildings[game.BMarket] = 4
		for i := 0; i < 2; i++ {
			if _, err := game.CreateTradeOffer(w, "h1", game.TradeOfferInput{OriginID: a.ID,
				Give: game.Resources{Wood: 2000}, Want: game.Resources{Clay: 2000}}); err != nil {
				t.Fatalf("copy %d: %v", i, err)
			}
		}
		if got := game.MerchantsAvailable(w, a); got != 0 {
			t.Fatalf("%d merchants free after two copies of a 2-merchant deal, want 0", got)
		}
		// A third copy would need a merchant the market does not have, and says so with the numbers.
		be, ok := func() error {
			_, e := game.CreateTradeOffer(w, "h1", game.TradeOfferInput{OriginID: a.ID,
				Give: game.Resources{Wood: 2000}, Want: game.Resources{Clay: 2000}})
			return e
		}().(*game.BlockedError)
		if !ok {
			t.Fatal("a third copy went up with no merchants left")
		}
		if b0 := be.Blockers[0]; b0["kind"] != "merchants" || b0["needed"] != 2 || b0["available"] != 0 {
			t.Fatalf("blocker %+v, want merchants 2 of 0", b0)
		}
	})

	t.Run("withdrawing an offer hands its merchants back", func(t *testing.T) {
		w, a, _ := tradeWorld(t, 9)
		a.Buildings[game.BMarket] = 4
		before := game.MerchantsAvailable(w, a)
		o, err := game.CreateTradeOffer(w, "h1", game.TradeOfferInput{OriginID: a.ID,
			Give: game.Resources{Wood: 2000}, Want: game.Resources{Clay: 2000}})
		if err != nil {
			t.Fatal(err)
		}
		if err := game.CancelTradeOffer(w, "h1", o.ID); err != nil {
			t.Fatal(err)
		}
		if after := game.MerchantsAvailable(w, a); after != before {
			t.Fatalf("%d free after withdrawing, want %d", after, before)
		}
	})

	t.Run("accepting works even when the seller spent its other merchants", func(t *testing.T) {
		w, a, b := tradeWorld(t, 9)
		a.Buildings[game.BMarket], b.Buildings[game.BMarket] = 4, 4
		o, err := game.CreateTradeOffer(w, "h1", game.TradeOfferInput{OriginID: a.ID,
			Give: game.Resources{Clay: 2000}, Want: game.Resources{Iron: 4000}})
		if err != nil {
			t.Fatal(err)
		}
		// The seller now spends what little is left, so its *free* pool is empty but the offer is still booked.
		if _, err := game.SendTrade(w, "h1", a.ID, b.ID, game.Resources{Wood: 2000}); err != nil {
			t.Fatal(err)
		}
		if err := game.AcceptTradeOffer(w, "h2", o.ID, b.ID); err != nil {
			t.Fatalf("accept was refused: %v", err)
		}
	})

	t.Run("an offer posted before merchants were booked still reserves its share", func(t *testing.T) {
		w, a, b := tradeWorld(t, 9)
		a.Buildings[game.BMarket], b.Buildings[game.BMarket] = 4, 4
		// Simulate a legacy row: no Reserved count, as an existing world has.
		o, err := game.CreateTradeOffer(w, "h1", game.TradeOfferInput{OriginID: a.ID,
			Give: game.Resources{Clay: 2000}, Want: game.Resources{Iron: 4000}})
		if err != nil {
			t.Fatal(err)
		}
		o.Reserved = 0
		if got, want := game.ReservedMerchants(o), 2; got != want {
			t.Fatalf("a legacy offer reserves %d merchants, want %d from its give side", got, want)
		}
		if err := game.AcceptTradeOffer(w, "h2", o.ID, b.ID); err != nil {
			t.Fatalf("a legacy offer could not be accepted: %v", err)
		}
	})
}
