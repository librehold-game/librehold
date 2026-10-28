package game

import "strconv"

// Bot settings and tables: difficulty, sessions, styles and names.

// DifficultyFloor is the minimum share of strong bots (skill ≥ BotConfig.StrongSkill) per difficulty.
var DifficultyFloor = map[Difficulty]float64{Easy: 0.05, Normal: 0.2, Hard: 0.5, Brutal: 0.85}

type StrongWeak struct {
	Strong float64 `json:"strong"`
	Weak   float64 `json:"weak"`
}

// BotConfig holds the bot tuning: skill, temperament, styles and pacing.
var BotConfig = struct {
	StrongSkill     float64 `json:"strongSkill"`
	Temperature     Range   `json:"temperature"`
	WeakSkipCheckIn float64 `json:"weakSkipCheckIn"`
	StartRush       struct {
		Days     float64    `json:"days"`
		GapMin   [2]float64 `json:"gapMin"`
		LenMin   [2]float64 `json:"lenMin"`
		WeakSkip float64    `json:"weakSkip"`
	} `json:"startRush"`
	ThinkEveryMin         float64              `json:"thinkEveryMin"`
	FreePop               int                  `json:"freePop"`
	BottleneckShare       float64              `json:"bottleneckShare"`
	MaxedFarmReserve      float64              `json:"maxedFarmReserve"`
	KnowRadius            float64              `json:"knowRadius"`
	FarmRadius            StrongWeak           `json:"farmRadius"`
	FarmMinIntervalMin    float64              `json:"farmMinIntervalMin"`
	DeadHaulRatio         float64              `json:"deadHaulRatio"`
	DeadFarmDays          float64              `json:"deadFarmDays"`
	RediscoverHours       float64              `json:"rediscoverHours"`
	ForgetDays            float64              `json:"forgetDays"`
	FarmLoot              float64              `json:"farmLoot"`
	FarmRefillHours       float64              `json:"farmRefillHours"`
	MaxRaidsPerThink      int                  `json:"maxRaidsPerThink"`
	MaxNightRuns          int                  `json:"maxNightRuns"`
	QuitScale             float64              `json:"quitScale"`
	AbandonAfterDays      float64              `json:"abandonAfterDays"`
	PeerJoinDays          float64              `json:"peerJoinDays"`
	GrudgeFadeDays        float64              `json:"grudgeFadeDays"`
	NukeAttack            float64              `json:"nukeAttack"`
	WeakExpandDailyChance float64              `json:"weakExpandDailyChance"`
	OffShare              map[BotStyle]float64 `json:"offShare"`
	StyleWeights          map[BotStyle]float64 `json:"styleWeights"`
}{
	StrongSkill:     0.7,
	Temperature:     Range{0.1, 1},
	WeakSkipCheckIn: 0.3,
	StartRush: struct {
		Days     float64    `json:"days"`
		GapMin   [2]float64 `json:"gapMin"`
		LenMin   [2]float64 `json:"lenMin"`
		WeakSkip float64    `json:"weakSkip"`
	}{1, [2]float64{60, 120}, [2]float64{5, 12}, 0.9},
	ThinkEveryMin:         12,
	FreePop:               250,
	BottleneckShare:       0.2,
	MaxedFarmReserve:      0.08,
	KnowRadius:            20,
	FarmRadius:            StrongWeak{15, 20},
	FarmMinIntervalMin:    30,
	DeadHaulRatio:         0.35,
	DeadFarmDays:          3,
	RediscoverHours:       24,
	ForgetDays:            3,
	FarmLoot:              1500,
	FarmRefillHours:       6,
	MaxRaidsPerThink:      25,
	MaxNightRuns:          15,
	QuitScale:             3,
	AbandonAfterDays:      21,
	PeerJoinDays:          7,
	GrudgeFadeDays:        5,
	NukeAttack:            150_000,
	WeakExpandDailyChance: 0.3,
	OffShare: map[BotStyle]float64{StyleAggressor: 0.75, StyleFarmer: 0.6, StyleOpportunist: 0.6,
		StylePointBuilder: 0.5, StyleTurtle: 0.3},
	StyleWeights: map[BotStyle]float64{StyleFarmer: 1, StyleTurtle: 1, StyleAggressor: 1, StylePointBuilder: 1,
		StyleOpportunist: 1},
}

// Bot names are two syllables that read the same in every language ("Torvik", "Velmund"), sometimes with digits.
var botPrefixes = [...]string{
	"Tor", "Vel", "Mar", "Kal", "Bren", "Dra", "Sil", "Ror", "Gal", "Ost", "Wen", "Har", "Lud", "Mir", "Rad",
	"Sven", "Bor", "Dob", "Jar", "Kas", "Ler", "Nor", "Ul", "Var", "Zed", "Ark", "Bel", "Cor", "Fen", "Grim",
}

var botSuffixes = [...]string{
	"vik", "dor", "wen", "ric", "mund", "las", "gar", "tan", "mir", "bert", "win", "slav", "helm", "ard", "ko",
	"rik", "mar", "ton", "ger", "nik", "ius", "an", "dan", "gon", "el", "ias", "ok", "us", "wyn", "ald",
}

const botNameStep = 337

// BotName is deterministic for (seed, index) and unique in a world for index < 900.
func BotName(seed uint32, index int) string {
	combos := uint64(len(botPrefixes) * len(botSuffixes))
	k := (uint64(HashSeed(seed, "names")) + uint64(index)*botNameStep) % combos
	name := botPrefixes[k/uint64(len(botSuffixes))] + botSuffixes[k%uint64(len(botSuffixes))]
	h := HashSeed(seed, "nameDigits", index)
	if h%5 < 2 {
		// `h >> 3` works on the signed 32-bit value, and `%` keeps the sign.
		return name + strconv.Itoa(int(10+(int32(h)>>3)%90))
	}
	return name
}

var PlayerColors = []string{
	"#d23b2f", "#2f6fd2", "#8e3fbf", "#e07b1a", "#e8c51e", "#7a4a1e", "#e04f9a", "#1fb4c4",
	"#f2f2f2", "#222222", "#b5172f", "#5a3fd2", "#ff9d6e", "#9fb7ff", "#c79a3d", "#6b1f5f",
	"#004d73", "#ff5252", "#3949ab", "#ffb300", "#8d6e63", "#ec407a", "#26c6da", "#9e9e9e",
}
