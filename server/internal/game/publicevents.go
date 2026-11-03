package game

import "time"

// PublicEvent is a conquest anyone may see, on the site's live feed and world pages.
type PublicEvent struct {
	At          time.Time `json:"at"`
	Kind        string    `json:"kind"` // "conquest"
	WorldID     int64     `json:"worldId"`
	WorldName   string    `json:"worldName"`
	WorldKind   WorldKind `json:"worldKind,omitempty"`  // filled by the store on read
	GameNumber  int       `json:"gameNumber,omitempty"` // instant worlds; filled by the store on read
	Attacker    string    `json:"attacker"`
	Defender    string    `json:"defender"` // "" = barbarians
	AttackerBot bool      `json:"attackerBot"`
	DefenderBot bool      `json:"defenderBot"`
	X           int       `json:"x"`
	Y           int       `json:"y"`
}

// PublicKind says whether a world's battles and conquests are public.
func PublicKind(cfg *WorldConfig) bool { return cfg.Kind == KindLong || cfg.Kind == KindInstant }

// recordPublicEvent notes the time of a battle with a human on either side in a public world, and writes it to the
// public feed only when it was a conquest: who fought whom, where and with how much stays private.
func recordPublicEvent(w World, kind string, attacker, defender PlayerRef, x, y int) {
	if !PublicKind(w.Config()) || attacker == "" {
		return
	}
	a, d := w.Player(PlayerID(attacker)), w.Player(PlayerID(defender))
	if a == nil || !a.IsHuman && (d == nil || !d.IsHuman) {
		return
	}
	w.NoteBattle()
	if kind != "conquest" {
		return
	}
	e := PublicEvent{Kind: kind, Attacker: a.Name, AttackerBot: !a.IsHuman, X: x, Y: y}
	if d != nil {
		e.Defender, e.DefenderBot = d.Name, !d.IsHuman
	}
	w.AddPublicEvent(e)
}
