package game

import (
	"slices"
	"strings"
)

// Build plans and templates, and the village and report management orders.

// How much a player may keep: a plan of a village, templates, groups. Each request may carry a megabyte, and
// without a cap it would add to the stored state for good.
const (
	maxPlanSteps      = 200
	MaxBuildTemplates = 20
	MaxGroups         = 50
)

func init() {
	registerHandler("plan", onPlanEvent)
	planFeeder = func(w World, villageID string) { FeedBuildPlan(w, villageID) }
}

// FeedBuildPlan sends plan steps to the queue while there are free slots and resources. Impossible steps (max
// level, requirements) are dropped.
func FeedBuildPlan(w World, villageID string) int {
	v := w.Village(villageID)
	if v == nil || len(v.BuildPlan) == 0 || v.OwnerID == "" {
		return 0
	}
	owner := PlayerID(v.OwnerID)
	Touch(w, v)
	added := 0
	for guard := 0; len(v.BuildPlan) > 0 && guard < 50; guard++ {
		next := v.BuildPlan[0]
		kinds := blockerKinds(CheckBuild(w, v, next).Blockers)
		if slices.Contains(kinds, "queueFull") {
			break
		}
		if slices.Contains(kinds, "maxLevel") || slices.Contains(kinds, "requirements") {
			v.BuildPlan = v.BuildPlan[1:]
			continue
		}
		if len(kinds) > 0 {
			break // resources / population — wait
		}
		if _, err := EnqueueBuild(w, owner, villageID, next); err != nil {
			break
		}
		v.BuildPlan = v.BuildPlan[1:]
		added++
	}
	if len(v.BuildPlan) == 0 {
		v.BuildPlan = nil
	}
	return added
}

func blockerKinds(bs []Blocker) []string {
	out := make([]string, len(bs))
	for i, b := range bs {
		out[i] = b.Kind()
	}
	return out
}

func SetBuildPlan(w World, actor PlayerID, villageID string, steps []BuildingID) error {
	v, err := ownsVillage(w, actor, villageID, "set-build-plan")
	if err != nil {
		return err
	}
	if len(v.BuildPlan)+len(steps) > maxPlanSteps {
		return Rule("planTooLong", map[string]any{"max": maxPlanSteps})
	}
	v.BuildPlan = append(slices.Clone(v.BuildPlan), steps...)
	FeedBuildPlan(w, villageID)
	if len(v.BuildPlan) > 0 {
		schedulePlan(w, villageID)
	}
	return nil
}

func ClearBuildPlan(w World, actor PlayerID, villageID string) error {
	v, err := ownsVillage(w, actor, villageID, "clear-build-plan")
	if err != nil {
		return err
	}
	v.BuildPlan = nil
	w.Unschedule("plan:" + villageID)
	return nil
}

// schedulePlan: a plan tries to send its next step every PlanBalance.FeedIntervalMs (grid offset per village).
func schedulePlan(w World, villageID string) {
	interval := PlanBalance.FeedIntervalMs
	w.Schedule("plan:"+villageID, NextGridTime(w.Now(), int64(HashSeed(villageID, "plan"))%interval, interval))
}

func onPlanEvent(w World, villageID string) error {
	FeedBuildPlan(w, villageID)
	if v := w.Village(villageID); v != nil && len(v.BuildPlan) > 0 {
		schedulePlan(w, villageID)
	}
	return nil
}

// ---------- build templates ----------

func ApplyBuildTemplate(w World, actor PlayerID, villageID, templateID string) error {
	h := w.Human(actor)
	if h == nil {
		return NotOwner("apply-build-template")
	}
	i := slices.IndexFunc(h.BuildTemplates, func(t BuildTemplate) bool { return t.ID == templateID })
	if i < 0 {
		return Rule("noTemplate", map[string]any{"id": templateID})
	}
	return SetBuildPlan(w, actor, villageID, h.BuildTemplates[i].Steps)
}

func SaveBuildTemplate(w World, actor PlayerID, tpl BuildTemplate) error {
	h := w.Human(actor)
	if h == nil {
		return NotOwner("save-build-template")
	}
	tpl.Name = strings.TrimSpace(tpl.Name)
	if tpl.Name == "" || len([]rune(tpl.Name)) > 40 || len(tpl.Steps) > maxPlanSteps {
		return Rule("templateInvalid", map[string]any{"maxName": 40, "maxSteps": maxPlanSteps})
	}
	if tpl.ID == "" {
		tpl.ID = w.NewID("tpl")
	}
	tpl.Steps = slices.Clone(tpl.Steps)
	if i := slices.IndexFunc(h.BuildTemplates, func(t BuildTemplate) bool { return t.ID == tpl.ID }); i >= 0 {
		h.BuildTemplates[i] = tpl
	} else if len(h.BuildTemplates) >= MaxBuildTemplates {
		return Rule("tooManyTemplates", map[string]any{"max": MaxBuildTemplates})
	} else {
		h.BuildTemplates = append(h.BuildTemplates, tpl)
	}
	return nil
}

func DeleteBuildTemplate(w World, actor PlayerID, id string) error {
	h := w.Human(actor)
	if h == nil {
		return NotOwner("delete-build-template")
	}
	h.BuildTemplates = slices.DeleteFunc(h.BuildTemplates, func(t BuildTemplate) bool { return t.ID == id })
	return nil
}

// ---------- village groups ----------

func SaveGroup(w World, actor PlayerID, g VillageGroup) error {
	h := w.Human(actor)
	if h == nil {
		return NotOwner("save-group")
	}
	g.Name = strings.TrimSpace(g.Name)
	if g.Name == "" || len([]rune(g.Name)) > 32 {
		return Rule("groupName", map[string]any{"max": 32})
	}
	for _, id := range g.VillageIDs {
		if v := w.Village(id); v == nil || v.OwnerID != Ref(actor) {
			return NotOwner("save-group")
		}
	}
	if g.ID == "" {
		g.ID = w.NewID("grp")
	}
	g.VillageIDs = distinct(g.VillageIDs)
	if i := slices.IndexFunc(h.Groups, func(x VillageGroup) bool { return x.ID == g.ID }); i >= 0 {
		h.Groups[i] = g
	} else if len(h.Groups) >= MaxGroups {
		return Rule("tooManyGroups", map[string]any{"max": MaxGroups})
	} else {
		h.Groups = append(h.Groups, g)
	}
	return nil
}

func DeleteGroup(w World, actor PlayerID, id string) error {
	h := w.Human(actor)
	if h == nil {
		return NotOwner("delete-group")
	}
	h.Groups = slices.DeleteFunc(h.Groups, func(g VillageGroup) bool { return g.ID == id })
	return nil
}

// ---------- mass actions ----------

// MassBuild queues a building in each village; refused ones (foreign, blocked) are skipped.
func MassBuild(w World, actor PlayerID, villageIDs []string, b BuildingID) int {
	n := 0
	for _, id := range villageIDs {
		if _, err := EnqueueBuild(w, actor, id, b); err == nil {
			n++
		}
	}
	return n
}

// MassRecruit tops each village up to `upTo` units (present plus queued), as far as it can afford.
func MassRecruit(w World, actor PlayerID, villageIDs []string, u UnitID, upTo int) int {
	n := 0
	for _, id := range villageIDs {
		v := w.Village(id)
		if v == nil || v.OwnerID != Ref(actor) {
			continue
		}
		Touch(w, v)
		want := min(upTo-UnitCountIncludingQueued(v, u), MaxRecruitable(w, v, u))
		if want <= 0 {
			continue
		}
		if _, err := EnqueueRecruit(w, actor, id, u, want); err == nil {
			n += want
		}
	}
	return n
}

// ---------- villages and reports ----------

func RenameVillage(w World, actor PlayerID, villageID, name string) error {
	v, err := ownsVillage(w, actor, villageID, "rename-village")
	if err != nil {
		return err
	}
	name = strings.TrimSpace(name)
	if l := len([]rune(name)); l < 1 || l > 32 {
		return Rule("nameLength", map[string]any{"max": 32})
	}
	v.Name = name
	return nil
}

const allReports = int64(1) << 62

func MarkReportsRead(w World, actor PlayerID, ids []string, read bool) error {
	w.SetReportRead(actor, ids, read)
	return nil
}

func reportIDs(w World, actor PlayerID, keep func(*Report) bool) []string {
	var ids []string
	for _, r := range w.Reports(actor, allReports, 0) {
		if keep(r) {
			ids = append(ids, r.ID)
		}
	}
	return ids
}

func MarkAllReportsRead(w World, actor PlayerID) error {
	w.SetReportRead(actor, reportIDs(w, actor, func(r *Report) bool { return !r.Read }), true)
	return nil
}

func DeleteReport(w World, actor PlayerID, id string) error {
	w.DeleteReports(actor, []string{id})
	return nil
}

func DeleteReadReports(w World, actor PlayerID) error {
	w.DeleteReports(actor, reportIDs(w, actor, func(r *Report) bool { return r.Read }))
	return nil
}

// distinct is ids without repeats, in their first order.
func distinct(ids []string) []string {
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		if !slices.Contains(out, id) {
			out = append(out, id)
		}
	}
	return out
}
