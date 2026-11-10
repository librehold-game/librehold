package game

import (
	"math"
	"strings"
)

// The market. Resources go to any player's village; the travelling trader
// is per human.

func init() {
	tradeArrival = onTradeArrival
	registerHandler("trader", onTraderEvent)
	humanJoinHooks = append(humanJoinHooks, func(w World, id PlayerID) {
		w.Human(id).Trader = TraderState{Offer: TraderOfferAt(w, 0), ExpiresAt: w.Now() + TraderBalance.IntervalMs}
		w.Schedule("trader:"+id, w.Now()+TraderBalance.IntervalMs)
	})
}

// MerchantsTotal is how many merchants a village's market gives. One per market level up to
// TradeBalance.MerchantCurveAt, then quadratic — the same shape Tribal Wars uses, so the expensive top levels of the
// market are worth building: level 19 gives 91 merchants rather than 19, level 25 gives 235 rather than 25.
func MerchantsTotal(v *Village) int {
	return MerchantsForLevel(v.Buildings[BMarket])
}

// MerchantsForLevel is MerchantsTotal without a village, for the table and the client.
func MerchantsForLevel(level int) int {
	if level <= 0 {
		return 0
	}
	n := TradeBalance.MerchantCurveAt
	if level <= n {
		return level
	}
	steps := level - n + 1
	return steps*steps + n - 1
}

// sameOwner reports whether both villages belong to the same player, which is what makes a transfer a move between
// one's own villages rather than a trade with somebody else.
func sameOwner(w World, a, b string) bool {
	va, vb := w.Village(a), w.Village(b)
	return va != nil && vb != nil && va.OwnerID != "" && va.OwnerID == vb.OwnerID
}

// MerchantsInTransit is every merchant of the village that is spoken for: those carrying a shipment and those held
// against an offer it still has on the board, which is why posting an offer costs merchants before anyone accepts.
func MerchantsInTransit(w World, villageID string) int {
	n := 0
	for _, c := range w.CommandsHome(villageID) {
		if c.Type == "trade" {
			n += c.Merchants
		}
	}
	for _, o := range w.AllTradeOffers() {
		if o.OriginID == villageID {
			n += ReservedMerchants(o)
		}
	}
	return n
}

// ReservedMerchants is how many of the seller's merchants an offer holds. Offers posted before merchants were
// booked carry no count, so they are valued from what they give — the amount an accept would have needed anyway.
func ReservedMerchants(o *TradeOffer) int {
	if o.Reserved > 0 {
		return o.Reserved
	}
	return MerchantsNeeded(o.Give)
}

func MerchantsAvailable(w World, v *Village) int {
	return max(0, MerchantsTotal(v)-MerchantsInTransit(w, v.ID))
}

func MerchantsNeeded(amount Resources) int {
	return int(math.Ceil(SumResources(amount) / CommandBalance.MerchantCapacity))
}

// TradeTravelMs: merchants, like troops, depend on unit speed, not world speed.
func TradeTravelMs(origin, target *Village, unitSpeed float64) int64 {
	d := Distance(origin.X, origin.Y, target.X, target.Y)
	return max(1, int64(math.Round(d*CommandBalance.MerchantSpeed*float64(MsPerMinute)/unitSpeed)))
}

type TradeCheck struct {
	Blockers  []Blocker
	Merchants int
	TravelMs  int64
}

func cleanAmount(a Resources) Resources {
	return Resources{math.Max(0, math.Floor(a.Wood)), math.Max(0, math.Floor(a.Clay)), math.Max(0, math.Floor(a.Iron))}
}

func CheckTrade(w World, actor PlayerID, originID, targetID string, amount Resources) TradeCheck {
	origin, target := w.Village(originID), w.Village(targetID)
	if origin == nil || origin.OwnerID != Ref(actor) {
		return TradeCheck{Blockers: []Blocker{{"kind": "notOwner"}}}
	}
	var blockers []Blocker
	// Switching trading off closes the player to player market only. A village moving its own resources between its
	// own villages is not trading with anyone, so it keeps working — the same reason the travelling trader stays.
	if !w.Config().TradingEnabled && !sameOwner(w, originID, targetID) {
		blockers = append(blockers, Blocker{"kind": "tradingOff"})
	}
	if origin.Buildings[BMarket] < 1 {
		blockers = append(blockers, Blocker{"kind": "noMarket"})
	}
	if target == nil || target.OwnerID == "" {
		blockers = append(blockers, Blocker{"kind": "noTarget"})
	}
	if originID == targetID {
		blockers = append(blockers, Blocker{"kind": "self"})
	}
	clean := cleanAmount(amount)
	if SumResources(clean) <= 0 {
		blockers = append(blockers, Blocker{"kind": "empty"})
	}
	if !HasResources(origin.Resources, clean) {
		blockers = append(blockers, Blocker{"kind": "resources"})
	}
	merchants := MerchantsNeeded(clean)
	if available := MerchantsAvailable(w, origin); merchants > available {
		blockers = append(blockers, Blocker{"kind": "merchants", "needed": merchants, "available": available})
	}
	var travel int64
	if target != nil {
		travel = TradeTravelMs(origin, target, w.Config().UnitSpeed)
	}
	return TradeCheck{blockers, merchants, travel}
}

// SendTrade sends resources to a village; the merchants come back after delivering.
func SendTrade(w World, actor PlayerID, originID, targetID string, amount Resources) (*Command, error) {
	return sendTrade(w, actor, originID, targetID, amount, false)
}

// dispatchSettled ships the half of an accepted offer. Same shipment, but marked Settled so neither side can
// recall it: by the time it is on the road the offer is gone and the other party has shipped against it.
func dispatchSettled(w World, actor PlayerID, originID, targetID string, amount Resources) (*Command, error) {
	return sendTrade(w, actor, originID, targetID, amount, true)
}

func sendTrade(w World, actor PlayerID, originID, targetID string, amount Resources, settled bool) (*Command, error) {
	if !w.Config().TradingEnabled && !sameOwner(w, originID, targetID) {
		return nil, Blocked("trade", Blocker{"kind": "tradingOff"})
	}
	if origin := w.Village(originID); origin != nil && origin.OwnerID == Ref(actor) {
		Touch(w, origin)
	}
	check := CheckTrade(w, actor, originID, targetID, amount)
	if len(check.Blockers) > 0 {
		return nil, Blocked("trade", check.Blockers...)
	}
	origin := w.Village(originID)
	clean := cleanAmount(amount)
	SubtractResources(&origin.Resources, clean)
	c := &Command{ID: w.NewID("c"), Type: "trade", OriginID: originID, TargetID: targetID, Haul: &clean,
		Merchants: check.Merchants, DepartedAt: w.Now(), ArrivesAt: w.Now() + check.TravelMs, Settled: settled}
	AddCommand(w, c)
	if h := w.Human(actor); h != nil {
		h.Stats.TradesSent++
	}
	return c, nil
}

func onTradeArrival(w World, c *Command, target *Village) error {
	if c.Returning {
		return nil
	}
	if c.Haul != nil {
		Touch(w, target)
		AddResourcesCapped(target, *c.Haul)
	}
	AddCommand(w, &Command{ID: w.NewID("c"), Type: "trade", OriginID: c.TargetID, TargetID: c.OriginID,
		Merchants: c.Merchants, Returning: true, DepartedAt: w.Now(), ArrivesAt: w.Now() + max(1, c.ArrivesAt-c.DepartedAt)})
	return nil
}

// Surplus is the resources above `ratio` of storage capacity.
func Surplus(v *Village, ratio float64) Resources {
	capacity := VillageStorageCapacity(v)
	var out Resources
	for _, r := range ResourceIDs {
		out.Set(r, math.Max(0, math.Floor(v.Resources.Get(r)-capacity*ratio)))
	}
	return out
}

// BalanceAmount ("split evenly") sends half of origin's surplus over target for each resource.
func BalanceAmount(origin, target *Village) Resources {
	var out Resources
	for _, r := range ResourceIDs {
		out.Set(r, math.Max(0, math.Floor((origin.Resources.Get(r)-target.Resources.Get(r))/2)))
	}
	return out
}

// ---------- travelling trader ----------

func TraderOfferAt(w World, index int) TraderOffer {
	rng := Mulberry32(HiddenSeed(w, "trader", index))
	give := ResourceIDs[RandInt(rng, 0, 2)]
	get := ResourceIDs[RandInt(rng, 0, 2)]
	if get == give {
		get = ResourceIDs[(int(give)+1+RandInt(rng, 0, 1))%3]
	}
	amount := TraderBalance.Amounts[RandInt(rng, 0, len(TraderBalance.Amounts)-1)]
	return TraderOffer{Give: give, GiveAmount: amount, Get: get, GetAmount: math.Round(amount * TraderBalance.Ratio)}
}

func rollTrader(w World, h *HumanState) {
	next := h.Trader.OfferIndex + 1
	h.Trader = TraderState{OfferIndex: next, Offer: TraderOfferAt(w, next), ExpiresAt: h.Trader.ExpiresAt + TraderBalance.IntervalMs}
}

// onTraderEvent (`trader:<human>`): a new offer and the next change planned.
func onTraderEvent(w World, ref string) error {
	id := PlayerID(strings.TrimSpace(ref))
	h := w.Human(id)
	if h == nil {
		return nil
	}
	if h.Trader.ExpiresAt <= w.Now() {
		rollTrader(w, h)
	}
	w.Schedule("trader:"+id, h.Trader.ExpiresAt)
	return nil
}

func CanAcceptTrader(w World, actor PlayerID, v *Village) bool {
	h := w.Human(actor)
	if h == nil || v.OwnerID != Ref(actor) {
		return false
	}
	t := h.Trader
	return !t.Accepted && v.Buildings[BMarket] >= 1 && v.Resources.Get(t.Offer.Give) >= t.Offer.GiveAmount
}

func AcceptTrader(w World, actor PlayerID, villageID string) error {
	// The travelling trader is a fixed-rate shop, not a deal with another player, so the trading switch does not
	// close it: the market keeps working on a world that has player to player trading switched off.
	v, err := ownsVillage(w, actor, villageID, "accept-trader")
	if err != nil {
		return err
	}
	Touch(w, v)
	if !CanAcceptTrader(w, actor, v) {
		return Rule("cannotAccept", nil)
	}
	h := w.Human(actor)
	o := h.Trader.Offer
	v.Resources.Set(o.Give, v.Resources.Get(o.Give)-o.GiveAmount)
	var got Resources
	got.Set(o.Get, o.GetAmount)
	AddResourcesCapped(v, got)
	h.Trader.Accepted = true
	return nil
}
