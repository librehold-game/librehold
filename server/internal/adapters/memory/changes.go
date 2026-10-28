package memory

import (
	"bytes"
	"encoding/json"

	"librehold/internal/app"
	"librehold/internal/game"
	"librehold/internal/game/gametest"
)

func jsonOf(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}

func villageMap(w *gametest.MemWorld) map[string]*game.Village {
	out := map[string]*game.Village{}
	for _, v := range w.Villages() {
		out[v.ID] = v
	}
	return out
}

func commandMap(w *gametest.MemWorld) map[string]*game.Command {
	out := map[string]*game.Command{}
	for _, c := range w.Commands() {
		out[c.ID] = c
	}
	return out
}

// changes compares the world before and after a kept unit of work, entity by entity in their stored JSON form,
// as the Postgres flush does.
func changes(before *gametest.MemWorld, t *tx) app.Changes {
	after := t.MemWorld
	var c app.Changes
	bv, av := villageMap(before), villageMap(after)
	for id, v := range av {
		old := bv[id]
		if old != nil && bytes.Equal(jsonOf(old), jsonOf(v)) {
			continue
		}
		ch := app.VillageChange{ID: id, NewOwner: v.OwnerID, OnMap: true}
		if old != nil {
			ch.OldOwner = old.OwnerID
			ch.OnMap = old.OwnerID != v.OwnerID || old.Name != v.Name
		}
		c.Villages = append(c.Villages, ch)
	}
	ownerOf := func(villageID string) game.PlayerRef {
		if v := av[villageID]; v != nil {
			return v.OwnerID
		}
		if v := bv[villageID]; v != nil {
			return v.OwnerID
		}
		return ""
	}
	bc, ac := commandMap(before), commandMap(after)
	touched := func(cmd *game.Command) {
		c.CommandOwners = append(c.CommandOwners, ownerOf(cmd.OriginID), ownerOf(cmd.TargetID))
	}
	for id, cmd := range ac {
		if old := bc[id]; old == nil || !bytes.Equal(jsonOf(old), jsonOf(cmd)) {
			touched(cmd)
			if old != nil {
				touched(old)
			}
		}
	}
	for id, old := range bc {
		if ac[id] == nil {
			touched(old)
		}
	}
	for _, p := range after.Players() {
		if !p.IsHuman {
			continue
		}
		if !bytes.Equal(jsonOf(before.Human(p.ID)), jsonOf(after.Human(p.ID))) {
			c.Humans = append(c.Humans, p.ID)
		}
		if !bytes.Equal(jsonOf(before.ReportsOf(p.ID)), jsonOf(after.ReportsOf(p.ID))) {
			c.ReportRecipients = append(c.ReportRecipients, p.ID)
		}
	}
	for _, p := range after.TakeNotified() {
		c.Viewers = append(c.Viewers, game.Ref(p))
	}
	c.News = !bytes.Equal(jsonOf(before.News(1)), jsonOf(after.News(1)))
	c.Ranking = t.statsRan
	return c
}
