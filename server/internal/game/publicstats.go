package game

import (
	"math"
	"sort"
)

// Public statistics for the rankings. The per-player parts live on the Player (OD, Daily); conquests in the World.

const dailyDays = 14

func popOf(u UnitCounts) float64 {
	n := 0.0
	for _, id := range UnitIDs {
		n += float64(UnitDefs[id].Pop * u[id])
	}
	return n
}

// todayCounts returns the player's counters for the local day, dropping days older than dailyDays.
func todayCounts(w World, p *Player) *DailyCounts {
	day := LocalDayIndex(w, w.Now())
	kept := p.Daily[:0]
	for _, d := range p.Daily {
		if d.Day > day-dailyDays {
			kept = append(kept, d)
		}
	}
	p.Daily = kept
	for i := range p.Daily {
		if p.Daily[i].Day == day {
			return &p.Daily[i]
		}
	}
	p.Daily = append(p.Daily, DailyCounts{Day: day})
	return &p.Daily[len(p.Daily)-1]
}

type Supporter struct {
	ID    PlayerID
	Share float64
}

type DefenderShare struct {
	OwnerShare float64
	Supporters []Supporter
}

// DefenderShares are the defenders' shares before the fight: the owner's garrison and the support (by population).
func DefenderShares(w World, v *Village) DefenderShare {
	own := popOf(UnitCounts(v.Units))
	homes := make([]string, 0, len(v.SupportFrom))
	for h := range v.SupportFrom {
		homes = append(homes, h)
	}
	sort.Strings(homes)
	type sup struct {
		id  PlayerID
		pop float64
	}
	var sups []sup
	total := own
	for _, h := range homes {
		if home := w.Village(h); home != nil && home.OwnerID != "" {
			p := popOf(v.SupportFrom[h])
			sups = append(sups, sup{PlayerID(home.OwnerID), p})
			total += p
		}
	}
	if total <= 0 {
		return DefenderShare{OwnerShare: 1}
	}
	out := DefenderShare{OwnerShare: own / total}
	for _, s := range sups {
		out.Supporters = append(out.Supporters, Supporter{s.id, s.pop / total})
	}
	return out
}

type BattleRecord struct {
	AttackerID   PlayerRef
	DefenderID   PlayerRef
	Shares       DefenderShare
	AttackerLost UnitCounts
	DefenderLost UnitCounts
	Loot         float64
}

// RecordBattle counts defeated opponents (by the population of the units killed): the attacker gets
// "att" for the defence's losses, defenders "def" for the attack's losses by their share, support "sup". Loot goes
// to the daily counter, and from barbarians also to "farmer of the day".
func RecordBattle(w World, b BattleRecord) {
	killedByAttacker := popOf(b.DefenderLost)
	killedByDefence := popOf(b.AttackerLost)
	if b.AttackerID != "" {
		if p := w.Player(PlayerID(b.AttackerID)); p != nil {
			p.OD.Att += killedByAttacker
			d := todayCounts(w, p)
			d.Att += killedByAttacker
			d.Loot += b.Loot
			if b.DefenderID == "" {
				d.Farm += b.Loot
			}
		}
	}
	if b.DefenderID != "" && killedByDefence > 0 {
		if p := w.Player(PlayerID(b.DefenderID)); p != nil {
			pts := math.Round(killedByDefence * b.Shares.OwnerShare)
			p.OD.Def += pts
			todayCounts(w, p).Def += pts
		}
	}
	for _, s := range b.Shares.Supporters {
		pts := math.Round(killedByDefence * s.Share)
		if pts <= 0 {
			continue
		}
		if p := w.Player(s.ID); p != nil {
			p.OD.Sup += pts
			todayCounts(w, p).Sup += pts
		}
	}
}

// RecordConquest puts a takeover on the public list.
func RecordConquest(w World, v *Village, from PlayerRef, to PlayerID) {
	w.AddConquest(Conquest{T: w.Now(), VillageID: v.ID, X: v.X, Y: v.Y, FromID: from, ToID: to})
}
