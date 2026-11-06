package game

// TradeBalance is the world-invariant tuning of player to player trades: how long an offer waits, how many
// public posts a player may have out, and how long a post may take to deliver.

var TradeBalance = struct {
	// OfferExpiryMs is how long a trade offer stays open before it lapses (24 game hours).
	OfferExpiryMs int64 `json:"offerExpiryMs"`
	// PublicPerVillage is how many public posts one village may have out at once; a player's cap is this
	// times their village count.
	PublicPerVillage int `json:"publicPerVillage"`
	// MinDeliveryMs and MaxDeliveryMs bound how long a post may take to reach the taker, measured as merchant
	// travel time. 0 means unlimited.
	MinDeliveryMs int64 `json:"minDeliveryMs"`
	MaxDeliveryMs int64 `json:"maxDeliveryMs"`
	// MerchantCurveAt is the market level at which merchants stop being one per level and start growing
	// quadratically, the way Tribal Wars does: `(L - MerchantCurveAt + 1)^2 + MerchantCurveAt - 1`. Up to this level
	// a market gives one merchant per level; past it the top levels are worth their exponential build cost.
	MerchantCurveAt int `json:"merchantCurveAt"`
}{24 * MsPerHour, 3, MsPerHour, 24 * MsPerHour, 11}
