package game

import "math"

// Troop movements: sending, recalling and arrivals. Support goes to any player's village.

func init() { registerHandler("cmd", onCommandEvent) }

// SlowestSpeed is the slowest unit's minutes per tile.
func SlowestSpeed(u UnitCounts) float64 {
	s := 0.0
	for _, id := range UnitIDs {
		if u[id] > 0 {
			s = math.Max(s, UnitDefs[id].Speed)
		}
	}
	return s
}

// TravelTimeMs is distance × the slowest unit's minutes per tile, shortened only by unit speed (not world speed).
func TravelTimeMs(dist, speedMinPerTile, unitSpeed float64) int64 {
	return int64(math.Round((dist * speedMinPerTile * float64(MsPerMinute)) / unitSpeed))
}

func TravelTimeForUnits(w World, origin *Village, tx, ty int, u UnitCounts) int64 {
	return TravelTimeMs(Distance(origin.X, origin.Y, tx, ty), SlowestSpeed(u), w.Config().UnitSpeed)
}

type SendCheck struct {
	Blockers       []Blocker
	Warnings       []Blocker
	Distance       float64
	TravelMs       int64
	ArrivesAt      int64
	IsScoutMission bool
}

type SendOpts struct {
	CatapultTarget *BuildingID
}

// CheckSend says whether troops can leave originID for (tx, ty) as an attack or support (no mutation).
func CheckSend(w World, actor PlayerID, originID string, tx, ty int, u UnitCounts, typ string) SendCheck {
	origin := w.Village(originID)
	if origin == nil || origin.OwnerID != Ref(actor) {
		return SendCheck{Blockers: []Blocker{{"kind": "notOwner"}}}
	}
	var blockers, warnings []Blocker
	n := u.Total()
	if n <= 0 {
		blockers = append(blockers, Blocker{"kind": "noUnits"})
	}
	for _, id := range UnitIDs {
		if want := u[id]; want < 0 || want > origin.Units[id] {
			blockers = append(blockers, Blocker{"kind": "notEnough", "unit": id, "have": origin.Units[id]})
		}
	}
	if origin.X == tx && origin.Y == ty {
		blockers = append(blockers, Blocker{"kind": "self"})
	}
	targetExists := VillageExistsAt(w, tx, ty)
	if !targetExists {
		blockers = append(blockers, Blocker{"kind": "noTarget"})
	}
	isScout := n > 0 && u[UScout] == n
	if typ == "attack" && !isScout {
		power := 0.0
		for _, id := range UnitIDs {
			power += float64(u[id]) * UnitDefs[id].Attack
		}
		if power <= 0 {
			blockers = append(blockers, Blocker{"kind": "noAttackPower"})
		}
		if u[UNoble] > 0 && n-u[UNoble] == 0 {
			warnings = append(warnings, Blocker{"kind": "nobleUnescorted"})
		}
	}
	target := w.Village(VillageIDFor(tx, ty))
	if typ == "support" && targetExists && (target == nil || target.OwnerID == "") {
		blockers = append(blockers, Blocker{"kind": "supportTargetBarbarian"})
	}
	if typ == "attack" && target != nil && target.OwnerID != "" && target.OwnerID == origin.OwnerID && target.ID != origin.ID {
		blockers = append(blockers, Blocker{"kind": "attackOwnVillage"})
	}
	// Beginner protection: protected players cannot be attacked, and attack only barbarians themselves.
	if typ == "attack" && target != nil && target.OwnerID != "" && target.OwnerID != origin.OwnerID {
		if IsProtected(w, PlayerID(target.OwnerID)) {
			blockers = append(blockers, Blocker{"kind": "targetProtected", "until": w.Player(PlayerID(target.OwnerID)).ProtectedUntil})
		}
		if IsProtected(w, actor) {
			blockers = append(blockers, Blocker{"kind": "selfProtected"})
		}
	}
	dist := Distance(origin.X, origin.Y, tx, ty)
	if u[UNoble] > 0 && dist > ConquestBalance.NobleMaxDistance {
		blockers = append(blockers, Blocker{"kind": "nobleTooFar", "max": ConquestBalance.NobleMaxDistance})
	}
	travel := TravelTimeMs(dist, SlowestSpeed(u), w.Config().UnitSpeed)
	return SendCheck{blockers, warnings, dist, travel, w.Now() + travel, isScout}
}

// phoneCheck lets a defending bot glance at the game when an attack is sent at it (set by bot sessions).
var phoneCheck func(w World, botID PlayerID, commandID string, arrivesAt int64)

func SendCommand(w World, actor PlayerID, originID string, tx, ty int, u UnitCounts, typ string, opts SendOpts) (_ *Command, err error) {
	defer func() {
		if err == nil {
			countAttack(w, typ)
		}
	}()
	if origin := w.Village(originID); origin != nil && origin.OwnerID == Ref(actor) {
		Touch(w, origin)
	}
	check := CheckSend(w, actor, originID, tx, ty, u, typ)
	if len(check.Blockers) > 0 {
		return nil, Blocked("send", check.Blockers...)
	}
	origin := w.Village(originID)
	target := EnsureVillage(w, tx, ty)
	if target == nil {
		return nil, Rule("noTarget", nil)
	}
	var sent UnitCounts
	for _, id := range UnitIDs {
		if n := u[id]; n > 0 {
			origin.Units[id] -= n
			sent[id] = n
		}
	}
	c := &Command{ID: w.NewID("c"), Type: typ, OriginID: originID, TargetID: target.ID, Units: sent,
		DepartedAt: w.Now(), ArrivesAt: w.Now() + check.TravelMs}
	if opts.CatapultTarget != nil && sent[UCatapult] > 0 {
		ct := *opts.CatapultTarget
		c.CatapultTarget = &ct
	}
	AddCommand(w, c)
	if typ == "attack" && target.OwnerID != "" && target.OwnerID != origin.OwnerID {
		defender := PlayerID(target.OwnerID)
		if b := w.Brain(defender); b != nil {
			b.LastSessionStats.IncomingAttacks++
			if phoneCheck != nil {
				phoneCheck(w, defender, c.ID, c.ArrivesAt)
			}
		}
	}
	if typ == "attack" {
		countHumanAttack(w, actor, sent)
	} else if h := w.Human(actor); h != nil {
		h.Stats.SupportsSent++
	}
	return c, nil
}

// countHumanAttack records an attack (or a scouting run, when only scouts went) in a human's stats.
func countHumanAttack(w World, actor PlayerID, sent UnitCounts) {
	h := w.Human(actor)
	if h == nil {
		return
	}
	if sent[UScout] == sent.Total() {
		h.Stats.ScoutsSent++
	} else {
		h.Stats.AttacksSent++
	}
	day := w.Now() / MsPerDay
	if h.Stats.AttacksToday.Day != day {
		h.Stats.AttacksToday = DayCount{Day: day}
	}
	h.Stats.AttacksToday.Count++
}

func CanCancelCommand(w World, c *Command) bool {
	if c.Type == "return" || c.Type == "intercept" {
		return false
	}
	// An army hit in the field may turn back at any time before it arrives.
	if c.FieldHit && (c.Type == "attack" || c.Type == "support") {
		return w.Now() < c.ArrivesAt
	}
	window := CommandBalance.CancelWindowMs
	if c.Type == "trade" {
		window = CommandBalance.TradeCancelWindowMs
	}
	return w.Now()-c.DepartedAt < min(window, c.ArrivesAt-c.DepartedAt)
}

// CancelCommand turns the troops round: they come back as long as they had been marching.
func CancelCommand(w World, actor PlayerID, commandID string) error {
	c := w.Command(commandID)
	if c == nil {
		return Rule("noCommand", map[string]any{"id": commandID})
	}
	if home := w.Village(CommandHome(c)); home == nil || home.OwnerID != Ref(actor) {
		return NotOwner("cancel-command")
	}
	if !CanCancelCommand(w, c) {
		return Rule("cannotCancel", nil)
	}
	turnBack(w, c)
	return nil
}

func turnBack(w World, c *Command) {
	elapsed := w.Now() - c.DepartedAt
	c.OriginID, c.TargetID = c.TargetID, c.OriginID
	c.Type = "return"
	c.DepartedAt = w.Now()
	c.ArrivesAt = w.Now() + elapsed
	c.CatapultTarget = nil
	w.Schedule("cmd:"+c.ID, c.ArrivesAt)
	scheduleSight(w, c)
}

// WithdrawSupport sends support stationed in hostID back to homeID (all of it with u == nil). The owner of the
// troops withdraws them, or the host sends them back; anyone else gets notOwner. Nothing to take → (nil, nil).
func WithdrawSupport(w World, actor PlayerID, hostID, homeID string, u *UnitCounts) (*Command, error) {
	host, home := w.Village(hostID), w.Village(homeID)
	if host == nil || home == nil {
		return nil, Rule("noVillage", nil)
	}
	if home.OwnerID != Ref(actor) && host.OwnerID != Ref(actor) {
		return nil, NotOwner("withdraw-support")
	}
	stationed, ok := host.SupportFrom[homeID]
	if !ok {
		return nil, nil
	}
	var take UnitCounts
	for _, id := range UnitIDs {
		want := stationed[id]
		if u != nil {
			want = min(stationed[id], u[id])
		}
		if want > 0 {
			take[id] = want
			stationed[id] -= want
		}
	}
	if take.Total() == 0 {
		return nil, nil
	}
	if stationed.Total() == 0 {
		delete(host.SupportFrom, homeID)
	} else {
		host.SupportFrom[homeID] = stationed
	}
	return CreateReturn(w, hostID, homeID, take, nil), nil
}

// AddCommand puts a command in the world and schedules its arrival. Every new command goes through here.
func AddCommand(w World, c *Command) {
	w.PutCommand(c)
	w.Schedule("cmd:"+c.ID, c.ArrivesAt)
	scheduleSight(w, c)
}

func CreateReturn(w World, fromID, homeID string, units UnitCounts, haul *Resources) *Command {
	from, home := w.Village(fromID), w.Village(homeID)
	travel := TravelTimeMs(Distance(from.X, from.Y, home.X, home.Y), SlowestSpeed(units), w.Config().UnitSpeed)
	c := &Command{ID: w.NewID("c"), Type: "return", OriginID: fromID, TargetID: homeID, Units: units,
		DepartedAt: w.Now(), ArrivesAt: w.Now() + max(1, travel)}
	if haul != nil {
		h := *haul
		c.Haul = &h
	}
	AddCommand(w, c)
	return c
}

// attackArrival and tradeArrival resolve those arrivals (set by battle.go and market.go).
var (
	attackArrival = func(w World, c *Command, target *Village) error {
		return Rule("notPorted", map[string]any{"what": "attack"})
	}
	tradeArrival = func(w World, c *Command, target *Village) error {
		return Rule("notPorted", map[string]any{"what": "trade"})
	}
)

// onCommandEvent: a command arrives (a cancelled one already has its new time and event).
func onCommandEvent(w World, commandID string) error {
	c := w.Command(commandID)
	if c == nil {
		return nil
	}
	if c.ArrivesAt > w.Now() {
		w.Schedule("cmd:"+c.ID, c.ArrivesAt)
		return nil
	}
	w.DeleteCommand(c.ID)
	target := w.Village(c.TargetID)
	if target == nil {
		return nil
	}
	switch c.Type {
	case "attack":
		return attackArrival(w, c, target)
	case "support":
		origin := w.Village(c.OriginID)
		if origin == nil || origin.OwnerID == "" || target.OwnerID == "" || c.OriginID == c.TargetID {
			if origin != nil {
				CreateReturn(w, c.TargetID, c.OriginID, c.Units, nil)
			}
			return nil
		}
		slot := target.SupportFrom[c.OriginID]
		for _, id := range UnitIDs {
			slot[id] += c.Units[id]
		}
		target.SupportFrom[c.OriginID] = slot
	case "return":
		Touch(w, target)
		for _, id := range UnitIDs {
			target.Units[id] += c.Units[id]
		}
		if c.Haul != nil {
			AddResourcesCapped(target, *c.Haul)
		}
	case "trade":
		return tradeArrival(w, c, target)
	}
	return nil
}
