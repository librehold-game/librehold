package game

// AddNewsItem appends a rumour at the world's now: a catalogue key and the values its text needs. A non-empty
// `For` makes it one player's private news.
func AddNewsItem(w World, key string, params map[string]any, villageID string, alert bool, forPlayer PlayerID) {
	w.AddNews(&NewsItem{Time: w.Now(), Kind: "world", Key: key, Params: params, VillageID: villageID, Alert: alert,
		For: forPlayer})
}
