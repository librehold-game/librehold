// Package gametest holds an in-memory game.World for rule tests and for the memory adapter.
package gametest

import (
	"container/heap"
	"slices"
	"sort"
	"strconv"

	"librehold/internal/game"
)

type queuedEvent struct {
	at  int64
	seq int64
	key string
}

type eventHeap []queuedEvent

func (h eventHeap) Len() int { return len(h) }
func (h eventHeap) Less(i, j int) bool {
	return h[i].at < h[j].at || (h[i].at == h[j].at && h[i].seq < h[j].seq)
}
func (h eventHeap) Swap(i, j int) { h[i], h[j] = h[j], h[i] }
func (h *eventHeap) Push(x any)   { *h = append(*h, x.(queuedEvent)) }
func (h *eventHeap) Pop() any {
	old := *h
	e := old[len(old)-1]
	*h = old[:len(old)-1]
	return e
}

type intelKey struct {
	player  game.PlayerID
	village string
}

// MemWorld implements game.World over plain maps. The event queue is a heap whose
// stale entries (a time other than the key's current one) are skipped when popped.
type MemWorld struct {
	state       game.WorldState
	now         int64
	seed        uint32
	secret      []byte
	cfg         game.WorldConfig
	clockOrigin int64
	nextID      int64

	villages  map[string]*game.Village
	players   map[game.PlayerID]*game.Player
	brains    map[game.PlayerID]*game.BotBrain
	humans    map[game.PlayerID]*game.HumanState
	commands  map[string]*game.Command
	reports   map[game.PlayerID][]*game.Report
	intel     map[intelKey]*game.Intel
	scheduled map[string]*game.ScheduledSend
	news      []*game.NewsItem
	conquests []game.Conquest
	public    []game.PublicEvent

	events     eventHeap
	eventIndex map[string]int64
	eventSeq   int64

	touched  map[game.PlayerID]bool
	notified map[game.PlayerID]bool
	actions  []game.LoggedAction  // logged since the last TakeActions
	timeline []game.TimelineEntry // kept since the last TakeTimeline
}

func NewMemWorld(seed uint32, cfg game.WorldConfig, clockOrigin int64) *MemWorld {
	return &MemWorld{
		seed:        seed,
		cfg:         cfg,
		clockOrigin: clockOrigin,
		nextID:      1,
		villages:    map[string]*game.Village{},
		players:     map[game.PlayerID]*game.Player{},
		brains:      map[game.PlayerID]*game.BotBrain{},
		humans:      map[game.PlayerID]*game.HumanState{},
		commands:    map[string]*game.Command{},
		reports:     map[game.PlayerID][]*game.Report{},
		intel:       map[intelKey]*game.Intel{},
		scheduled:   map[string]*game.ScheduledSend{},
		eventIndex:  map[string]int64{},
		touched:     map[game.PlayerID]bool{},
	}
}

func (w *MemWorld) Now() int64              { return w.now }
func (w *MemWorld) State() *game.WorldState { return &w.state }
func (w *MemWorld) SetNow(t int64)          { w.now = t }
func (w *MemWorld) Seed() uint32            { return w.seed }
func (w *MemWorld) Secret() []byte          { return w.secret }

// SetSecret gives the world a key for HiddenSeed; without one it rolls from its seed.
func (w *MemWorld) SetSecret(secret []byte)   { w.secret = secret }
func (w *MemWorld) Config() *game.WorldConfig { return &w.cfg }
func (w *MemWorld) ClockOrigin() int64        { return w.clockOrigin }

func (w *MemWorld) NewID(prefix string) string {
	id := prefix + "_" + strconv.FormatInt(w.nextID, 36)
	w.nextID++
	return id
}

// NextIDCounter and EventSeq expose the counters the Postgres adapter persists.
func (w *MemWorld) NextIDCounter() int64 { return w.nextID }
func (w *MemWorld) EventSeq() int64      { return w.eventSeq }

func (w *MemWorld) touchOwner(v *game.Village) {
	if v != nil && v.OwnerID != "" {
		if _, ok := w.humans[game.PlayerID(v.OwnerID)]; ok {
			w.touched[game.PlayerID(v.OwnerID)] = true
		}
	}
}

func (w *MemWorld) touchCommand(c *game.Command) {
	w.touchOwner(w.villages[c.OriginID])
	w.touchOwner(w.villages[c.TargetID])
}

// TouchedHumans returns, sorted, the humans whose villages, commands, human state or reports were accessed
// since the last call, and resets the set.
func (w *MemWorld) LogAction(actor string, isBot bool, kind string, payload any) {
	w.actions = append(w.actions, game.NewLoggedAction(w.now, actor, isBot, kind, payload))
}

func (w *MemWorld) AddTimeline(e game.TimelineEntry) { w.timeline = append(w.timeline, e) }

// TakeTimeline returns the timeline entries kept since the last call and forgets them.
func (w *MemWorld) TakeTimeline() []game.TimelineEntry {
	out := w.timeline
	w.timeline = nil
	return out
}

// TakeActions returns the actions logged since the last call and forgets them.
func (w *MemWorld) TakeActions() []game.LoggedAction {
	out := w.actions
	w.actions = nil
	return out
}

func (w *MemWorld) TouchedHumans() []game.PlayerID {
	out := make([]game.PlayerID, 0, len(w.touched))
	for id := range w.touched {
		out = append(out, id)
	}
	sort.Strings(out)
	w.touched = map[game.PlayerID]bool{}
	return out
}

// ---- villages ----

func (w *MemWorld) Village(id string) *game.Village {
	v := w.villages[id]
	w.touchOwner(v)
	return v
}

func (w *MemWorld) PutVillage(v *game.Village) {
	w.villages[v.ID] = v
	w.touchOwner(v)
}

func (w *MemWorld) VillagesIn(x0, y0, x1, y1 int) []game.VillageRef {
	var out []game.VillageRef
	for _, v := range w.villages {
		if v.X >= x0 && v.X <= x1 && v.Y >= y0 && v.Y <= y1 {
			out = append(out, game.VillageRef{ID: v.ID, X: v.X, Y: v.Y, Name: v.Name, OwnerID: v.OwnerID, Points: v.Points})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func (w *MemWorld) SupportHosts(homeID string) []*game.Village {
	var out []*game.Village
	for _, v := range w.villages {
		if _, ok := v.SupportFrom[homeID]; ok {
			out = append(out, v)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	for _, v := range out {
		w.touchOwner(v)
	}
	return out
}

func (w *MemWorld) PointsByOwner() map[game.PlayerID]int {
	out := map[game.PlayerID]int{}
	for _, v := range w.villages {
		if v.OwnerID != "" {
			out[game.PlayerID(v.OwnerID)] += v.Points
		}
	}
	return out
}

// Villages returns every stored village, sorted by ID (tests).
func (w *MemWorld) Villages() []*game.Village {
	out := make([]*game.Village, 0, len(w.villages))
	for _, v := range w.villages {
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// ---- players ----

func (w *MemWorld) Player(id game.PlayerID) *game.Player { return w.players[id] }

func (w *MemWorld) Players() []*game.Player {
	out := make([]*game.Player, 0, len(w.players))
	for _, p := range w.players {
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func (w *MemWorld) PutPlayer(p *game.Player)                    { w.players[p.ID] = p }
func (w *MemWorld) Brain(id game.PlayerID) *game.BotBrain       { return w.brains[id] }
func (w *MemWorld) PutBrain(id game.PlayerID, b *game.BotBrain) { w.brains[id] = b }

func (w *MemWorld) Human(id game.PlayerID) *game.HumanState {
	h := w.humans[id]
	if h != nil {
		w.touched[id] = true
	}
	return h
}

func (w *MemWorld) PutHuman(id game.PlayerID, h *game.HumanState) {
	w.humans[id] = h
	w.touched[id] = true
}

// ---- commands ----

func (w *MemWorld) Command(id string) *game.Command {
	c := w.commands[id]
	if c != nil {
		w.touchCommand(c)
	}
	return c
}

func (w *MemWorld) commandsWhere(match func(c *game.Command) bool) []*game.Command {
	var out []*game.Command
	for _, c := range w.commands {
		if match(c) {
			out = append(out, c)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].ArrivesAt != out[j].ArrivesAt {
			return out[i].ArrivesAt < out[j].ArrivesAt
		}
		return out[i].ID < out[j].ID
	})
	for _, c := range out {
		w.touchCommand(c)
	}
	return out
}

func (w *MemWorld) CommandsHome(villageID string) []*game.Command {
	return w.commandsWhere(func(c *game.Command) bool { return game.CommandHome(c) == villageID })
}

func (w *MemWorld) CommandsTo(villageID string) []*game.Command {
	return w.commandsWhere(func(c *game.Command) bool { return c.TargetID == villageID })
}

// Commands returns every command, sorted by (ArrivesAt, ID) (tests).
func (w *MemWorld) Commands() []*game.Command {
	return w.commandsWhere(func(*game.Command) bool { return true })
}

func (w *MemWorld) ArmiesMarching(now int64) []*game.Command {
	var out []*game.Command
	for _, c := range w.commands {
		if game.IsArmy(c) && c.ArrivesAt > now {
			out = append(out, c)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func (w *MemWorld) MovingCommands(now int64) []*game.Command {
	var out []*game.Command
	for _, c := range w.commands {
		if (game.IsArmy(c) || c.Type == "intercept") && c.ArrivesAt > now {
			out = append(out, c)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func (w *MemWorld) Notify(p game.PlayerID) {
	if w.notified == nil {
		w.notified = map[game.PlayerID]bool{}
	}
	w.notified[p] = true
}

// TakeNotified returns, sorted, the players notified since the last call, and forgets them.
func (w *MemWorld) TakeNotified() []game.PlayerID {
	out := make([]game.PlayerID, 0, len(w.notified))
	for id := range w.notified {
		out = append(out, id)
	}
	sort.Strings(out)
	w.notified = nil
	return out
}

func (w *MemWorld) PutCommand(c *game.Command) {
	w.commands[c.ID] = c
	w.touchCommand(c)
}

func (w *MemWorld) DeleteCommand(id string) {
	if c := w.commands[id]; c != nil {
		w.touchCommand(c)
	}
	delete(w.commands, id)
}

// ---- events ----

func (w *MemWorld) Schedule(key string, at int64) {
	if at < w.now {
		at = w.now
	}
	if cur, ok := w.eventIndex[key]; ok && cur == at {
		return
	}
	w.eventIndex[key] = at
	heap.Push(&w.events, queuedEvent{at: at, seq: w.eventSeq, key: key})
	w.eventSeq++
}

func (w *MemWorld) Unschedule(key string) { delete(w.eventIndex, key) }

func (w *MemWorld) IsScheduled(key string) bool {
	_, ok := w.eventIndex[key]
	return ok
}

// ScheduledAt returns the current time of a scheduled key (tests).
func (w *MemWorld) ScheduledAt(key string) (int64, bool) {
	at, ok := w.eventIndex[key]
	return at, ok
}

// PopDue removes and returns the next valid event with time ≤ until, in (at, seq) order.
func (w *MemWorld) PopDue(until int64) (string, int64, bool) {
	for w.events.Len() > 0 {
		top := w.events[0]
		if cur, ok := w.eventIndex[top.key]; !ok || cur != top.at {
			heap.Pop(&w.events)
			continue
		}
		if top.at > until {
			return "", 0, false
		}
		heap.Pop(&w.events)
		delete(w.eventIndex, top.key)
		return top.key, top.at, true
	}
	return "", 0, false
}

// NextDue returns the time of the next valid event.
func (w *MemWorld) NextDue() (int64, bool) {
	for w.events.Len() > 0 {
		top := w.events[0]
		if cur, ok := w.eventIndex[top.key]; ok && cur == top.at {
			return top.at, true
		}
		heap.Pop(&w.events)
	}
	return 0, false
}

// ---- per-player records ----

func (w *MemWorld) AddReport(recipient game.PlayerID, r *game.Report) {
	r.ID = w.NewID("rep")
	list := append(w.reports[recipient], r)
	if over := len(list) - game.CombatBalance.MaxReports; over > 0 {
		list = append([]*game.Report(nil), list[over:]...)
	}
	w.reports[recipient] = list
	w.touched[recipient] = true
}

func (w *MemWorld) Reports(recipient game.PlayerID, before int64, limit int) []*game.Report {
	var out []*game.Report
	list := w.reports[recipient]
	for i := len(list) - 1; i >= 0 && (limit <= 0 || len(out) < limit); i-- {
		if list[i].Time < before {
			out = append(out, list[i])
		}
	}
	w.touched[recipient] = true
	return out
}

func (w *MemWorld) SetReportRead(recipient game.PlayerID, ids []string, read bool) {
	for _, r := range w.reports[recipient] {
		for _, id := range ids {
			if r.ID == id {
				r.Read = read
			}
		}
	}
	w.touched[recipient] = true
}

func (w *MemWorld) DeleteReports(recipient game.PlayerID, ids []string) {
	kept := w.reports[recipient][:0:0]
	for _, r := range w.reports[recipient] {
		drop := false
		for _, id := range ids {
			drop = drop || r.ID == id
		}
		if !drop {
			kept = append(kept, r)
		}
	}
	w.reports[recipient] = kept
	w.touched[recipient] = true
}

// ReportsOf returns a recipient's reports, oldest first (tests).
func (w *MemWorld) ReportsOf(recipient game.PlayerID) []*game.Report { return w.reports[recipient] }

func (w *MemWorld) Intel(player game.PlayerID, villageID string) *game.Intel {
	return w.intel[intelKey{player, villageID}]
}

func (w *MemWorld) PutIntel(player game.PlayerID, villageID string, in *game.Intel) {
	if in == nil {
		delete(w.intel, intelKey{player, villageID})
		return
	}
	w.intel[intelKey{player, villageID}] = in
}

// IntelOf returns every intel record of a player, keyed by village (views).
func (w *MemWorld) IntelOf(player game.PlayerID) map[string]*game.Intel {
	out := map[string]*game.Intel{}
	for k, in := range w.intel {
		if k.player == player {
			out[k.village] = in
		}
	}
	return out
}

func (w *MemWorld) ScheduledSend(id string) *game.ScheduledSend { return w.scheduled[id] }

func (w *MemWorld) ScheduledSendsOf(owner game.PlayerID) []*game.ScheduledSend {
	var out []*game.ScheduledSend
	for _, s := range w.scheduled {
		if s.OwnerID == owner {
			out = append(out, s)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].SendAt != out[j].SendAt {
			return out[i].SendAt < out[j].SendAt
		}
		return out[i].ID < out[j].ID
	})
	return out
}

func (w *MemWorld) PutScheduledSend(s *game.ScheduledSend) { w.scheduled[s.ID] = s }
func (w *MemWorld) DeleteScheduledSend(id string)          { delete(w.scheduled, id) }

func (w *MemWorld) AddNews(n *game.NewsItem) {
	n.ID = w.NewID("news")
	w.news = append(w.news, n)
	if over := len(w.news) - game.NewsConfig.Max; over > 0 {
		w.news = append([]*game.NewsItem(nil), w.news[over:]...)
	}
}

func (w *MemWorld) News(limit int) []*game.NewsItem {
	var out []*game.NewsItem
	for i := len(w.news) - 1; i >= 0 && (limit <= 0 || len(out) < limit); i-- {
		out = append(out, w.news[i])
	}
	return out
}

func (w *MemWorld) AddConquest(c game.Conquest) { w.conquests = append(w.conquests, c) }

func (w *MemWorld) ConquestsSince(t int64) []game.Conquest {
	var out []game.Conquest
	for _, c := range w.conquests {
		if c.T >= t {
			out = append(out, c)
		}
	}
	return out
}

func (w *MemWorld) AddPublicEvent(e game.PublicEvent) { w.public = append(w.public, e) }

// PublicEvents returns the public events in the order they were added.
func (w *MemWorld) PublicEvents() []game.PublicEvent { return slices.Clone(w.public) }

var _ game.World = (*MemWorld)(nil)
