package gametest

import (
	"encoding/json"
	"maps"
	"slices"

	"librehold/internal/game"
)

func cloneJSON[T any](v *T) *T {
	if v == nil {
		return nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	out := new(T)
	if err := json.Unmarshal(b, out); err != nil {
		panic(err)
	}
	return out
}

func cloneMap[K comparable, T any](m map[K]*T) map[K]*T {
	out := make(map[K]*T, len(m))
	for k, v := range m {
		out[k] = cloneJSON(v)
	}
	return out
}

// CloneVillage is a deep copy of v, for tests that compute expectations without touching the world.
func CloneVillage(v *game.Village) *game.Village { return cloneJSON(v) }

// Clone is a deep copy that shares nothing with w, through the same JSON form the database stores; the
// memory adapter runs each unit of work on one and keeps it only on success.
func (w *MemWorld) Clone() *MemWorld {
	c := *w
	c.state = *cloneJSON(&w.state)
	c.villages = cloneMap(w.villages)
	c.players = cloneMap(w.players)
	for id, p := range w.players {
		c.players[id].AccountID = p.AccountID
	}
	c.brains = cloneMap(w.brains)
	c.humans = cloneMap(w.humans)
	c.commands = cloneMap(w.commands)
	c.reports = make(map[game.PlayerID][]*game.Report, len(w.reports))
	for id, list := range w.reports {
		cl := make([]*game.Report, len(list))
		for i, r := range list {
			cl[i] = cloneJSON(r)
		}
		c.reports[id] = cl
	}
	c.intel = cloneMap(w.intel)
	c.scheduled = cloneMap(w.scheduled)
	c.news = make([]*game.NewsItem, len(w.news))
	for i, n := range w.news {
		c.news[i] = cloneJSON(n)
	}
	c.conquests = make([]game.Conquest, len(w.conquests))
	for i := range w.conquests {
		c.conquests[i] = *cloneJSON(&w.conquests[i])
	}
	c.public = slices.Clone(w.public)
	c.events = slices.Clone(w.events)
	c.eventIndex = maps.Clone(w.eventIndex)
	c.touched = maps.Clone(w.touched)
	c.actions = slices.Clone(w.actions)
	c.timeline = slices.Clone(w.timeline)
	return &c
}
