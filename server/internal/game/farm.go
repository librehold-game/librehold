package game

import (
	"math"
	"slices"
	"sort"
)

// The farm assistant, per human: filters, templates and intel are the
// actor's own, and "inbound" means the actor's own attacks only — other players' attacks are never revealed.

func init() {
	registerHandler("autofarm", onAutoFarmEvent)
	autoFarmSync = SyncAutoFarm
}

type FarmTarget struct {
	ID               string   `json:"id"`
	X                int      `json:"x"`
	Y                int      `json:"y"`
	Distance         float64  `json:"distance"`
	Points           int      `json:"points"`
	LastAttackAt     *int64   `json:"lastAttackAt"`
	LastHaul         *float64 `json:"lastHaul"`
	LastHaulCapacity *float64 `json:"lastHaulCapacity"`
	LastLosses       *int     `json:"lastLosses"`
	Wall             *int     `json:"wall"`
	Defenders        *int     `json:"defenders"` // last garrison seen; nil = unknown
	Inbound          bool     `json:"inbound"`
	Status           string   `json:"status"` // untouched | full | partial | losses | inbound
	EstimatedLoot    float64  `json:"estimatedLoot"`
}

// ownInbound are the targets of the actor's own attacks on the way.
func ownInbound(w World, actor PlayerID) map[string]bool {
	out := map[string]bool{}
	for _, v := range PlayerVillages(w, actor) {
		for _, c := range w.CommandsHome(v.ID) {
			if c.Type == "attack" {
				out[c.TargetID] = true
			}
		}
	}
	return out
}

// EstimateLoot is what the actor may expect in a village from their own reports only: what their scout saw, or
// what their last raid left, plus production since then; the bonus is on the map for anyone. An unscouted village
// is taken as a fresh barbarian one: mines at 0, warehouse at 1. Without a scouting or a won raid it is 0, unknown.
func EstimateLoot(w World, actor PlayerID, villageID string, bonus *VillageBonus) float64 {
	return estimateLoot(w, w.Intel(actor, villageID), bonus)
}

func estimateLoot(w World, in *Intel, bonus *VillageBonus) float64 {
	if in == nil {
		return 0
	}
	known := Village{Bonus: bonus}
	known.Buildings[BStorage] = 1
	var scouted *Scouted
	if in.Intel != nil && in.Intel.Scouted != nil {
		scouted = in.Intel.Scouted
		known.Buildings = scouted.Buildings
	}
	var since int64
	start := 0.0
	switch {
	case scouted != nil && (in.LastFarmedAt == nil || scouted.At >= *in.LastFarmedAt):
		since, start = scouted.At, SumResources(scouted.Resources)
	case in.LastFarmedAt != nil:
		since = *in.LastFarmedAt
		if vi := in.Intel; vi != nil && vi.LastHaul != nil && vi.LastHaulCapacity != nil && *vi.LastHaulCapacity > 0 &&
			SumResources(*vi.LastHaul) >= *vi.LastHaulCapacity {
			start = VillageStorageCapacity(&known) * 0.5
		}
	default:
		return 0
	}
	hours := math.Max(0, float64(w.Now()-since)) / float64(MsPerHour)
	prod := 0.0
	for _, r := range ResourceIDs {
		prod += VillageProductionPerHour(&known, r, w.Config()) * hours
	}
	return math.Min(VillageStorageCapacity(&known)*3, math.Floor(start+prod))
}

func FarmTargets(w World, actor PlayerID, villageID string) []FarmTarget {
	v, h := w.Village(villageID), w.Human(actor)
	if v == nil || h == nil || v.OwnerID != Ref(actor) {
		return nil
	}
	f := h.FarmAssistant.Filters
	inbound := ownInbound(w, actor)
	var tiles []Tile
	var ids []string
	for _, t := range VillagesInRadius(w, v.X, v.Y, f.Radius) {
		if t.Kind == "village" && t.OwnerID == "" {
			tiles = append(tiles, t)
			ids = append(ids, t.ID)
		}
	}
	known := w.IntelFor(actor, ids)
	var out []FarmTarget
	for _, t := range tiles {
		out = append(out, farmTargetFor(w, v, t, known[t.ID], inbound))
	}
	return out
}

// farmTargetFor is one tile as the actor knows it from their own reports: in is their intel on it, nil when none.
func farmTargetFor(w World, from *Village, t Tile, in *Intel, inbound map[string]bool) FarmTarget {
	ft := FarmTarget{ID: t.ID, X: t.X, Y: t.Y, Distance: Distance(from.X, from.Y, t.X, t.Y), Points: t.Points,
		Inbound: inbound[t.ID], Status: "untouched"}
	if in != nil {
		ft.LastAttackAt = in.LastFarmedAt
		if vi := in.Intel; vi != nil {
			if vi.LastHaul != nil {
				ft.LastHaul = ptrTo(SumResources(*vi.LastHaul))
			}
			ft.LastHaulCapacity = vi.LastHaulCapacity
			ft.LastLosses = vi.LastLosses
			ft.Wall = ptrTo(vi.Wall)
			ft.Defenders = ptrTo(vi.Units.Total())
		}
	}
	switch {
	case ft.Inbound:
		ft.Status = "inbound"
	case ft.LastLosses != nil && *ft.LastLosses > 0:
		ft.Status = "losses"
	case ft.LastHaul != nil && ft.LastHaulCapacity != nil && *ft.LastHaulCapacity > 0:
		ft.Status = "partial"
		if *ft.LastHaul >= *ft.LastHaulCapacity {
			ft.Status = "full"
		}
	}
	ft.EstimatedLoot = estimateLoot(w, in, t.Bonus)
	return ft
}

// FilterReason is why the actor's filters hide a target: inbound | losses | wall | recent; "" when it is shown.
func FilterReason(w World, actor PlayerID, t FarmTarget) string {
	h := w.Human(actor)
	if h == nil {
		return "none"
	}
	f := h.FarmAssistant.Filters
	switch {
	case f.HideInbound && t.Inbound:
		return "inbound"
	case f.HideLosses && t.LastLosses != nil && *t.LastLosses > 0:
		return "losses"
	case t.Wall != nil && *t.Wall > f.MaxWall:
		return "wall"
	case t.LastAttackAt != nil && float64(w.Now()-*t.LastAttackAt) < f.MinMinutesSinceAttack*float64(MsPerMinute):
		return "recent"
	}
	return ""
}

func PassesFilters(w World, actor PlayerID, t FarmTarget) bool {
	return FilterReason(w, actor, t) == ""
}

// TemplateHaul is the carrying capacity of one "pack" of a template.
func TemplateHaul(u UnitCounts) float64 {
	h := 0.0
	for _, id := range UnitIDs {
		h += float64(u[id]) * UnitDefs[id].Haul
	}
	return h
}

// TemplateCUnits: as many packs of template C as the estimated loot needs (at least 1, at most what is at home).
func TemplateCUnits(w World, actor PlayerID, v *Village, estimatedLoot float64) (UnitCounts, bool) {
	h := w.Human(actor)
	if h == nil {
		return UnitCounts{}, false
	}
	base := h.FarmAssistant.Templates["C"].Units
	perSet := TemplateHaul(base)
	if perSet <= 0 {
		return UnitCounts{}, false
	}
	sets := max(1, int(math.Ceil(estimatedLoot/perSet)))
	for _, u := range UnitIDs {
		if base[u] > 0 {
			sets = min(sets, v.Units[u]/base[u])
		}
	}
	if sets <= 0 {
		return UnitCounts{}, false
	}
	var out UnitCounts
	for _, u := range UnitIDs {
		out[u] = base[u] * sets
	}
	return out, true
}

var farmTemplateKeys = []string{"A", "B", "C", "D"}

func SetFarmTemplate(w World, actor PlayerID, key string, u UnitCounts) error {
	h := w.Human(actor)
	if h == nil {
		return NotOwner("set-farm-template")
	}
	if !slices.Contains(farmTemplateKeys, key) {
		return Rule("templateKey", map[string]any{"key": key})
	}
	var clean UnitCounts
	for _, id := range UnitIDs {
		clean[id] = max(0, u[id])
	}
	h.FarmAssistant.Templates[key] = FarmTemplate{Units: clean}
	return nil
}

// SendFarmTemplate sends template `key` from originID (the server has no "active village").
func SendFarmTemplate(w World, actor PlayerID, originID, key string, x, y int) (*Command, error) {
	h := w.Human(actor)
	if h == nil {
		return nil, NotOwner("send-farm-template")
	}
	tpl, ok := h.FarmAssistant.Templates[key]
	if !ok || tpl.Units.Total() == 0 {
		return nil, Rule("emptyTemplate", map[string]any{"key": key})
	}
	return SendCommand(w, actor, originID, x, y, tpl.Units, "attack", SendOpts{Farm: true})
}

func SendTemplateC(w World, actor PlayerID, originID string, x, y int) (*Command, error) {
	v, err := ownsVillage(w, actor, originID, "send-template-c")
	if err != nil {
		return nil, err
	}
	Touch(w, v)
	loot := 0.0
	for _, ft := range FarmTargets(w, actor, originID) {
		if ft.X == x && ft.Y == y {
			loot = ft.EstimatedLoot
		}
	}
	units, ok := TemplateCUnits(w, actor, v, loot)
	if !ok {
		return nil, Rule("emptyTemplate", map[string]any{"key": "c"})
	}
	return SendCommand(w, actor, originID, x, y, units, "attack", SendOpts{Farm: true})
}

// HideFarmOnMap keeps the player's farm raids, and their ways home, off their map or brings them back.
func HideFarmOnMap(w World, actor PlayerID, hide bool) error {
	h := w.Human(actor)
	if h == nil {
		return NotOwner("hide-farm-on-map")
	}
	h.FarmAssistant.HideOnMap = hide
	return nil
}

type FarmFiltersPatch struct {
	Radius                *float64 `json:"radius,omitempty"`
	MinMinutesSinceAttack *float64 `json:"minMinutesSinceAttack,omitempty"`
	MaxWall               *int     `json:"maxWall,omitempty"`
	HideLosses            *bool    `json:"hideLosses,omitempty"`
	HideInbound           *bool    `json:"hideInbound,omitempty"`
}

func SetFarmFilters(w World, actor PlayerID, p FarmFiltersPatch) error {
	h := w.Human(actor)
	if h == nil {
		return NotOwner("set-farm-filters")
	}
	f := &h.FarmAssistant.Filters
	if p.Radius != nil {
		f.Radius = math.Max(1, math.Min(50, *p.Radius))
	}
	if p.MinMinutesSinceAttack != nil {
		f.MinMinutesSinceAttack = math.Max(0, *p.MinMinutesSinceAttack)
	}
	if p.MaxWall != nil {
		f.MaxWall = max(0, *p.MaxWall)
	}
	if p.HideLosses != nil {
		f.HideLosses = *p.HideLosses
	}
	if p.HideInbound != nil {
		f.HideInbound = *p.HideInbound
	}
	return nil
}

func autoFarmUnlocked(w World, actor PlayerID) bool {
	h, p := w.Human(actor), w.Player(actor)
	return h != nil && p != nil && (h.FarmAssistant.AutoFarmUnlocked || len(p.VillageIDs) >= FarmBalance.AutoFarmUnlockVillages)
}

// SetAutoFarm switches the auto-farm on or off; villageIDs nil keeps the list, else every id must be the actor's.
func SetAutoFarm(w World, actor PlayerID, enabled bool, villageIDs []string) error {
	h := w.Human(actor)
	if h == nil {
		return NotOwner("set-auto-farm")
	}
	if villageIDs != nil {
		for _, id := range villageIDs {
			if v := w.Village(id); v == nil || v.OwnerID != Ref(actor) {
				return NotOwner("set-auto-farm")
			}
		}
		h.FarmAssistant.AutoFarmVillages = distinct(villageIDs)
	}
	h.FarmAssistant.AutoFarmEnabled = enabled
	SyncAutoFarm(w, actor)
	return nil
}

// autoFarmSend is the action log's record of a raid the auto-farm sent: the replay sends it again from the world's
// own events, so it is not an order.
type autoFarmSend struct {
	OriginID string     `json:"originId"`
	X        int        `json:"x"`
	Y        int        `json:"y"`
	Units    UnitCounts `json:"units"`
}

// AutoFarmTick sends template A from a village to filtered targets, nearest first, while the troops last.
func AutoFarmTick(w World, actor PlayerID, villageID string) int {
	v, h := w.Village(villageID), w.Human(actor)
	if v == nil || h == nil || v.OwnerID != Ref(actor) {
		return 0
	}
	tpl := h.FarmAssistant.Templates["A"].Units
	if tpl.Total() == 0 {
		return 0
	}
	Touch(w, v)
	var targets []FarmTarget
	for _, t := range FarmTargets(w, actor, villageID) {
		if PassesFilters(w, actor, t) {
			targets = append(targets, t)
		}
	}
	sort.SliceStable(targets, func(i, j int) bool { return targets[i].Distance < targets[j].Distance })
	sent := 0
	for _, t := range targets {
		if sent >= FarmBalance.AutoFarmMaxPerTick {
			break
		}
		check := CheckSend(w, actor, villageID, t.X, t.Y, tpl, "attack")
		if len(check.Blockers) > 0 {
			if slices.ContainsFunc(check.Blockers, func(b Blocker) bool { return b.Kind() == "notEnough" }) {
				break
			}
			continue
		}
		if _, err := SendCommand(w, actor, villageID, t.X, t.Y, tpl, "attack", SendOpts{AutoFarm: true, Farm: true}); err == nil {
			sent++
			w.LogAction(ActorOf(w.Player(actor)), false, "auto-farm-send", autoFarmSend{OriginID: villageID, X: t.X, Y: t.Y, Units: tpl})
		}
	}
	return sent
}

func autoFarmActive(w World, actor PlayerID, villageID string) bool {
	h, v := w.Human(actor), w.Village(villageID)
	return h != nil && v != nil && v.OwnerID == Ref(actor) && h.FarmAssistant.AutoFarmEnabled && autoFarmUnlocked(w, actor) &&
		slices.Contains(h.FarmAssistant.AutoFarmVillages, villageID)
}

func scheduleAutoFarm(w World, villageID string) { scheduleAutoFarmTick(w, "autofarm:", villageID) }

// scheduleAutoFarmTick puts the village's next tick on the auto-farm grid (offset per village).
func scheduleAutoFarmTick(w World, prefix, villageID string) {
	interval := FarmBalance.AutoFarmIntervalMs
	w.Schedule(prefix+villageID, NextGridTime(w.Now(), int64(HashSeed(villageID, "autofarm"))%interval, interval))
}

// SyncAutoFarm reconciles a human's auto-farm events with their settings: active villages without a tick get one,
// inactive ones lose it. Called after every change that may switch it.
func SyncAutoFarm(w World, actor PlayerID) {
	h, p := w.Human(actor), w.Player(actor)
	if h == nil || p == nil {
		return
	}
	ids := slices.Clone(h.FarmAssistant.AutoFarmVillages)
	for _, id := range p.VillageIDs {
		if !slices.Contains(ids, id) {
			ids = append(ids, id)
		}
	}
	for _, id := range ids {
		key := "autofarm:" + id
		if !autoFarmActive(w, actor, id) {
			w.Unschedule(key)
		} else if !w.IsScheduled(key) {
			scheduleAutoFarm(w, id)
		}
	}
}

func onAutoFarmEvent(w World, villageID string) error {
	v := w.Village(villageID)
	if v == nil || v.OwnerID == "" {
		return nil
	}
	owner := PlayerID(v.OwnerID)
	if !autoFarmActive(w, owner, villageID) {
		return nil
	}
	AutoFarmTick(w, owner, villageID)
	scheduleAutoFarm(w, villageID)
	return nil
}
