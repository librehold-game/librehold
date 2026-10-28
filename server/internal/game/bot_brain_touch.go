package game

import "errors"

// TouchVillages settles a bot's villages to now — bots are settled when they act, not on every event.
func TouchVillages(w World, botID PlayerID) {
	for _, v := range PlayerVillages(w, botID) {
		Touch(w, v)
	}
}

// BotActionError receives orders a bot planner issued that the rules refused. Planners check first, so this
// means a planner bug; tests fail on it, a running world logs it and carries on.
var BotActionError func(botID PlayerID, err error)

// act runs one bot action and records its time in the brain's last 50; an accepted action also goes to the
// world's action log as kind (an order name) with args shaped like that order's body.
func act(w World, botID PlayerID, kind string, args any, fn func() error) bool {
	if b := w.Brain(botID); b != nil {
		b.ActionLog = append(b.ActionLog, w.Now())
		if len(b.ActionLog) > 50 {
			b.ActionLog = append([]int64{}, b.ActionLog[len(b.ActionLog)-50:]...)
		}
	}
	if err := fn(); err != nil {
		if BotActionError != nil && err != errSoft {
			BotActionError(botID, err)
		}
		return false
	}
	w.LogAction(string(botID), true, kind, args)
	return true
}

type logArgs map[string]any

func sendArgs(originID string, x, y int, units UnitCounts, typ string) logArgs {
	return logArgs{"originId": originID, "x": x, "y": y, "units": units, "type": typ}
}

func villageArgs(villageID string, kv ...any) logArgs {
	a := logArgs{"villageId": villageID}
	for i := 0; i+1 < len(kv); i += 2 {
		a[kv[i].(string)] = kv[i+1]
	}
	return a
}

// actSoft is act for calls whose refusal is an ordinary outcome (ScheduleSend,
// PlanDodge, PlanSnipe — "too late", "no troops"). Only blocked orders (e.g. notOwner) are reported.
func actSoft(w World, botID PlayerID, kind string, args any, fn func() error) bool {
	return act(w, botID, kind, args, func() error {
		err := fn()
		var re *RuleError
		if errors.As(err, &re) {
			return errSoft
		}
		return err
	})
}

var errSoft = errors.New("soft refusal")

func brainOf(w World, botID PlayerID) *BotBrain {
	b := w.Brain(botID)
	if b == nil {
		panic("player " + botID + " is not a bot")
	}
	return b
}
