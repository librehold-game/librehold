package game

// TimelineEntry is one step of a short world's history, enough to replay its map: a village appearing, a village
// changing hands, a battle landing. Names are left out; owners are player ids.
type TimelineEntry struct {
	At       int64     `json:"t"`
	Kind     string    `json:"k"` // "village" | "owner" | "battle"
	X        float64   `json:"x"` // the village; for a battle the target village or the meeting point
	Y        float64   `json:"y"`
	FromX    int       `json:"fx,omitempty"` // battle: the attacker's origin village
	FromY    int       `json:"fy,omitempty"`
	Owner    PlayerRef `json:"o,omitempty"` // village, owner: the (new) owner, "" = barbarian
	Attacker PlayerRef `json:"a,omitempty"`
	Defender PlayerRef `json:"d,omitempty"`
	Troops   int       `json:"n,omitempty"`
}

// RecordsTimeline says whether a world keeps a timeline: short worlds do, for their replay.
func RecordsTimeline(cfg *WorldConfig) bool { return IsShort(cfg) }

func recordTimeline(w World, e TimelineEntry) {
	if !RecordsTimeline(w.Config()) {
		return
	}
	e.At = w.Now()
	w.AddTimeline(e)
}

// BattleSizeClass sorts a battle by the troops on both sides: 1 below 100, 2 below 1,000, 3 below 5,000, else 4.
func BattleSizeClass(troops int) int {
	switch {
	case troops < 100:
		return 1
	case troops < 1000:
		return 2
	case troops < 5000:
		return 3
	default:
		return 4
	}
}

// TimelineOwner is a player as the replay colours them; names come from the game's public results.
type TimelineOwner struct {
	PlayerID PlayerID `json:"playerId"`
	Color    string   `json:"color"`
	Bot      bool     `json:"bot,omitempty"`
}

// TimelineFile is a finished game's timeline as the browser replays it: no seed, no names.
type TimelineFile struct {
	Version    int             `json:"v"` // 1
	MapSize    int             `json:"mapSize"`
	EndAt      int64           `json:"endAt"`      // world ms of the end
	Owners     []TimelineOwner `json:"owners"`     // owner n in events is Owners[n-1]; 0 = barbarian
	Barbarians [][2]int        `json:"barbarians"` // generated villages at world time 0
	// [t,0,x,y,owner] village · [t,1,x,y,owner] owner change · [t,2,fx,fy,x,y,attacker,defender,size]
	Events [][]float64 `json:"events"`
}

// PackTimeline turns a world's timeline into the file the replay reads: owners numbered by first appearance,
// the start barbarians derived from the seed, events in the order they happened.
func PackTimeline(seed uint32, cfg *WorldConfig, endAt int64, players []*Player, entries []TimelineEntry) TimelineFile {
	f := TimelineFile{Version: 1, MapSize: cfg.MapSize, EndAt: endAt, Owners: []TimelineOwner{}, Barbarians: [][2]int{},
		Events: make([][]float64, 0, len(entries))}
	byID := make(map[PlayerID]*Player, len(players))
	for _, p := range players {
		byID[p.ID] = p
	}
	numbers := map[PlayerRef]int{}
	owner := func(r PlayerRef) float64 {
		if r == "" {
			return 0
		}
		n, ok := numbers[r]
		if !ok {
			o := TimelineOwner{PlayerID: PlayerID(r)}
			if p := byID[PlayerID(r)]; p != nil {
				o.Color, o.Bot = p.Color, !p.IsHuman
			}
			f.Owners = append(f.Owners, o)
			n = len(f.Owners)
			numbers[r] = n
		}
		return float64(n)
	}
	for x := range cfg.MapSize {
		for y := range cfg.MapSize {
			if TileHasVillage(seed, cfg, x, y) {
				f.Barbarians = append(f.Barbarians, [2]int{x, y})
			}
		}
	}
	for _, e := range entries {
		t := float64(e.At)
		switch e.Kind {
		case "village":
			f.Events = append(f.Events, []float64{t, 0, e.X, e.Y, owner(e.Owner)})
		case "owner":
			f.Events = append(f.Events, []float64{t, 1, e.X, e.Y, owner(e.Owner)})
		case "battle":
			f.Events = append(f.Events, []float64{t, 2, float64(e.FromX), float64(e.FromY), e.X, e.Y, owner(e.Attacker),
				owner(e.Defender), float64(BattleSizeClass(e.Troops))})
		}
	}
	return f
}
