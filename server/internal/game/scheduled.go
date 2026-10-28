package game

// Scheduled sends: "send at" and "arrive at" orders.

func init() {
	registerHandler("sched", onScheduledEvent)
	registerHandler("schedCancel", onScheduledCancelEvent)
}

type ScheduleSendInput struct {
	OriginID       string      `json:"originId"`
	X              int         `json:"x"`
	Y              int         `json:"y"`
	Units          UnitCounts  `json:"units"`
	Type           string      `json:"type"` // attack | support
	CatapultTarget *BuildingID `json:"catapultTarget,omitempty"`
	Mode           string      `json:"mode"` // sendAt | arriveAt
	At             int64       `json:"at"`
	CancelAt       *int64      `json:"cancelAt,omitempty"`
}

// MaxScheduledSends is how many sends a player may have waiting.
const MaxScheduledSends = 100

// ScheduleSend plans a send: "send at" (SendAt = At) or "arrive at" (SendAt = At − march). The send conditions
// (troops home, target) are checked only when the troops leave — they may still be on their way back.
func ScheduleSend(w World, actor PlayerID, in ScheduleSendInput) (*ScheduledSend, error) {
	origin := w.Village(in.OriginID)
	if origin == nil || origin.OwnerID != Ref(actor) {
		return nil, NotOwner("schedule-send")
	}
	if len(w.ScheduledSendsOf(actor)) >= MaxScheduledSends {
		return nil, Rule("tooManyScheduled", map[string]any{"max": MaxScheduledSends})
	}
	travel := TravelTimeForUnits(w, origin, in.X, in.Y, in.Units)
	sendAt := in.At
	if in.Mode == "arriveAt" {
		sendAt = in.At - travel
	}
	if sendAt < w.Now() {
		return nil, Rule("tooLate", map[string]any{"ms": w.Now() - sendAt})
	}
	s := &ScheduledSend{ID: w.NewID("s"), OwnerID: actor, OriginID: in.OriginID, X: in.X, Y: in.Y, Units: in.Units,
		Type: in.Type, CatapultTarget: in.CatapultTarget, Mode: in.Mode, At: in.At, SendAt: sendAt, CancelAt: in.CancelAt}
	w.PutScheduledSend(s)
	w.Schedule("sched:"+s.ID, sendAt)
	return s, nil
}

func CancelScheduled(w World, actor PlayerID, id string) error {
	s := w.ScheduledSend(id)
	if s == nil {
		return Rule("noScheduled", map[string]any{"id": id})
	}
	if s.OwnerID != actor {
		return NotOwner("cancel-scheduled")
	}
	w.DeleteScheduledSend(id)
	w.Unschedule("sched:" + id)
	return nil
}

// onScheduledEvent sends at the planned millisecond, or tells the owner why the send did not go.
func onScheduledEvent(w World, id string) error {
	s := w.ScheduledSend(id)
	if s == nil {
		return nil
	}
	w.DeleteScheduledSend(id)
	origin := w.Village(s.OriginID)
	key, params := "news.scheduledLostVillage", map[string]any(nil)
	if origin != nil && origin.OwnerID == Ref(s.OwnerID) {
		Touch(w, origin)
		check := CheckSend(w, s.OwnerID, s.OriginID, s.X, s.Y, s.Units, s.Type)
		if len(check.Blockers) == 0 {
			c, err := SendCommand(w, s.OwnerID, s.OriginID, s.X, s.Y, s.Units, s.Type, SendOpts{CatapultTarget: s.CatapultTarget})
			if err != nil {
				return err
			}
			if s.CancelAt != nil {
				w.Schedule("schedCancel:"+c.ID, *s.CancelAt)
			}
			return nil
		}
		key, params = "news.scheduledFailed", map[string]any{"blockers": check.Blockers}
	}
	if w.Human(s.OwnerID) != nil {
		AddNewsItem(w, key, params, s.OriginID, true, s.OwnerID)
	}
	return nil
}

// onScheduledCancelEvent: a planned recall (a dodge — the troops turn back before the hit).
func onScheduledCancelEvent(w World, commandID string) error {
	if c := w.Command(commandID); c != nil && CanCancelCommand(w, c) {
		turnBack(w, c)
	}
	return nil
}
