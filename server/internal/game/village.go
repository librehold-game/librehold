package game

import "strconv"

func VillageIDFor(x, y int) string {
	return "v:" + strconv.Itoa(x) + ":" + strconv.Itoa(y)
}

// CreateVillage builds a new village object. Registering it with its owner
// (Player.VillageIDs) is the caller's job, through SetOwner.
func CreateVillage(x, y int, name string, owner PlayerRef, now int64) *Village {
	return &Village{
		ID:                 VillageIDFor(x, y),
		X:                  x,
		Y:                  y,
		Name:               name,
		OwnerID:            owner,
		ResourcesUpdatedAt: now,
		SupportFrom:        map[string]UnitCounts{},
		Loyalty:            100,
		LoyaltyUpdatedAt:   now,
		BuildQueue:         []BuildJob{},
		RecruitQueues:      EmptyRecruitQueues(),
		ResearchQueue:      []ResearchJob{},
	}
}
