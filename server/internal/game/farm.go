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

// EstimateLoot: the last known state (haul plus what stayed above the hiding place) plus production since the
// last attack, capped by capacity.
func EstimateLoot(w World, actor PlayerID, target *Village) float64 {
	capacity := VillageStorageCapacity(target) * 3
	in := w.Intel(actor, target.ID)
	var since *int64
	if in != nil {
		if in.LastFarmedAt != nil {
			since = in.LastFarmedAt
		} else if in.Intel != nil {
			since = &in.Intel.At
		}
	}
	if since == nil {
		return math.Min(capacity, math.Floor(VillageStorageCapacity(target)*3*0.6))
	}
	hours := math.Max(0, float64(w.Now()-*since)) / float64(MsPerHour)
	prod := 0.0
	for _, r := range ResourceIDs {
		prod += VillageProductionPerHour(target, r, w.Config()) * hours
	}
	leftover := 0.0
	if vi := in.Intel; vi != nil && vi.LastHaul != nil && vi.LastHaulCapacity != nil && *vi.LastHaulCapacity > 0 &&
		SumResources(*vi.LastHaul) >= *vi.LastHaulCapacity {
		leftover = VillageStorageCapacity(target) * 0.5
	}
	return math.Min(capacity, math.Floor(prod+leftover))
}

func FarmTargets(w World, actor PlayerID, villageID string) []FarmTarget {
	v, h := w.Village(villageID), w.Human(actor)
	if v == nil || h == nil || v.OwnerID != Ref(actor) {
		return nil
	}
	f := h.FarmAssistant.Filters
	inbound := ownInbound(w, actor)
	var out []FarmTarget
	for _, t := range VillagesInRadius(w, v.X, v.Y, f.Radius) {
		if t.Kind != "village" || t.OwnerID != "" {
			continue
		}
		out = append(out, farmTargetFor(w, actor, v, t, inbound))
	}
	return out
}

// farmTargetFor is one tile as the actor knows it from their own reports.
func farmTargetFor(w World, actor PlayerID, from *Village, t Tile, inbound map[string]bool) FarmTarget {
	in := w.Intel(actor, t.ID)
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
	if mat := w.Village(t.ID); mat != nil {
		ft.EstimatedLoot = EstimateLoot(w, actor, mat)
	}
	return ft
}

func PassesFilters(w World, actor PlayerID, t FarmTarget) bool {
	h := w.Human(actor)
	if h == nil {
		return false
	}
	f := h.FarmAssistant.Filters
	if f.HideInbound && t.Inbound {
		return false
	}
	if f.HideLosses && t.LastLosses != nil && *t.LastLosses > 0 {
		return false
	}
	if t.Wall != nil && *t.Wall > f.MaxWall {
		return false
	}
	if t.LastAttackAt != nil && float64(w.Now()-*t.LastAttackAt) < f.MinMinutesSinceAttack*float64(MsPerMinute) {
		return false
	}
	return true
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
	return SendCommand(w, actor, originID, x, y, tpl.Units, "attack", SendOpts{})
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
	return SendCommand(w, actor, originID, x, y, units, "attack", SendOpts{})
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
		if _, err := SendCommand(w, actor, villageID, t.X, t.Y, tpl, "attack", SendOpts{}); err == nil {
			sent++
		}
	}
	return sent
}

func autoFarmActive(w World, actor PlayerID, villageID string) bool {
	h, v := w.Human(actor), w.Village(villageID)
	return h != nil && v != nil && v.OwnerID == Ref(actor) && h.FarmAssistant.AutoFarmEnabled && autoFarmUnlocked(w, actor) &&
		slices.Contains(h.FarmAssistant.AutoFarmVillages, villageID)
}

func scheduleAutoFarm(w World, villageID string) {
	interval := FarmBalance.AutoFarmIntervalMs
	w.Schedule("autofarm:"+villageID, NextGridTime(w.Now(), int64(HashSeed(villageID, "autofarm"))%interval, interval))
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
