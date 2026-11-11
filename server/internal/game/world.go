package game

// World is everything the rules may read or change. Implementations load lazily and track changes;
// an infrastructure failure inside a method panics with *StoreError and is recovered by the caller of the
// unit of work (internal/app), never inside internal/game.
type World interface {
	Now() int64
	State() *WorldState // the ended flag and totals of an instant world
	Seed() uint32
	Secret() []byte // the key of HiddenSeed; empty in test worlds
	Config() *WorldConfig
	ClockOrigin() int64         // local wall ms (zone offset applied) at world time 0
	NewID(prefix string) string // `${prefix}_${base36(counter)}`, counter per world

	Village(id string) *Village                 // stored villages only; nil if none
	PutVillage(v *Village)                      // insert a new village
	VillagesIn(x0, y0, x1, y1 int) []VillageRef // stored villages in the rectangle, inclusive, sorted by ID
	SupportHosts(homeID string) []*Village      // villages whose SupportFrom has homeID, sorted by ID
	PointsByOwner() map[PlayerID]int

	Player(id PlayerID) *Player
	Players() []*Player // sorted by ID
	PutPlayer(p *Player)
	Brain(id PlayerID) *BotBrain   // nil for humans
	Human(id PlayerID) *HumanState // nil for bots
	PutBrain(id PlayerID, b *BotBrain)
	PutHuman(id PlayerID, h *HumanState)

	Command(id string) *Command
	CommandsHome(villageID string) []*Command // CommandHome(c) == villageID (or a player id: their armies cut loose), sorted by (ArrivesAt, ID)
	CommandsTo(villageID string) []*Command   // c.TargetID == villageID, sorted by (ArrivesAt, ID)
	PutCommand(c *Command)
	DeleteCommand(id string)
	ArmiesMarching(now int64, near Box) []*Command // attack, support and return commands not yet arrived whose LegBox overlaps near, by ID (bystander sight)

	Schedule(key string, at int64) // at < Now() is clamped to Now(); same key+time is a no-op
	Unschedule(key string)
	IsScheduled(key string) bool
	ScheduledAt(key string) (int64, bool)

	AddReport(recipient PlayerID, r *Report)                       // assigns r.ID = NewID("rep"); keeps the newest CombatBalance.MaxReports per recipient
	Reports(recipient PlayerID, before int64, limit int) []*Report // Time < before, newest first; limit ≤ 0 = all
	SetReportRead(recipient PlayerID, ids []string, read bool)
	DeleteReports(recipient PlayerID, ids []string)
	Intel(player PlayerID, villageID string) *Intel
	IntelFor(player PlayerID, villageIDs []string) map[string]*Intel // the player's intel on those villages, in one read
	PutIntel(player PlayerID, villageID string, in *Intel)           // nil deletes
	ScheduledSend(id string) *ScheduledSend
	ScheduledSendsOf(owner PlayerID) []*ScheduledSend // sorted by (SendAt, ID)
	PutScheduledSend(s *ScheduledSend)
	DeleteScheduledSend(id string)

	AddNews(n *NewsItem)        // assigns ID
	News(limit int) []*NewsItem // newest first
	AddConquest(c Conquest)
	ConquestsSince(t int64) []Conquest
	// AddPublicEvent records a conquest for the public feed; the store stamps At and WorldID.
	AddPublicEvent(e PublicEvent)
	// NoteBattle marks that a public battle happened now; the store keeps only the newest time, never who or where.
	NoteBattle()

	// Notify marks a player's state as changed when none of their own entities did (an army came into sight).
	Notify(p PlayerID)

	// AddTimeline keeps a step of a short world's history, in the same unit of work as the change it records.
	AddTimeline(e TimelineEntry)

	// LogAction appends to the action log at Now(), in the same unit of work as the change it records.
	LogAction(actor string, isBot bool, kind string, payload any)
}

// CommandHome is the village a command belongs to: the origin of an outgoing command, the target of a return
// (troops or merchants on their way back); an army cut loose from its village by a conquest belongs to its owner
// and is filed under the owner's id.
func CommandHome(c *Command) string {
	if c.OwnerID != "" {
		return string(c.OwnerID)
	}
	if c.Type == "return" || (c.Type == "trade" && c.Returning) {
		return c.TargetID
	}
	return c.OriginID
}

// LooseArmies are the owner's armies cut loose from their villages by a conquest, sorted by (ArrivesAt, ID).
func LooseArmies(w World, owner PlayerID) []*Command {
	return w.CommandsHome(owner)
}
