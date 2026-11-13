package game

import (
	"math"
	"strconv"
)

// Bot strategy: roles, troop targets and conquest campaigns.

// phaseWeights: how many "tickets" building and campaigning get per style.
var phaseWeights = map[BotStyle][2]float64{ // [build, campaign]
	StyleAggressor:    {1, 3},
	StyleTurtle:       {3, 0.5},
	StyleFarmer:       {2, 1},
	StylePointBuilder: {3, 0.3},
	StyleOpportunist:  {1, 1.5},
}

// UpdateMood: the next days' mood — calm building or a campaign (1–4 days), drawn by style (the current phase at
// half weight), so one cannot tell when an aggressor strikes. After losing a village: 1–2 days of consolidation.
func UpdateMood(w World, botID PlayerID) {
	b := w.Brain(botID)
	if b == nil {
		return
	}
	rng := Mulberry32(HiddenSeed(w, "mood", botID, w.Now()))
	if b.LastSessionStats.VillagesLost > 0 {
		b.Mood = Mood{Phase: "consolidate", Until: w.Now() + BotDur(w, int64(math.Round((1+rng())*float64(MsPerDay))))}
		if w.Config().BotThinkEverySec > 0 {
			b.LastSessionStats.VillagesLost = 0
		}
		return
	}
	if w.Now() >= b.Mood.Until {
		weights := phaseWeights[b.Style]
		switch b.Mood.Phase {
		case "build":
			weights[0] *= 0.5
		case "campaign":
			weights[1] *= 0.5
		}
		phase := "build"
		if rng()*(weights[0]+weights[1]) < weights[1] {
			phase = "campaign"
		}
		b.Mood = Mood{Phase: phase, Until: w.Now() + BotDur(w, int64(math.Round((1+rng()*3)*float64(MsPerDay))))}
	}
	if b.Mood.Phase == "campaign" {
		PickCampaignTarget(w, botID)
	}
}

// ScoutParty is how many scouts one scouting send carries. It follows from the scouting rule: a party gets through only
// when it beats ScoutDefenseFactor per defending scout, so seeing a defence village's home guard (400) takes more than
// 800. A village keeps its role's scout line (TroopTargets), so an offence village can field this and still have scouts
// left for the next look, and recruitment refills what it spent.
const ScoutParty = 900

// scoutPartyFor is the scouts to commit from a village: a full party, or everything it has if that is less.
func scoutPartyFor(have int) int {
	return min(have, ScoutParty)
}

// campaignScout sends a real scouting party at a target whose defence it does not know. Scouts only: mixed into a
// real attack they are ordinary light troops, and an attack only teaches what it survives.
func campaignScout(w World, botID PlayerID, x, y int) bool {
	var best *Village
	for _, v := range PlayerVillages(w, botID) {
		if v.Units[UScout] > 0 && (best == nil || Distance(v.X, v.Y, x, y) < Distance(best.X, best.Y, x, y)) {
			best = v
		}
	}
	if best == nil {
		return false
	}
	units := UnitCounts{UScout: scoutPartyFor(best.Units[UScout])}
	if len(CheckSend(w, botID, best.ID, x, y, units, "attack").Blockers) > 0 {
		return false
	}
	at := w.Now() + SendJitter(w, botID, "scout:"+strconv.Itoa(x)+":"+strconv.Itoa(y))
	if pastTheBell(w, at) {
		return false
	}
	in := ScheduleSendInput{OriginID: best.ID, X: x, Y: y, Units: units, Type: "attack", Mode: "sendAt", At: at}
	return actSoft(w, botID, "schedule-send", in, func() error {
		_, err := ScheduleSend(w, botID, in)
		return err
	})
}

// PlanCampaign, in a campaign: every so often (4–10 h, at random so the rhythm cannot be read) an operation on
// the campaign target; a target of unknown defence is scouted first.
func PlanCampaign(w World, botID PlayerID) {
	b := w.Brain(botID)
	if b == nil || b.Mood.Phase != "campaign" {
		return
	}
	PickCampaignTarget(w, botID)
	target := b.Memory.Known[b.Mood.TargetVillage]
	if b.Mood.TargetVillage == "" || target == nil {
		return
	}
	opAt := int64(0)
	if b.Mood.OpAt != nil {
		opAt = *b.Mood.OpAt
	}
	rng := Mulberry32(HiddenSeed(w, "op", botID, opAt))
	gap := float64(BotDur(w, int64((4+rng()*6)*float64(MsPerHour))))
	if b.Mood.OpAt != nil && float64(w.Now()-*b.Mood.OpAt) < gap {
		return
	}
	// Unknown or out-of-date defence — look first. A player does not send nobles blind, and intel ages: the owner
	// rebuilds, troops come home and support arrives, so what was seen yesterday is a guess today. The operation waits
	// for the scouts only while there is a chance of learning something; intelHopeless lets a bot that keeps getting its
	// scouts caught attack on public data instead of never attacking at all.
	if intelStale(w, target) && !intelHopeless(w, target) {
		if campaignScout(w, botID, target.X, target.Y) {
			b.Mood.OpAt = ptrTo(w.Now())
			return
		}
	}
	LaunchOperation(w, botID, target)
	b.Mood.OpAt = ptrTo(w.Now())
}
