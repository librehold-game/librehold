package game

// Conquest constants: loyalty, noble drops, coins.
var ConquestBalance = struct {
	MaxLoyalty           float64   `json:"maxLoyalty"`
	LoyaltyPerHour       float64   `json:"loyaltyPerHour"`
	NobleDropMin         int       `json:"nobleDropMin"`
	NobleDropMax         int       `json:"nobleDropMax"`
	NobleDropAvg         float64   `json:"nobleDropAvg"`
	LoyaltyAfterConquest float64   `json:"loyaltyAfterConquest"`
	PlunderLevelsMin     int       `json:"plunderLevelsMin"`
	PlunderLevelsMax     int       `json:"plunderLevelsMax"`
	CoinBaseCost         Resources `json:"coinBaseCost"`
	CoinsPerVillage      int       `json:"coinsPerVillage"`
	CoinsPerNoble        int       `json:"coinsPerNoble"`
	NobleMaxDistance     float64   `json:"nobleMaxDistance"`
}{100, 1, 20, 35, 27.5, 25, 1, 3, Resources{28000, 30000, 25000}, 1, 1, 70}
