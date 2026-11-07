package game_test

import (
	"testing"

	"librehold/internal/game"
	"librehold/internal/game/gametest"
)

// botTradeWorld: a bot (bot_0) with one village that has a market, next to a human village.
func botTradeWorld(t *testing.T, seed uint32) (*gametest.MemWorld, *game.Village, *game.Village) {
	t.Helper()
	failOnBotErrors(t)
	cfg := game.DefaultWorldConfig
	cfg.BotCount, cfg.BeginnerProtectionDays = 1, 0
	w, human := gametest.NewWorldWithHuman(seed, cfg)
	w.Brain("bot_0").Skill = 1
	bot := game.PlayerVillages(w, "bot_0")[0]
	for _, v := range []*game.Village{bot, human} {
		v.Buildings[game.BMarket], v.Buildings[game.BStorage] = 5, 15
		game.RecomputePoints(v)
	}
	human.Resources = game.Resources{Wood: 5000, Clay: 5000, Iron: 5000}
	return w, bot, human
}

func TestBotTrading(t *testing.T) {
	t.Run("a bot short of a resource posts an offer for it", func(t *testing.T) {
		w, bot, _ := botTradeWorld(t, 5)
		game.TouchVillages(w, "bot_0")
		// No iron, plenty of wood: the bot wants iron and gives wood.
		bot.Resources = game.Resources{Wood: 9000, Clay: 4000, Iron: 20}
		game.PlanTrading(w, "bot_0", bot)

		offers := w.AllTradeOffers()
		if len(offers) != 1 {
			t.Fatalf("want one post, got %d", len(offers))
		}
		o := offers[0]
		if o.FromID != "bot_0" || o.OriginID != bot.ID {
			t.Fatalf("post should be the bot's own: %+v", o)
		}
		if o.Want.Iron <= 0 {
			t.Fatalf("the post should ask for the missing iron, want %+v", o.Want)
		}
		if o.Give.Wood <= 0 || o.Give.Clay > 0 {
			t.Fatalf("the post should give the spare wood only, give %+v", o.Give)
		}
	})

	t.Run("the give side is set aside from the village", func(t *testing.T) {
		w, bot, _ := botTradeWorld(t, 5)
		game.TouchVillages(w, "bot_0")
		bot.Resources = game.Resources{Wood: 9000, Clay: 4000, Iron: 20}
		before := bot.Resources.Wood
		game.PlanTrading(w, "bot_0", bot)
		offers := w.AllTradeOffers()
		if len(offers) != 1 {
			t.Fatalf("want one post, got %d", len(offers))
		}
		if bot.Resources.Wood != before-offers[0].Give.Wood {
			t.Fatalf("wood should be reserved: %v -> %v, gave %v", before, bot.Resources.Wood, offers[0].Give.Wood)
		}
	})

	t.Run("a balanced bot posts nothing", func(t *testing.T) {
		w, bot, _ := botTradeWorld(t, 5)
		game.TouchVillages(w, "bot_0")
		bot.Resources = game.Resources{Wood: 5000, Clay: 5000, Iron: 5000}
		game.PlanTrading(w, "bot_0", bot)
		if n := len(w.AllTradeOffers()); n != 0 {
			t.Fatalf("want no post, got %d", n)
		}
	})

	t.Run("a village without a market does not trade", func(t *testing.T) {
		w, bot, _ := botTradeWorld(t, 5)
		game.TouchVillages(w, "bot_0")
		bot.Buildings[game.BMarket] = 0
		bot.Resources = game.Resources{Wood: 9000, Clay: 4000, Iron: 20}
		game.PlanTrading(w, "bot_0", bot)
		if n := len(w.AllTradeOffers()); n != 0 {
			t.Fatalf("want no post, got %d", n)
		}
	})

	t.Run("a bot takes a board post it can pay for and is short of", func(t *testing.T) {
		w, bot, human := botTradeWorld(t, 5)
		game.TouchVillages(w, "bot_0")
		game.TouchVillages(w, "h1")
		bot.Resources = game.Resources{Wood: 4000, Clay: 4000, Iron: 30}
		human.Resources = game.Resources{Wood: 100, Clay: 100, Iron: 9000}
		// The human offers iron (which the bot lacks) and asks for wood (which it has to spare).
		o, err := game.CreateTradeOffer(w, "h1", game.TradeOfferInput{OriginID: human.ID, Give: game.Resources{Iron: 500}, Want: game.Resources{Wood: 500}})
		if err != nil {
			t.Fatal(err)
		}
		game.PlanTrading(w, "bot_0", bot)

		if w.TradeOffer(o.ID) != nil {
			t.Fatal("the bot should have taken the post")
		}
		if bot.Resources.Wood >= 4000 {
			t.Fatalf("the bot should have paid wood, have %v", bot.Resources.Wood)
		}
	})

	// The board is a map, so its own order varies between runs. When several posts are equally good the bot has to
	// take the same one every time: replaying an action log must reach the state the live world reached, and a
	// planner that follows map order does not.
	t.Run("picks the same post every run when several are equally good", func(t *testing.T) {
		taken := func() string {
			w, bot, human := botTradeWorld(t, 5)
			game.TouchVillages(w, "bot_0")
			game.TouchVillages(w, "h1")
			bot.Resources = game.Resources{Wood: 4000, Clay: 4000, Iron: 30}
			human.Resources = game.Resources{Wood: 100, Clay: 100, Iron: 9000}
			// Three villages of one player, each posting the same deal: identical value, different ids.
			for i := 0; i < 3; i++ {
				if i > 0 {
					next := gametest.AddStartVillage(w, "h1", human.X+2+i, human.Y+2+i)
					next.Buildings[game.BMarket], next.Buildings[game.BStorage] = 5, 15
					next.Resources = game.Resources{Wood: 100, Clay: 100, Iron: 9000}
					human = next
				}
				if _, err := game.CreateTradeOffer(w, "h1", game.TradeOfferInput{OriginID: human.ID, Give: game.Resources{Iron: 500}, Want: game.Resources{Wood: 500}}); err != nil {
					t.Fatal(err)
				}
			}
			before := map[string]bool{}
			for _, o := range w.AllTradeOffers() {
				before[o.ID] = true
			}
			game.PlanTrading(w, "bot_0", bot)
			for id := range before {
				if w.TradeOffer(id) == nil {
					return id
				}
			}
			return ""
		}
		first := taken()
		if first == "" {
			t.Fatal("the bot took nothing")
		}
		for i := 0; i < 12; i++ {
			if got := taken(); got != first {
				t.Fatalf("run %d took %q, first run took %q", i, got, first)
			}
		}
	})

	t.Run("a bot leaves a post asking for a resource it is short of", func(t *testing.T) {
		w, bot, human := botTradeWorld(t, 5)
		game.TouchVillages(w, "bot_0")
		game.TouchVillages(w, "h1")
		bot.Resources = game.Resources{Wood: 4000, Clay: 4000, Iron: 30}
		human.Resources = game.Resources{Wood: 100, Clay: 100, Iron: 9000}
		// Gives iron but asks for wood *and* iron: we cannot spare the iron it wants back.
		o, err := game.CreateTradeOffer(w, "h1", game.TradeOfferInput{OriginID: human.ID, Give: game.Resources{Iron: 500}, Want: game.Resources{Iron: 400}})
		if err == nil {
			game.PlanTrading(w, "bot_0", bot)
			if w.TradeOffer(o.ID) == nil {
				t.Fatal("a post wanting our short resource should be left alone")
			}
		}
	})

	t.Run("trading off in the world stops bots posting", func(t *testing.T) {
		w, bot, _ := botTradeWorld(t, 5)
		game.TouchVillages(w, "bot_0")
		cfg := w.Config()
		cfg.TradingEnabled = false
		defer func() { cfg.TradingEnabled = true }()
		bot.Resources = game.Resources{Wood: 9000, Clay: 4000, Iron: 20}
		game.PlanTrading(w, "bot_0", bot)
		if n := len(w.AllTradeOffers()); n != 0 {
			t.Fatalf("want no post, got %d", n)
		}
	})
}

// TestBotTradingEmerges runs a bot-only world forward and checks that trading actually happens with nobody telling
// the bots to do it: posts appear on the board and other bots take them.
func TestBotTradingEmerges(t *testing.T) {
	cfg := game.DefaultWorldConfig
	cfg.BotCount, cfg.BeginnerProtectionDays = 6, 0
	// A small, dense map so the bots are within each other's delivery limits.
	cfg.MapSize = 30
	w := gametest.NewMemWorld(11, cfg, 0)
	if err := game.NewWorld(w); err != nil {
		t.Fatal(err)
	}
	// Half the bots are out of wood and rich in iron, half the other way round, so each side complements the other.
	rich := game.Resources{Wood: 9000, Clay: 5000, Iron: 300}
	poor := game.Resources{Wood: 300, Clay: 5000, Iron: 9000}
	for i, id := range []game.PlayerID{"bot_0", "bot_1", "bot_2", "bot_3", "bot_4", "bot_5"} {
		v := game.PlayerVillages(w, id)[0]
		v.Buildings[game.BMarket], v.Buildings[game.BStorage] = 5, 20
		if i%2 == 0 {
			v.Resources = rich
		} else {
			v.Resources = poor
		}
	}
	peak := 0
	for step := int64(1); step <= 48; step++ {
		if err := gametest.Advance(w, step*game.MsPerHour); err != nil {
			t.Fatalf("advance to hour %d: %v", step, err)
		}
		peak = max(peak, len(w.AllTradeOffers()))
	}
	var posted, settled int
	for _, a := range w.TakeActions() {
		switch a.Kind {
		case "create-trade-offer":
			posted++
		case "accept-trade-offer":
			settled++
		}
	}
	if posted == 0 {
		t.Fatal("bots never posted an offer")
	}
	if settled == 0 {
		t.Fatalf("bots posted %d offers but never took one (peak board %d)", posted, peak)
	}
	t.Logf("posts: %d, settled deals: %d, peak board: %d", posted, settled, peak)
}

// TestBotWithdrawsStaleOffer covers a bot taking its own post back when the situation behind it changed.
func TestBotWithdrawsStaleOffer(t *testing.T) {
	// postBot sets up a village short of iron and rich in wood, with one post giving wood for iron.
	postBot := func(t *testing.T) (*gametest.MemWorld, *game.Village) {
		t.Helper()
		w, bot, _ := botTradeWorld(t, 5)
		game.TouchVillages(w, "bot_0")
		bot.Resources = game.Resources{Wood: 9000, Clay: 4000, Iron: 20}
		game.PlanTrading(w, "bot_0", bot)
		if n := len(w.AllTradeOffers()); n != 1 {
			t.Fatalf("setup should leave one post, got %d", n)
		}
		return w, bot
	}

	t.Run("it keeps a post that still fits", func(t *testing.T) {
		w, bot := postBot(t)
		game.PlanTrading(w, "bot_0", bot)
		if n := len(w.AllTradeOffers()); n != 1 {
			t.Fatalf("a fitting post should stay, got %d", n)
		}
	})

	t.Run("it withdraws once it runs short of what it was offering", func(t *testing.T) {
		w, bot := postBot(t)
		// The wood is gone (spent, looted, or burnt) but the iron never arrived: the post now promises what the
		// village cannot spare.
		game.TouchVillages(w, "bot_0")
		bot.Resources = game.Resources{Wood: 50, Clay: 4000, Iron: 20}
		game.PlanTrading(w, "bot_0", bot)
		if n := len(w.AllTradeOffers()); n != 0 {
			t.Fatalf("a post giving a resource we now lack should be withdrawn, %d left", n)
		}
	})

	t.Run("it withdraws once it no longer needs what it asked for", func(t *testing.T) {
		w, bot := postBot(t)
		// The iron arrived from a raid, so the ask is pointless and the reserved wood is better spent.
		game.TouchVillages(w, "bot_0")
		bot.Resources = game.Resources{Wood: 9000, Clay: 4000, Iron: 6000}
		game.PlanTrading(w, "bot_0", bot)
		if n := len(w.AllTradeOffers()); n != 0 {
			t.Fatalf("a post asking for a resource we no longer lack should be withdrawn, %d left", n)
		}
	})

	t.Run("the reserved share comes home with the cancel", func(t *testing.T) {
		w, bot := postBot(t)
		offers := w.AllTradeOffers()
		reserved := offers[0].Give.Wood
		if reserved <= 0 {
			t.Fatalf("setup post should reserve wood, got %+v", offers[0].Give)
		}
		game.TouchVillages(w, "bot_0")
		before := bot.Resources.Wood
		bot.Resources = game.Resources{Wood: 50, Clay: 4000, Iron: 20}
		game.PlanTrading(w, "bot_0", bot)
		if n := len(w.AllTradeOffers()); n != 0 {
			t.Fatalf("want the post gone, %d left", n)
		}
		// The refund is capped by storage, so it can only be at least what the village held.
		if bot.Resources.Wood < 50 {
			t.Fatalf("the reserved wood should come home, wood %v (held 50, post reserved %v, before %v)", bot.Resources.Wood, reserved, before)
		}
	})

	t.Run("it leaves another player's post alone", func(t *testing.T) {
		w, bot, human := botTradeWorld(t, 5)
		game.TouchVillages(w, "bot_0")
		game.TouchVillages(w, "h1")
		bot.Resources = game.Resources{Wood: 9000, Clay: 4000, Iron: 20}
		human.Resources = game.Resources{Wood: 100, Clay: 5000, Iron: 9000}
		o, err := game.CreateTradeOffer(w, "h1", game.TradeOfferInput{OriginID: human.ID, Give: game.Resources{Iron: 300}, Want: game.Resources{Wood: 500}})
		if err != nil {
			t.Fatal(err)
		}
		// Make the human's post stale for the human, then let the bot think: only its own posts are its business.
		game.PlanTrading(w, "bot_0", bot)
		if w.TradeOffer(o.ID) == nil {
			t.Fatal("a bot must not withdraw someone else's post")
		}
	})

	t.Run("a withdrawn post is re-posted only when the gap is real again", func(t *testing.T) {
		w, bot := postBot(t)
		game.TouchVillages(w, "bot_0")
		bot.Resources = game.Resources{Wood: 50, Clay: 4000, Iron: 20}
		game.PlanTrading(w, "bot_0", bot)
		if n := len(w.AllTradeOffers()); n != 0 {
			t.Fatalf("the stale post should be gone, %d left", n)
		}
		// A raid brings wood back in: the village is lopsided again and posts a fresh deal.
		game.TouchVillages(w, "bot_0")
		bot.Resources = game.Resources{Wood: 9000, Clay: 4000, Iron: 20}
		game.PlanTrading(w, "bot_0", bot)
		if n := len(w.AllTradeOffers()); n != 1 {
			t.Fatalf("a real gap should be advertised again, %d posts", n)
		}
	})
}

// TestBotPostsDoNotPileUp runs a bot-only world for four days and checks the board clears itself: stale posts are
// withdrawn rather than lingering forever with resources locked up behind them.
func TestBotPostsDoNotPileUp(t *testing.T) {
	cfg := game.DefaultWorldConfig
	cfg.BotCount, cfg.BeginnerProtectionDays = 6, 0
	cfg.MapSize = 30
	w := gametest.NewMemWorld(11, cfg, 0)
	if err := game.NewWorld(w); err != nil {
		t.Fatal(err)
	}
	// Every bot is lopsided, so posts are made and then go stale as their villages change.
	rich := game.Resources{Wood: 9000, Clay: 5000, Iron: 300}
	poor := game.Resources{Wood: 300, Clay: 5000, Iron: 9000}
	for i, id := range []game.PlayerID{"bot_0", "bot_1", "bot_2", "bot_3", "bot_4", "bot_5"} {
		v := game.PlayerVillages(w, id)[0]
		v.Buildings[game.BMarket], v.Buildings[game.BStorage] = 5, 20
		if i%2 == 0 {
			v.Resources = rich
		} else {
			v.Resources = poor
		}
	}
	for step := int64(1); step <= 96; step++ {
		if err := gametest.Advance(w, step*game.MsPerHour); err != nil {
			t.Fatalf("advance to hour %d: %v", step, err)
		}
	}
	counts := map[string]int{}
	for _, a := range w.TakeActions() {
		counts[a.Kind]++
	}
	posted, settled, cancelled := counts["create-trade-offer"], counts["accept-trade-offer"], counts["cancel-trade-offer"]
	if posted == 0 {
		t.Fatal("bots never posted")
	}
	if cancelled == 0 {
		t.Fatal("bots never withdrew a post of their own")
	}
	// Every post ends either settled or withdrawn: one post per village means a stale one blocks a fresh one, so
	// an un-cleared board would show up as a long tail of open offers.
	if open := len(w.AllTradeOffers()); open > 6 {
		t.Fatalf("%d posts left open; stale posts should be withdrawn, not piled up", open)
	}
	t.Logf("posts: %d, settled: %d, withdrawn: %d, open at the end: %d", posted, settled, cancelled, len(w.AllTradeOffers()))
}

// TestBotPostsArePricedByNeed pins the band a bot posts at: it must ask more per unit than it gives, never more
// than it would itself pay as a taker, and a village that is nearly out of the resource it wants pays more than one
// that is only mildly lopsided. The board's Ratio column is want/give, so the band reads 1.15 to 1.40.
func TestBotPostsArePricedByNeed(t *testing.T) {
	// post is a village's rate for iron, the resource it lacks, after PlanTrading posts one deal.
	post := func(t *testing.T, wood, iron float64) float64 {
		t.Helper()
		w, bot, _ := botTradeWorld(t, 5)
		game.TouchVillages(w, "bot_0")
		bot.Resources = game.Resources{Wood: wood, Clay: wood, Iron: iron}
		game.PlanTrading(w, "bot_0", bot)
		offers := w.AllTradeOffers()
		if len(offers) != 1 {
			t.Fatalf("want one post, got %d", len(offers))
		}
		give, want := game.SumResources(offers[0].Give), game.SumResources(offers[0].Want)
		if give <= 0 || want <= 0 {
			t.Fatalf("two-sided post expected, give %v want %v", give, want)
		}
		return want / give
	}

	// Barely lopsided: iron is just under half the best store, so the village pays the cheap end of the band.
	cheap := post(t, 4000, 1900)
	// Nearly out of iron: the same gap in kind, but the scarce side is close to nothing.
	dear := post(t, 9000, 20)
	// A small stash makes a small post, where rounding the give side moves the ratio the most: the band must hold here
	// too, not just on the large deals where a single unit is worth less than a percent.
	small := post(t, 2200, 1090)

	for name, rate := range map[string]float64{"cheap": cheap, "dear": dear, "small": small} {
		if rate < 1.15 || rate > 1.40 {
			t.Fatalf("%s post at %.3f is outside the 1.15–1.40 band", name, rate)
		}
	}
	if dear <= cheap {
		t.Fatalf("a starving village should pay more: dear %.3f, cheap %.3f", dear, cheap)
	}
	t.Logf("cheap %.3f dear %.3f small %.3f", cheap, dear, small)
	// The worst a bot posts at stays under what a bot will pay as a taker, or its own posts would price themselves out.
	if dear > 1.25*1.25 {
		t.Fatalf("a post at %.3f exceeds botTradeMaxCost and no bot would take it", dear)
	}
}
