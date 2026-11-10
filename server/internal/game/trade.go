package game

import (
	"sort"
	"strconv"
)

// Player to player trades. A trade is an offer: send me these resources, bring back those. A post sits on the public
// board and anyone it can deliver to may pick it up. An accepted deal fires two shipments (each side pays its own
// merchants) using the plain market trade machinery.

func init() {
	registerHandler("tradeExpire", onTradeOfferExpired)
}

// TradeOfferInput is what a client names when posting an offer. Every post goes on the public board and may be
// taken by anyone whose merchant can reach it in time. MaxDeliveryMs (0 = unlimited) bounds that trip.
type TradeOfferInput struct {
	OriginID      string    `json:"originId"`
	Give          Resources `json:"give"`          // what From sends to the taker
	Want          Resources `json:"want"`          // what the taker brings back
	MaxDeliveryMs int64     `json:"maxDeliveryMs"` // 0 = unlimited
}

// tradeOfferReady reports whether an offer is still alive.
func tradeOfferReady(w World, o *TradeOffer) bool { return o != nil && o.ExpiresAt > w.Now() }

// hasMarket is the building gate both sides of a deal must clear (a level 1 market).
func hasMarket(v *Village) bool { return v != nil && v.Buildings[BMarket] >= 1 }

// CreateTradeOffer posts an offer from one of the actor's villages. The "give" resources are set aside from
// the origin village now, so nobody can spend them while the offer is out; a cancel or the expiry hands them back.
func CreateTradeOffer(w World, actor PlayerID, in TradeOfferInput) (*TradeOffer, error) {
	if !w.Config().TradingEnabled {
		return nil, Blocked("create-trade-offer", Blocker{"kind": "tradingOff"})
	}
	origin, err := ownsVillage(w, actor, in.OriginID, "create-trade-offer")
	if err != nil {
		return nil, err
	}
	give := cleanAmount(in.Give)
	want := cleanAmount(in.Want)

	var blockers []Blocker
	switch {
	case !hasMarket(origin):
		blockers = append(blockers, Blocker{"kind": "noMarket"})
	case SumResources(give) <= 0:
		// A deal swaps resources: an offer that gives nothing is a request for a gift, and because it reserves
		// nothing it would also post for free past the board cap. Both sides are required.
		blockers = append(blockers, Blocker{"kind": "empty"})
	case SumResources(want) <= 0:
		blockers = append(blockers, Blocker{"kind": "empty"})
	case SumResources(give) > 0 && !HasResources(origin.Resources, give):
		// The share is reserved from the village there and then, so it has to be there to reserve.
		blockers = append(blockers, Blocker{"kind": "resources"})
	}

	// The merchants that will carry the give side are booked now, next to the resources, so a post on the board can
	// always be accepted: the taker is never refused for a stranger's busy merchants, which they cannot see and
	// cannot fix. The market's level is the merchant count, so a share larger than the whole market is the same
	// answer as a target that cannot hold it — a smaller offer, not a wait.
	// Every post books, copies included: each copy a taker picks up ships its own haul, so a shared booking would
	// let a village promise more than its market can carry. The board cap counts distinct deals only, so that — not
	// the merchant pool — is what stops a village filling itself up with copies.
	known := isKnownPost(w, actor, in, give, want)
	reserved := 0
	if SumResources(give) > 0 {
		reserved = MerchantsNeeded(give)
		switch {
		case reserved > MerchantsTotal(origin):
			blockers = append(blockers, Blocker{"kind": "marketTooSmall", "needed": reserved, "total": MerchantsTotal(origin)})
		case reserved > MerchantsAvailable(w, origin):
			blockers = append(blockers, Blocker{"kind": "merchants", "needed": reserved, "available": MerchantsAvailable(w, origin)})
		}
	}
	if !known && activeOffersAt(w, actor, in.OriginID) >= TradeBalance.PublicPerVillage {
		blockers = append(blockers, Blocker{"kind": "boardFull"})
	}
	if len(blockers) > 0 {
		return nil, Blocked("create-trade-offer", blockers...)
	}

	now := w.Now()
	// Set the giver's share aside before the offer goes up; a cancel or the expiry hands it back.
	if SumResources(give) > 0 {
		SubtractResources(&origin.Resources, give)
	}
	o := &TradeOffer{ID: w.NewID("to"), FromID: actor, OriginID: in.OriginID,
		Give: give, Want: want, Reserved: reserved, MaxDeliveryMs: normalizeDelivery(in.MaxDeliveryMs),
		ExpiresAt: now + TradeOfferExpiryMs(w.Config()), CreatedAt: now}
	w.PutTradeOffer(o)
	w.Schedule("tradeExpire:"+o.ID, o.ExpiresAt)
	Touch(w, origin)
	return o, nil
}

// activeOffersAt counts the distinct posts one village has out. The cap is per origin village because it is that
// village's merchants that deliver, so owning more villages adds slots per village rather than raising a pool they
// all share. Copies of one deal count once: they are the same offer, however many takers they are meant for.
func activeOffersAt(w World, actor PlayerID, originID string) int {
	seen := make(map[string]bool)
	n := 0
	for _, o := range w.AllTradeOffers() {
		if o.FromID != actor || o.OriginID != originID || !tradeOfferReady(w, o) {
			continue
		}
		if key := postKey(o); !seen[key] {
			seen[key] = true
			n++
		}
	}
	return n
}

// dealKey is the one identity of a deal, used both for "is this a copy of something already out" (so free of the
// board cap) and for counting the distinct deals that fill it. The two must agree: if they did not, one option of a
// deal could be cap-free and a distinct post at the same time.
func dealKey(originID string, give, want Resources, maxDeliveryMs int64) string {
	return originID + "|" + postAmounts(give) + "|" + postAmounts(want) + "|" + strconv.FormatInt(maxDeliveryMs, 10)
}

func postKey(o *TradeOffer) string { return dealKey(o.OriginID, o.Give, o.Want, o.MaxDeliveryMs) }

func postAmounts(r Resources) string {
	return strconv.FormatFloat(r.Wood, 'f', -1, 64) + "," +
		strconv.FormatFloat(r.Clay, 'f', -1, 64) + "," +
		strconv.FormatFloat(r.Iron, 'f', -1, 64)
}

// isKnownPost reports whether an identical deal is already on the board, which would make a new copy free of the cap.
func isKnownPost(w World, actor PlayerID, in TradeOfferInput, give, want Resources) bool {
	key := dealKey(in.OriginID, give, want, normalizeDelivery(in.MaxDeliveryMs))
	for _, o := range w.AllTradeOffers() {
		if o.FromID == actor && tradeOfferReady(w, o) && postKey(o) == key {
			return true
		}
	}
	return false
}

// normalizeDelivery clamps a posted delivery limit to the world's band and treats 0 as unlimited.
func normalizeDelivery(ms int64) int64 {
	if ms <= 0 {
		return 0 // unlimited
	}
	if ms < TradeBalance.MinDeliveryMs {
		ms = TradeBalance.MinDeliveryMs
	}
	if ms > TradeBalance.MaxDeliveryMs {
		ms = TradeBalance.MaxDeliveryMs
	}
	return ms
}

// CancelTradeOffer withdraws an offer the actor posted; the set-aside share goes back to its village.
func CancelTradeOffer(w World, actor PlayerID, id string) error {
	// Deliberately not gated on TradingEnabled: withdrawing is how a player takes their resources back after the host
	// switches trading off, so it has to keep working.
	o := w.TradeOffer(id)
	if o == nil || o.FromID != actor {
		return NotOwner("cancel-trade-offer")
	}
	w.DeleteTradeOffer(id)
	w.Unschedule("tradeExpire:" + id)
	refundReserved(w, o)
	return nil
}

// AcceptTradeOffer settles one deal: the taker's chosen village takes in what From gives and sends back what it
// wants. Both villages need a level 1 market; each side pays merchants for what it ships. The offer is consumed.
func AcceptTradeOffer(w World, actor PlayerID, id, villageID string) error {
	return AcceptTradeOffers(w, actor, []string{id}, villageID)
}

// AcceptTradeOffers settles several offers in one go, which is how a taker picks up N copies of the same board
// post. Everything is checked against the totals before a single resource moves, so a taker who can afford one
// copy but not five is refused outright instead of settling a partial deal: one market, one merchant pool and
// one haul per side. Each offer is still consumed and reported on its own.
func AcceptTradeOffers(w World, actor PlayerID, ids []string, villageID string) error {
	if !w.Config().TradingEnabled {
		return Blocked("accept-trade-offer", Blocker{"kind": "tradingOff"})
	}
	if len(ids) == 0 {
		return Rule("noOffer", map[string]any{"id": ""})
	}
	offers := make([]*TradeOffer, 0, len(ids))
	seen := make(map[string]bool, len(ids))
	for _, id := range ids {
		o := w.TradeOffer(id)
		if o == nil || !tradeOfferReady(w, o) || seen[id] {
			return Rule("noOffer", map[string]any{"id": id})
		}
		if o.FromID == actor {
			return NotOwner("accept-trade-offer")
		}
		seen[id] = true
		offers = append(offers, o)
	}

	// A taker cannot mix posts from different sellers at different ranges into one deal.
	origin := w.Village(offers[0].OriginID)
	if origin == nil || origin.OwnerID != Ref(offers[0].FromID) {
		return Blocked("accept-trade-offer", Blocker{"kind": "gone"})
	}
	taker, err := ownsVillage(w, actor, villageID, "accept-trade-offer")
	if err != nil {
		return err
	}

	var totalGive, totalWant Resources
	var blockers []Blocker
	if !hasMarket(taker) {
		blockers = append(blockers, Blocker{"kind": "noMarket"})
	} else if !hasMarket(origin) {
		blockers = append(blockers, Blocker{"kind": "noMarket"})
	}
	for _, o := range offers {
		give, want := cleanAmount(o.Give), cleanAmount(o.Want)
		if SumResources(give)+SumResources(want) <= 0 {
			blockers = append(blockers, Blocker{"kind": "empty"})
			continue
		}
		if v := w.Village(o.OriginID); v == nil || v.OwnerID != Ref(o.FromID) || v.ID != origin.ID {
			blockers = append(blockers, Blocker{"kind": "gone"})
			continue
		}
		if !deliverable(w, o, taker) {
			blockers = append(blockers, Blocker{"kind": "outOfRange"})
			continue
		}
		totalGive = AddResources(totalGive, give)
		totalWant = AddResources(totalWant, want)
	}
	if len(blockers) > 0 {
		return Blocked("accept-trade-offer", blockers...)
	}

	// Both the giver's resources and its merchants were set aside when each offer went up, so there is nothing left to
	// refuse here: an offer on the board can always be accepted. Checking again would count its own reservation
	// against it and turn every accept into "not enough merchants".
	// The taker's side must hold everything it owes at once and have enough free merchants for the whole haul. The
	// market's level is the merchant count, so a haul bigger than the whole market can never be shipped: that is a
	// different answer from "some merchants are busy", and the player fixes it a different way.
	if SumResources(totalWant) > 0 {
		switch need := MerchantsNeeded(totalWant); {
		case !HasResources(taker.Resources, totalWant):
			blockers = append(blockers, Blocker{"kind": "resources"})
		case need > MerchantsTotal(taker):
			blockers = append(blockers, Blocker{"kind": "marketTooSmall"})
		case need > MerchantsAvailable(w, taker):
			blockers = append(blockers, Blocker{"kind": "merchants"})
		}
	}
	if len(blockers) > 0 {
		return Blocked("accept-trade-offer", blockers...)
	}

	// Both legs are booked once for the whole deal, so the merchant count matches the pre-check exactly: one haul
	// each way. The giver's leg is an already-reserved haul and only books merchants; the taker's pays its share.
	// Both are marked Settled: the offer is consumed below and each side ships against the other, so neither leg may
	// be recalled. Without that a taker could take the goods and cancel its payment, or a seller could cancel its
	// give-side after the taker had already paid — minting resources from the other party every cycle.
	if SumResources(totalGive) > 0 {
		dispatchTrade(w, origin.ID, taker.ID, totalGive)
	}
	if SumResources(totalWant) > 0 {
		if _, err := dispatchSettled(w, actor, taker.ID, origin.ID, totalWant); err != nil {
			return err
		}
	}
	for _, o := range offers {
		w.DeleteTradeOffer(o.ID)
		w.Unschedule("tradeExpire:" + o.ID)
	}

	for _, to := range []PlayerID{offers[0].FromID, actor} {
		if w.Human(to) != nil {
			AddNewsItem(w, "news.tradeSettled", map[string]any{}, "", false, to)
		}
	}
	sendMarketReports(w, actor, offers[0].FromID, origin.ID, taker.ID, totalGive, totalWant, len(offers))
	return nil
}

// sendMarketReports tells both sides of a settled deal, one report each however many posts the deal covered: a taker
// who picks up ten copies of a post has one deal to read, not ten. The reader is told who the other party was, where
// the goods they are waiting for are shipped from and how long that takes.
func sendMarketReports(w World, takerID, posterID PlayerID, fromVillageID, toVillageID string,
	posterGives, takerGives Resources, offers int) {
	now := w.Now()
	// The give side travels poster -> taker; the want side taker -> poster. Each reader is told about the leg that
	// brings them their resources, which is the side dispatched by the other player.
	for _, side := range []struct {
		reader, other PlayerID
		give, want    Resources
		fromVillage   string
		toVillage     string
	}{
		{reader: posterID, other: takerID, give: posterGives, want: takerGives, fromVillage: toVillageID, toVillage: fromVillageID},
		{reader: takerID, other: posterID, give: takerGives, want: posterGives, fromVillage: fromVillageID, toVillage: toVillageID},
	} {
		if w.Player(side.reader) == nil {
			continue
		}
		m := &MarketReport{
			CounterpartyID: side.other,
			FromVillage:    ReportVillageOf(w, side.fromVillage),
			Give:           side.give,
			Want:           side.want,
			Offers:         offers,
		}
		if p := w.Player(side.other); p != nil {
			m.CounterpartyName, m.CounterpartyBot = p.Name, !p.IsHuman
		}
		if from, to := w.Village(side.fromVillage), w.Village(side.toVillage); from != nil && to != nil {
			m.TravelMs = TradeTravelMs(from, to, w.Config().UnitSpeed)
		}
		w.AddReport(side.reader, &Report{
			Time:     now,
			Type:     "market",
			OriginID: side.fromVillage,
			TargetID: side.toVillage,
			Origin:   ReportVillageOf(w, side.fromVillage),
			Target:   ReportVillageOf(w, side.toVillage),
			Market:   m,
		})
	}
}

// dispatchTrade books an already-reserved haul onto the road. The resources were set aside when the offer was
// posted, so this skips the ownership and balance checks a plain market trade makes; merchants are checked by
// the caller right before. Arrival and merchant return ride the ordinary market-trade command machinery.
func dispatchTrade(w World, originID, targetID string, amount Resources) {
	m := MerchantsNeeded(amount)
	now := w.Now()
	c := &Command{ID: w.NewID("c"), Type: "trade", OriginID: originID, TargetID: targetID, Haul: &amount,
		Merchants: m, DepartedAt: now, Settled: true,
		ArrivesAt: now + TradeTravelMs(w.Village(originID), w.Village(targetID), w.Config().UnitSpeed)}
	AddCommand(w, c)
}

// CanShip reports whether a village can dispatch `amount`: it holds the resources and has enough free merchants.
func CanShip(w World, v *Village, amount Resources) bool {
	if !HasResources(v.Resources, amount) {
		return false
	}
	return MerchantsNeeded(amount) <= MerchantsAvailable(w, v)
}

// deliverable reports whether merchants can bring a public post's give side to `taker` inside its posted
// delivery limit (0 = unlimited). The trade itself is timed by the slower of the two legs, but the limit is
// about what the poster promises: how long the goods take to arrive.
func deliverable(w World, o *TradeOffer, taker *Village) bool {
	if o.MaxDeliveryMs <= 0 {
		return true
	}
	origin := w.Village(o.OriginID)
	if origin == nil || taker == nil {
		return false
	}
	return TradeTravelMs(origin, taker, w.Config().UnitSpeed) <= o.MaxDeliveryMs
}

// VisibleOffers is everything the viewer may act on or see: their own posts, and posts that can deliver to one of
// their villages. Newest first.
func VisibleOffers(w World, viewer PlayerID) []*TradeOffer {
	if !w.Config().TradingEnabled {
		// Nothing to show, and nothing that could be taken: the market is empty while trading is off.
		return nil
	}
	var own, pub []*TradeOffer
	for _, o := range w.AllTradeOffers() {
		if !tradeOfferReady(w, o) {
			continue
		}
		if o.FromID == viewer {
			own = append(own, o)
			continue
		}
		if deliverableTo(w, o, viewer) {
			pub = append(pub, o)
		}
	}
	out := append([]*TradeOffer{}, own...)
	out = append(out, pub...)
	sort.SliceStable(out, func(i, j int) bool { return out[i].CreatedAt > out[j].CreatedAt })
	return out
}

// deliverableTo reports whether a post can deliver to any of the viewer's villages.
func deliverableTo(w World, o *TradeOffer, viewer PlayerID) bool {
	from := w.Player(viewer)
	if from == nil {
		return false
	}
	for _, id := range from.VillageIDs {
		if v := w.Village(id); v != nil && deliverable(w, o, v) {
			return true
		}
	}
	return false
}

// onTradeOfferExpired lapses an offer whose time ran out (the `tradeExpire:<id>` event) and hands the set-aside
// share back to its village.
func onTradeOfferExpired(w World, id string) error {
	if o := w.TradeOffer(id); o != nil && o.ExpiresAt <= w.Now() {
		w.DeleteTradeOffer(id)
		refundReserved(w, o)
	}
	return nil
}

// refundReserved hands an offer's set-aside share back to its village (a cancel or the expiry). If the storage
// no longer fits it all, only what room remains comes home — the rest is lost, which mirrors what a full storehouse
// does to production rather than inventing somewhere to put it.
func refundReserved(w World, o *TradeOffer) {
	if SumResources(o.Give) <= 0 {
		return
	}
	v := w.Village(o.OriginID)
	if v == nil || v.OwnerID != Ref(o.FromID) {
		// The village was conquered or razed while the offer was up. There is no owner left to hand the share to, so
		// it goes to the player who posted it if they still hold any village, and lapses with the world otherwise.
		if dest := fallbackVillage(w, o.FromID); dest != nil {
			AddResourcesCapped(dest, o.Give)
			Touch(w, dest)
		}
		return
	}
	AddResourcesCapped(v, o.Give)
	Touch(w, v)
}

// fallbackVillage is any village the player still owns, for resources whose own village is gone.
func fallbackVillage(w World, player PlayerID) *Village {
	p := w.Player(player)
	if p == nil {
		return nil
	}
	for _, id := range p.VillageIDs {
		if v := w.Village(id); v != nil && v.OwnerID == Ref(player) {
			return v
		}
	}
	return nil
}

// CloseMarket takes every open offer off the board and hands its reserved give side back, for when an admin switches
// trading off. Without it the offers would sit in storage with their resources still deducted and, since the market is
// hidden while trading is off, nobody could withdraw them: the resources would be unreachable until the world ended.
// Each share goes back to the village that reserved it, capped at that village's storehouse exactly like any other
// refund (AddResourcesCapped) — a village already above capacity, or whose storehouse was demolished while the offer
// was up, keeps only what fits and the excess is lost. Returns how many offers were closed.
func CloseMarket(w World) int {
	offers := w.AllTradeOffers()
	for _, o := range offers {
		w.DeleteTradeOffer(o.ID)
		w.Unschedule("tradeExpire:" + o.ID)
		refundReserved(w, o)
	}
	return len(offers)
}
