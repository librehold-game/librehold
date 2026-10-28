package game

import "sort"

// Archive is what an instant world leaves behind when it ends: a summary and the final leaderboard.
type Archive struct {
	Summary ArchiveSummary `json:"summary"`
	Rows    []LeaderRow    `json:"rows"`
}

type ArchiveSummary struct {
	EndReason          string   `json:"endReason"`
	WinnerID           PlayerID `json:"winnerId"`
	Humans             int      `json:"humans"`
	Guests             int      `json:"guests"` // of the humans, those still guests when the world ended
	Bots               int      `json:"bots"`
	ConquestsHumans    int      `json:"conquestsHumans"`
	ConquestsBots      int      `json:"conquestsBots"`
	AttacksSent        int      `json:"attacksSent"`
	HumanUnitsKilled   int64    `json:"humanUnitsKilled"`
	HumanUnitsLost     int64    `json:"humanUnitsLost"`
	LargestBattleUnits int      `json:"largestBattleUnits"`
}

// LeaderRow is one player's line on the final leaderboard; AccountID is 0 for bots and departed players. Key is a
// human's player key: it outlives the account, so totals keep counting a deleted player, and it is never public.
type LeaderRow struct {
	Rank      int      `json:"rank"`
	PlayerID  PlayerID `json:"playerId"`
	Name      string   `json:"name"`
	IsBot     bool     `json:"isBot"`
	AccountID int64    `json:"accountId,omitempty"`
	Key       string   `json:"key,omitempty"`
	Points    int      `json:"points"`
	Villages  int      `json:"villages"`
	Conquered int      `json:"conquered"`
	Killed    int64    `json:"killed"`
}

func init() { endHook = func(w World) { w.State().Archive = BuildArchive(w) } }

// BuildArchive sums up the world: everyone who settled, ranked by points (ties by id); "killed" is the population
// of defeated units, as in the rankings.
func BuildArchive(w World) *Archive {
	s := w.State()
	a := &Archive{Summary: ArchiveSummary{EndReason: s.EndReason, WinnerID: s.WinnerID, AttacksSent: s.Totals.AttacksSent,
		LargestBattleUnits: s.Totals.LargestBattleUnits}}
	conquered := map[PlayerID]int{}
	for _, c := range w.ConquestsSince(0) {
		conquered[c.ToID]++
		if IsHumanID(c.ToID) {
			a.Summary.ConquestsHumans++
		} else {
			a.Summary.ConquestsBots++
		}
	}
	points := w.PointsByOwner()
	for _, p := range w.Players() {
		if p.Status == StatusPending {
			continue
		}
		row := LeaderRow{PlayerID: p.ID, Name: p.Name, IsBot: !p.IsHuman, Points: points[p.ID], Villages: len(p.VillageIDs),
			Conquered: conquered[p.ID], Killed: int64(p.OD.Att + p.OD.Def)}
		if p.IsHuman {
			a.Summary.Humans++
			if p.IsGuest {
				a.Summary.Guests++
			}
			row.AccountID, row.Key = p.AccountID, p.Key
			if h := w.Human(p.ID); h != nil {
				a.Summary.HumanUnitsKilled += int64(h.Stats.UnitsKilled)
				a.Summary.HumanUnitsLost += int64(h.Stats.UnitsLost)
			}
		} else {
			a.Summary.Bots++
		}
		a.Rows = append(a.Rows, row)
	}
	sort.SliceStable(a.Rows, func(i, j int) bool {
		if a.Rows[i].Points != a.Rows[j].Points {
			return a.Rows[i].Points > a.Rows[j].Points
		}
		return a.Rows[i].PlayerID < a.Rows[j].PlayerID
	})
	for i := range a.Rows {
		a.Rows[i].Rank = i + 1
	}
	return a
}
