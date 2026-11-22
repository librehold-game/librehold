package game

import "strings"

// PlayerMark is another player the player wants to pick out on the map, in a colour of their choosing. It is a
// display choice and changes nothing in the world; it lives in the human's state so it follows the player to
// another device.
type PlayerMark struct {
	Name  string `json:"name"`
	Color string `json:"color"`
}

// MaxMarks caps the marked players one human keeps: far more than a row of chips can show, so a client cannot
// fill the player's record with them.
const MaxMarks = 40

// maxMarkName is the longest player name there can be (`usernamePattern` in internal/app), so a longer one could
// never match a player and is dropped.
const maxMarkName = 24

// SetMarks replaces the players this human has marked, in the order given. Names are compared the way a player
// types them — trimmed and without regard to case — so each name is kept once, in the place it first appeared; a
// later mark on the same name is dropped rather than recolouring it, since recolouring is what the colour on the
// chip is for. A mark whose colour is not `#rrggbb`, or whose name could not be a player's, is dropped too: the
// marks are drawn as they are, so nothing else may reach the map.
func SetMarks(w World, actor PlayerID, marks []PlayerMark) error {
	h := w.Human(actor)
	if h == nil {
		return NotOwner("set-marks")
	}
	seen := make(map[string]bool, len(marks))
	out := make([]PlayerMark, 0, len(marks))
	for _, m := range marks {
		name := strings.TrimSpace(m.Name)
		key := strings.ToLower(name)
		if name == "" || len(name) > maxMarkName || seen[key] || !isMarkColor(m.Color) {
			continue
		}
		seen[key] = true
		out = append(out, PlayerMark{Name: name, Color: m.Color})
	}
	if len(out) > MaxMarks {
		out = out[:MaxMarks]
	}
	h.Marks = out
	return nil
}

func isMarkColor(c string) bool {
	if len(c) != 7 || c[0] != '#' {
		return false
	}
	for i := 1; i < len(c); i++ {
		d := c[i]
		if !(d >= '0' && d <= '9' || d >= 'a' && d <= 'f' || d >= 'A' && d <= 'F') {
			return false
		}
	}
	return true
}
