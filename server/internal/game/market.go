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

func MerchantsTotal(v *Village) int { return v.Buildings[BMarket] }

func MerchantsInTransit(w World, villageID string) int {
	n := 0
	for _, c := range w.CommandsHome(villageID) {
		if c.Type == "trade" {
			n += c.Merchants
		}
	}
	return n
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
		Merchants: check.Merchants, DepartedAt: w.Now(), ArrivesAt: w.Now() + check.TravelMs}
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
