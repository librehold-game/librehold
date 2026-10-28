package game

import (
	"regexp"
	"strconv"
)

var villageIDPattern = regexp.MustCompile(`^v:(\d+):(\d+)$`)

// ReportVillage is a village as a report remembers it: its name (empty = the default name, worded by the client)
// and its owner at the time of the report.
type ReportVillage struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	X          int      `json:"x"`
	Y          int      `json:"y"`
	OwnerID    PlayerID `json:"ownerId,omitempty"`
	OwnerName  string   `json:"ownerName,omitempty"`
	OwnerIsBot bool     `json:"ownerIsBot,omitempty"`
}

// ReportVillageOf snapshots a stored village; an unstored field keeps only its coordinates.
func ReportVillageOf(w World, id string) ReportVillage {
	if v := w.Village(id); v != nil {
		rv := ReportVillage{ID: id, Name: v.Name, X: v.X, Y: v.Y}
		if v.OwnerID != "" {
			rv.OwnerID = PlayerID(v.OwnerID)
			if p := w.Player(rv.OwnerID); p != nil {
				rv.OwnerName, rv.OwnerIsBot = p.Name, !p.IsHuman
			}
		}
		return rv
	}
	rv := ReportVillage{ID: id}
	if m := villageIDPattern.FindStringSubmatch(id); m != nil {
		rv.X, _ = strconv.Atoi(m[1])
		rv.Y, _ = strconv.Atoi(m[2])
	}
	return rv
}
