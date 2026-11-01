package game

// Bots' auto-farm: once unlocked as for humans (AutoFarmUnlockLoot looted, or FarmBalance.AutoFarmUnlockVillages
// villages), a bot whose play has AutoFarm raids from every village on the auto-farm tick, online or not — at
// barbarian and abandoned villages only.

// AutoFarmUnlockLoot is the loot that unlocks the auto-farm (the loot5000 quest for humans).
const AutoFarmUnlockLoot = 5000

func init() { registerHandler("botfarm", onBotFarmEvent) }

func botAutoFarmUnlocked(w World, botID PlayerID) bool {
	p, b := w.Player(botID), w.Brain(botID)
	return p != nil && b != nil && p.Status == StatusActive && economyPlay(w, botID).AutoFarm &&
		(SumResources(b.Looted) >= AutoFarmUnlockLoot || len(p.VillageIDs) >= FarmBalance.AutoFarmUnlockVillages)
}

// BotAutoFarmTick raids from the village up to FarmBalance.AutoFarmMaxPerTick times and returns how many went.
func BotAutoFarmTick(w World, botID PlayerID, villageID string) int {
	v := w.Village(villageID)
	if v == nil || v.OwnerID != Ref(botID) || !botAutoFarmUnlocked(w, botID) {
		return 0
	}
	Touch(w, v)
	return farmFrom(w, botID, v, FarmBalance.AutoFarmMaxPerTick, false)
}

// ScheduleBotAutoFarm gives each village of an unlocked bot its tick on the auto-farm grid.
func ScheduleBotAutoFarm(w World, botID PlayerID) {
	if !botAutoFarmUnlocked(w, botID) {
		return
	}
	for _, id := range w.Player(botID).VillageIDs {
		if _, ok := w.ScheduledAt("botfarm:" + id); !ok {
			scheduleAutoFarmTick(w, "botfarm:", id)
		}
	}
}

func onBotFarmEvent(w World, villageID string) error {
	v := w.Village(villageID)
	if v == nil || v.OwnerID == "" {
		return nil
	}
	botID := PlayerID(v.OwnerID)
	if !botAutoFarmUnlocked(w, botID) {
		return nil
	}
	BotAutoFarmTick(w, botID, villageID)
	scheduleAutoFarmTick(w, "botfarm:", villageID)
	return nil
}

// LootWindow is a bot's recent loot: hauls summed over windows of lootWindowMs, the last closed one as a rate.
type LootWindow struct {
	Since   int64   `json:"since"`
	Loot    float64 `json:"loot"`
	PerHour float64 `json:"perHour"`
}

const lootWindowMs = 12 * MsPerHour

// Add counts a haul brought home; a window of lootWindowMs or more closes into PerHour and a new one starts.
func (l *LootWindow) Add(now int64, amount float64) {
	if l.Loot == 0 && l.PerHour == 0 {
		l.Since = now // the first window starts with the first haul
	}
	if now-l.Since >= lootWindowMs {
		l.PerHour = l.Loot / (float64(now-l.Since) / float64(MsPerHour))
		l.Since, l.Loot = now, 0
	}
	l.Loot += amount
}
