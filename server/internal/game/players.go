package game

import (
	"fmt"
	"slices"
)

// Players and village ownership — the one place that knows how ownership is stored.

func IsBarbarianVillage(v *Village) bool { return v.OwnerID == "" }

func OwnerOf(w World, v *Village) *Player {
	if v.OwnerID == "" {
		return nil
	}
	return w.Player(PlayerID(v.OwnerID))
}

// PlayerVillages returns the stored villages of a player, in the order of Player.VillageIDs.
func PlayerVillages(w World, id PlayerID) []*Village {
	p := w.Player(id)
	if p == nil {
		return nil
	}
	out := make([]*Village, 0, len(p.VillageIDs))
	for _, vid := range p.VillageIDs {
		if v := w.Village(vid); v != nil {
			out = append(out, v)
		}
	}
	return out
}

// SetOwner is the only write of ownership: it moves the village and keeps both players' VillageIDs in step.
func SetOwner(w World, v *Village, owner PlayerRef) {
	if v.OwnerID != "" {
		if prev := w.Player(PlayerID(v.OwnerID)); prev != nil {
			prev.VillageIDs = slices.DeleteFunc(prev.VillageIDs, func(id string) bool { return id == v.ID })
		}
	}
	v.OwnerID = owner
	if owner != "" {
		next := w.Player(PlayerID(owner))
		if next == nil {
			panic(fmt.Sprintf("no player %s", owner))
		}
		if !slices.Contains(next.VillageIDs, v.ID) {
			next.VillageIDs = append(next.VillageIDs, v.ID)
		}
	}
}

func IsProtected(w World, id PlayerID) bool {
	p := w.Player(id)
	return p != nil && w.Now() < p.ProtectedUntil
}

// BuildQueueSlotsFor is the build queue size of a player (barbarians, p == nil, do not build).
func BuildQueueSlotsFor(w World, p *Player) int {
	if p == nil {
		return 0
	}
	return p.BuildQueueSlots
}

// PlayerPoints is the sum of a player's village points.
func PlayerPoints(w World, id PlayerID) int {
	return w.PointsByOwner()[id]
}

// ownsVillage resolves a village for an order and checks the actor owns it.
func ownsVillage(w World, actor PlayerID, villageID, op string) (*Village, error) {
	v := w.Village(villageID)
	if v == nil {
		return nil, Rule("noVillage", map[string]any{"id": villageID})
	}
	if v.OwnerID != Ref(actor) {
		return nil, NotOwner(op)
	}
	return v, nil
}
