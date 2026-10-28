package app

import (
	"maps"
	"slices"

	"librehold/internal/game"
)

const (
	TopicState   = "state"
	TopicReports = "reports"
	TopicMap     = "map"
	TopicRanking = "ranking"
	TopicNews    = "news"
)

// MaxUpdateVillages caps Update.VillageIDs so a notification fits Postgres' payload limit; past it the ids
// are dropped and the whole map counts as changed.
const MaxUpdateVillages = 400

type VillageChange struct {
	ID                 string
	OldOwner, NewOwner game.PlayerRef
	OnMap              bool // new, renamed or changed owner
}

// Changes is what one committed unit of work changed, as an adapter sees it.
type Changes struct {
	Villages         []VillageChange
	CommandOwners    []game.PlayerRef // owners of the home and target villages of new, changed or deleted commands
	Humans           []game.PlayerID  // changed human states
	ReportRecipients []game.PlayerID
	Ranking          bool // a stats event ran
	News             bool
	World            bool             // the world itself changed: paused, resumed, settings, status
	Viewers          []game.PlayerRef // players who began or stopped seeing an army pass
}

// Updates turns the changes into one update per affected human and one world-wide update, in a stable order.
func (c Changes) Updates(worldID int64) []Update {
	players := map[game.PlayerID]map[string]bool{}
	add := func(owner game.PlayerRef, topic string) {
		id := game.PlayerID(owner)
		if !game.IsHumanID(id) {
			return
		}
		if players[id] == nil {
			players[id] = map[string]bool{}
		}
		players[id][topic] = true
	}
	world := map[string]bool{}
	var mapIDs []string
	for _, v := range c.Villages {
		add(v.OldOwner, TopicState)
		add(v.NewOwner, TopicState)
		if v.OnMap {
			world[TopicMap] = true
			mapIDs = append(mapIDs, v.ID)
		}
	}
	for _, o := range c.CommandOwners {
		add(o, TopicState)
	}
	for _, h := range c.Humans {
		add(game.Ref(h), TopicState)
	}
	for _, v := range c.Viewers {
		add(v, TopicState)
	}
	for _, r := range c.ReportRecipients {
		add(game.Ref(r), TopicReports)
	}
	if c.Ranking {
		world[TopicRanking] = true
	}
	if c.News {
		world[TopicNews] = true
	}
	if c.World {
		world[TopicState] = true
	}
	var out []Update
	for _, id := range slices.Sorted(maps.Keys(players)) {
		out = append(out, Update{WorldID: worldID, PlayerID: id, Topics: slices.Sorted(maps.Keys(players[id]))})
	}
	if len(world) > 0 {
		slices.Sort(mapIDs)
		mapIDs = slices.Compact(mapIDs)
		if len(mapIDs) > MaxUpdateVillages {
			mapIDs = nil
		}
		out = append(out, Update{WorldID: worldID, Topics: slices.Sorted(maps.Keys(world)), VillageIDs: mapIDs})
	}
	return out
}
