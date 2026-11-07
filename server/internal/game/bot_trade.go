package game

import (
	"math"
	"sort"
)

// Bot trading: a bot that is short of one resource posts an offer for it, and a bot that can spare what a post
// asks for takes it. Both sides go through the same rules humans do — a market, set-aside resources, merchants —
// so bots trade with bots and with humans without any special casing.

// botTradeShare is the share of a spare resource one post commits. Kept small: a post nobody can afford is worse
// than no post, and a village only trades away what it will not miss.
const botTradeShare = 0.10

// botTradeMaxCopies caps how many identical posts one bot takes in a single deal.
const botTradeMaxCopies = 5

// botTradeMinAmount is the smallest stash a bot will trade from: below this there is nothing worth a merchant trip.
const botTradeMinAmount = 200

// botTradeRateMin and botTradeRateMax bound what a bot asks per unit it gives, the `want/give` figure the board's
// Ratio column shows. A village posts only once its scarcest resource is under half its best (tradeGap); the nearer
// that resource is to gone, the more the village pays for it, from a mild imbalance at the low end to a near-empty
// store at the high end. The travelling trader's 0.8 gives 1.25, so the band straddles it. Both ends stay under
// botTradeMaxCost, so one bot's post is still worth taking to another bot, and a human undercutting the band is the
// cheapest deal on the board.
const (
	botTradeRateMin = 1.15
	botTradeRateMax = 1.40
)

// botTradeMaxCost is the worst rate a bot accepts, in units paid per unit received. A post priced at the
// travelling trader's ratio costs 1/ratio, and a little slack above that keeps fair posts acceptable.
const botTradeMaxCost = 1.25 * 1.25

// boardOffers is every offer in a stable order. AllTradeOffers is map-ordered, so a planner that picks "the first"
// or "the cheapest" of a set of equally good deals would pick a different one between two runs of the same world;
// replaying a log has to reach the same state, so the order is pinned here rather than left to the map.
func boardOffers(w World) []*TradeOffer {
	offers := w.AllTradeOffers()
	sort.Slice(offers, func(i, j int) bool {
		if offers[i].CreatedAt != offers[j].CreatedAt {
			return offers[i].CreatedAt < offers[j].CreatedAt
		}
		return offers[i].ID < offers[j].ID
	})
	return offers
}

// PlanTrading runs before the economy planners so a bot short of a resource can fix it before it spends what it
// has. A village never has more than one post of its own out.
func PlanTrading(w World, botID PlayerID, v *Village) {
	if !w.Config().TradingEnabled || !hasMarket(v) || !tradeAllowed(w, botID) {
		return
	}
	// A post that no longer says anything true about the village is pulled before anything else: the situation
	// it was written for has moved on, and the reserved resources are better spent where they are needed now.
	if withdrawStaleOffer(w, botID, v) {
		return
	}
	// Spend the free merchants (and the market) on a good deal before posting a want of our own. Only when the
	// board has nothing for us does the village advertise its own gap.
	if takeBoardOffer(w, botID, v) {
		return
	}
	postWant(w, botID, v)
}

// withdrawStaleOffer takes back a post this village has out when the situation behind it changed: it is now short
// of the very resource it is offering, it no longer needs what it asked for, or the village became balanced enough
// that the deal is pointless. The reserved share comes home with the cancel. Returns whether a post was withdrawn.
func withdrawStaleOffer(w World, botID PlayerID, v *Village) bool {
	var stale *TradeOffer
	for _, o := range boardOffers(w) {
		if o.OriginID != v.ID || !tradeOfferReady(w, o) || o.FromID != botID {
			continue
		}
		if offerStillFits(v, o) {
			return false // one post per village; it still makes sense, leave it be
		}
		stale = o
		break
	}
	if stale == nil {
		return false
	}
	return actSoft(w, botID, "cancel-trade-offer",
		logArgs{"id": stale.ID},
		func() error { return CancelTradeOffer(w, botID, stale.ID) })
}

// offerStillFits is the test a post has to keep passing to stay on the board. A village hands over its spare and
// asks for what it lacks; when either side of that stops being true the post is out of date, and holding the
// resources back does the village more harm than the deal ever would.
func offerStillFits(v *Village, o *TradeOffer) bool {
	short, spare, ok := tradeGap(v)
	if !ok {
		// The village is balanced now: nothing it could ask for is urgent any more.
		return false
	}
	// What the post gives must not be the resource the village is now shortest of.
	for _, r := range ResourceIDs {
		if o.Give.Get(r) > 0 && r == short {
			return false
		}
	}
	// What it asks for must still be the resource the village actually lacks.
	if o.Want.Get(short) <= 0 {
		return false
	}
	// The post must still be aimed at a resource we can spare: if the spare moved on, rewrite the deal instead.
	if o.Give.Get(spare) <= 0 {
		return false
	}
	return true
}

// tradeAllowed: a bot trades on a normal world, and only sometimes on an instant one where every minute counts.
func tradeAllowed(w World, botID PlayerID) bool {
	if !IsShort(w.Config()) {
		return true
	}
	return Mulberry32(HiddenSeed(w, "tradeShort", botID, LocalDayIndex(w, w.Now())))() < BotConfig.ShortTradeChance
}

// postWant offers a spare resource for the one the village is shortest of: ask for a share of the surplus and give
// what that share is worth at the bot's going rate for the missing resource.
func postWant(w World, botID PlayerID, v *Village) {
	// One post per village at a time; a fresh one simply waits for the old one to settle or lapse.
	for _, o := range boardOffers(w) {
		if o.OriginID == v.ID && tradeOfferReady(w, o) {
			return
		}
	}
	short, spare, ok := tradeGap(v)
	if !ok {
		return
	}
	// Take at most a tenth of the surplus so a single post never drains the village.
	ask := math.Floor(v.Resources.Get(spare) * botTradeShare)
	if ask < botTradeMinAmount {
		return
	}
	// Round rather than floor: flooring gives less, which pushes the displayed want/give ratio above the band's top
	// on a small post (ask 200 at 1.40 would read 1.41), while rounding keeps it at 1.40 or just under.
	giveAmount := math.Round(ask / botTradeRate(v, short))
	if giveAmount < 1 {
		return
	}
	give := Resources{}
	give.Set(spare, giveAmount)
	want := Resources{}
	want.Set(short, ask)
	if len(CheckPostOffer(w, v, give, want)) > 0 {
		return
	}
	actSoft(w, botID, "create-trade-offer",
		logArgs{"originId": v.ID, "public": true, "give": give, "want": want, "maxDeliveryMs": botDeliveryLimit(w)},
		func() error {
			_, err := CreateTradeOffer(w, botID, TradeOfferInput{
				OriginID: v.ID, Give: give, Want: want, MaxDeliveryMs: botDeliveryLimit(w),
			})
			return err
		})
}

// botTradeRate is what the village pays per unit of `short` it asks for: botTradeRateMin while it is merely lopsided
// enough to post at all, rising to botTradeRateMax as the resource runs out. Scarcity is measured past the point
// tradeGap posts at (the scarce resource at half the best store), so the two ends line up with what a bot actually
// does: right at that threshold it pays the cheap end, with none of the resource left it pays the most.
func botTradeRate(v *Village, short Resource) float64 {
	best := 0.0
	for _, r := range ResourceIDs {
		if have := v.Resources.Get(r); have > best {
			best = have
		}
	}
	if best <= 0 {
		return botTradeRateMax
	}
	// 0 = the scarce resource is half the best store (the threshold for posting), 1 = none of it left.
	scarcity := math.Min(1, math.Max(0, (best-2*v.Resources.Get(short))/best))
	return botTradeRateMin + (botTradeRateMax-botTradeRateMin)*scarcity
}

// CheckPostOffer are the local reasons not to post, checked before the real call so a bot does not spam
// refusals: nothing to give, or no free merchant for it.
func CheckPostOffer(w World, v *Village, give, want Resources) []string {
	var out []string
	if SumResources(give)+SumResources(want) <= 0 {
		out = append(out, "empty")
	}
	if SumResources(give) > 0 && MerchantsNeeded(give) > MerchantsAvailable(w, v) {
		out = append(out, "merchants")
	}
	return out
}

// botDeliveryLimit: how far a bot lets its post reach. Bots post at the world's cap so a neighbour can always
// take the deal — a bot has no way to look up how far away the takers are when it posts.
func botDeliveryLimit(w World) int64 { return TradeBalance.MaxDeliveryMs }

// tradeGap names the resource the village is shortest of and the one it can best spare, judged against the
// village's own holdings rather than its storage: a bot with 900 wood and 200 iron is out of iron whatever its
// storehouse size. ok is false when the village is balanced enough not to trade.
func tradeGap(v *Village) (short, spare Resource, ok bool) {
	low, high := math.Inf(1), -1.0
	for _, r := range ResourceIDs {
		have := v.Resources.Get(r)
		if have < low {
			low, short = have, r
		}
		if have > high {
			high, spare = have, r
		}
	}
	if short == spare {
		return 0, 0, false
	}
	// A real imbalance worth a merchant trip, and enough of the spare to make a sensible offer.
	if high < botTradeMinAmount*2 || high < low*2 {
		return 0, 0, false
	}
	return short, spare, true
}

// takeBoardOffer picks up the best public post the village can take: it must want something we have in surplus,
// give something we are short of, and cost us no more merchants than we have free. Returns whether one settled.
func takeBoardOffer(w World, botID PlayerID, v *Village) bool {
	type pick struct {
		offer *TradeOffer
		value float64
		ids   []string
	}
	var picks []pick
	for _, o := range boardOffers(w) {
		if !tradeOfferReady(w, o) || o.FromID == botID {
			continue
		}
		if !deliverable(w, o, v) {
			continue
		}
		// The post has to give what this village is short of, and ask for something it can spare. Both are judged
		// per post: a village short of wood may still be the right taker for a wood-for-iron post.
		if !wantsWhatWeLack(v, o.Give) || !hasSpare(v, o.Want) {
			continue
		}
		if !CanShip(w, v, o.Want) {
			continue
		}
		// Value = what we gain per unit we give away; only favourable deals are taken.
		giveTotal, wantTotal := SumResources(o.Give), SumResources(o.Want)
		if wantTotal <= 0 {
			continue // a gift: always welcome, priced as infinitely good
		}
		if giveTotal <= 0 {
			continue
		}
		// value = what we pay per unit we receive. Bots post between botTradeRateMin and botTradeRateMax, and humans
		// price however they like; botTradeMaxCost is the worst rate still worth taking.
		value := wantTotal / giveTotal
		if value > botTradeMaxCost {
			continue
		}
		picks = append(picks, pick{offer: o, value: value})
	}
	if len(picks) == 0 {
		return false
	}
	// Cheapest deal first: least given away per unit received.
	sort.SliceStable(picks, func(i, j int) bool { return picks[i].value < picks[j].value })
	o := picks[0].offer
	// Take as many copies as the village's resources and merchants allow, at most the player's board cap.
	n := 1
	for _, other := range picks {
		if other.offer.FromID == o.FromID && other.offer.OriginID == o.OriginID &&
			other.offer.Give == o.Give && other.offer.Want == o.Want {
			n++
		}
	}
	n = min(n, maxCopies(w, v, o))
	if n <= 0 {
		return false
	}
	ids := make([]string, 0, n)
	for _, other := range picks {
		if len(ids) >= n {
			break
		}
		if other.offer.FromID == o.FromID && other.offer.OriginID == o.OriginID &&
			other.offer.Give == o.Give && other.offer.Want == o.Want {
			ids = append(ids, other.offer.ID)
		}
	}
	return actSoft(w, botID, "accept-trade-offer",
		logArgs{"offerIds": ids, "villageId": v.ID},
		func() error { return AcceptTradeOffers(w, botID, ids, v.ID) })
}

// maxCopies is how many identical posts the village can pay for and ship at once: both the resources it owes and
// the free merchants to carry them, capped so one bot never sweeps a whole board.
func maxCopies(w World, v *Village, o *TradeOffer) int {
	n := 0
	for n < botTradeMaxCopies {
		next := n + 1
		want := Resources{Wood: o.Want.Wood * float64(next), Clay: o.Want.Clay * float64(next), Iron: o.Want.Iron * float64(next)}
		if !HasResources(v.Resources, want) {
			break
		}
		// Mirror what the engine checks for the batch: the seller must have merchants for the whole give side and
		// the taker for the whole want side, so a batch can be refused for merchants even when one copy fits.
		if SumResources(want) > 0 && MerchantsNeeded(want) > MerchantsAvailable(w, v) {
			break
		}
		if seller := w.Village(o.OriginID); seller != nil && SumResources(o.Give) > 0 &&
			MerchantsNeeded(ScaleResources(o.Give, float64(next))) > MerchantsAvailable(w, seller) {
			break
		}
		n = next
	}
	return n
}

// wantsWhatWeLack reports whether a post gives a resource this village is comparatively short of: it holds clearly
// less of it than of its best-stocked resource, so taking the deal leaves it better balanced.
func wantsWhatWeLack(v *Village, give Resources) bool {
	most := 0.0
	for _, r := range ResourceIDs {
		if have := v.Resources.Get(r); have > most {
			most = have
		}
	}
	for _, r := range ResourceIDs {
		if give.Get(r) <= 0 {
			continue
		}
		have := v.Resources.Get(r)
		if have < botTradeMinAmount || have < most/2 {
			return true
		}
	}
	return false
}

// hasSpare reports whether the village can pay `amount` away: it must keep a working stash of everything it hands
// over, and it must not pay away the resource it has least of.
func hasSpare(v *Village, amount Resources) bool {
	lowest := math.Inf(1)
	for _, r := range ResourceIDs {
		if have := v.Resources.Get(r); have < lowest {
			lowest = have
		}
	}
	for _, r := range ResourceIDs {
		n := amount.Get(r)
		if n <= 0 {
			continue
		}
		have := v.Resources.Get(r)
		if have <= lowest && n > 0 && have-n < lowest {
			return false // it is the scarcest resource we have, and paying would deepen that
		}
		if have-n < botTradeMinAmount {
			return false
		}
	}
	return true
}
