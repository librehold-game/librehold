package game

import "time"

// PublicEvent is a battle or conquest anyone may see, on the site's live feed and world pages.
type PublicEvent struct {
	At          time.Time `json:"at"`
	Kind        string    `json:"kind"` // "conquest" | "battle"
	WorldID     int64     `json:"worldId"`
	WorldName   string    `json:"worldName"`
	WorldKind   WorldKind `json:"worldKind,omitempty"`  // filled by the store on read
	GameNumber  int       `json:"gameNumber,omitempty"` // instant worlds; filled by the store on read
	Attacker    string    `json:"attacker"`
	Defender    string    `json:"defender"` // "" = barbarians
	AttackerBot bool      `json:"attackerBot"`
	DefenderBot bool      `json:"defenderBot"`
	Troops      int       `json:"troops"`
	X           int       `json:"x"`
	Y           int       `json:"y"`
}

// LargeBattleTroops is the smallest battle, in troops on both sides, the public feed shows.
const LargeBattleTroops = 100

// PublicKind says whether a world's battles and conquests are public.
func PublicKind(cfg *WorldConfig) bool { return cfg.Kind == KindLong || cfg.Kind == KindInstant }

// recordPublicEvent writes a battle or conquest with a human on either side to the public feed of a public world.
func recordPublicEvent(w World, kind string, attacker, defender PlayerRef, troops, x, y int) {
	if !PublicKind(w.Config()) || attacker == "" {
		return
	}
	a, d := w.Player(PlayerID(attacker)), w.Player(PlayerID(defender))
	if a == nil || !a.IsHuman && (d == nil || !d.IsHuman) {
		return
	}
	e := PublicEvent{Kind: kind, Attacker: a.Name, AttackerBot: !a.IsHuman, Troops: troops, X: x, Y: y}
	if d != nil {
		e.Defender, e.DefenderBot = d.Name, !d.IsHuman
	}
	w.AddPublicEvent(e)
}

// ownerOf is the owner of a village; "" for barbarians or a village that is gone.
func ownerOf(w World, villageID string) PlayerRef {
	if v := w.Village(villageID); v != nil {
		return v.OwnerID
	}
	return ""
}
