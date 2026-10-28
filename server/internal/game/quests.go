package game

import "slices"

// Quests, per human.

// ActiveQuests are quests whose requirements are completed and which are not completed themselves.
func ActiveQuests(w World, humanID PlayerID) []QuestDef {
	h := w.Human(humanID)
	if h == nil {
		return nil
	}
	var out []QuestDef
	for _, q := range Quests {
		if slices.Contains(h.Quests.Completed, q.ID) {
			continue
		}
		ready := true
		for _, r := range q.Requires {
			ready = ready && slices.Contains(h.Quests.Completed, r)
		}
		if ready {
			out = append(out, q)
		}
	}
	return out
}

// QuestProgress is each active quest's bar value (0..1); quests without a bar are absent.
func QuestProgress(w World, humanID PlayerID) map[string]float64 {
	if w.Human(humanID) == nil {
		return nil
	}
	out := map[string]float64{}
	for _, q := range ActiveQuests(w, humanID) {
		if q.Progress != nil {
			out[q.ID] = q.Progress(w, humanID)
		}
	}
	return out
}

func ClaimableQuests(w World, humanID PlayerID) []QuestDef {
	h := w.Human(humanID)
	if h == nil {
		return nil
	}
	var out []QuestDef
	for _, id := range h.Quests.Completed {
		if q := QuestByID[id]; q != nil && !slices.Contains(h.Quests.Claimed, id) {
			out = append(out, *q)
		}
	}
	return out
}

// SettleQuests marks the active quests whose conditions hold as completed. It does not touch resources (the
// player claims rewards), so when it runs does not affect the simulation.
func SettleQuests(w World, humanID PlayerID) {
	h := w.Human(humanID)
	if h == nil {
		return
	}
	for guard := 0; guard <= len(Quests); guard++ {
		var met []string
		for _, q := range ActiveQuests(w, humanID) {
			if q.Condition(w, humanID) {
				met = append(met, q.ID)
			}
		}
		if len(met) == 0 {
			break
		}
		h.Quests.Completed = append(h.Quests.Completed, met...)
	}
	active := []string{}
	for _, q := range ActiveQuests(w, humanID) {
		active = append(active, q.ID)
	}
	h.Quests.Active = active
}

type ClaimResult struct {
	Lost bool `json:"lost"` // some reward resources did not fit in the storage
}

// ClaimQuest takes a reward into one of the actor's villages. Resources above the storage are lost.
func ClaimQuest(w World, actor PlayerID, questID, villageID string) (ClaimResult, error) {
	h := w.Human(actor)
	q := QuestByID[questID]
	if h == nil || q == nil {
		return ClaimResult{}, Rule("unknownQuest", nil)
	}
	if !slices.Contains(h.Quests.Completed, questID) {
		return ClaimResult{}, Rule("questNotDone", nil)
	}
	if slices.Contains(h.Quests.Claimed, questID) {
		return ClaimResult{}, Rule("questClaimed", nil)
	}
	v, err := ownsVillage(w, actor, villageID, "claim-quest")
	if err != nil {
		return ClaimResult{}, err
	}
	Touch(w, v)
	lost := false
	if rs := q.Reward.Resources; rs != nil {
		added := AddResourcesCapped(v, *rs)
		lost = added.Wood < rs.Wood || added.Clay < rs.Clay || added.Iron < rs.Iron
	}
	if q.Reward.BuildQueueSlot {
		p := w.Player(actor)
		p.BuildQueueSlots = min(int(WorldLimits.BuildQueueSlots.Max), p.BuildQueueSlots+1)
	}
	if q.Reward.AutoFarm {
		h.FarmAssistant.AutoFarmUnlocked = true
		SyncAutoFarm(w, actor)
	}
	h.Quests.Claimed = append(h.Quests.Claimed, questID)
	return ClaimResult{Lost: lost}, nil
}
