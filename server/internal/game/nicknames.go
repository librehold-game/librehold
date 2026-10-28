package game

import "strconv"

// Nickname is a guest's name: two syllables from the bot name lists and two or three digits, deterministic for
// (seed, n). The "guest" salt keeps it apart from bot names.
func Nickname(seed uint32, n int) string {
	h := HashSeed(seed, "guest", n)
	name := botPrefixes[h%uint32(len(botPrefixes))] + botSuffixes[(h/uint32(len(botPrefixes)))%uint32(len(botSuffixes))]
	digits := HashSeed(seed, "guestDigits", n)
	if digits%2 == 0 {
		return name + strconv.Itoa(int(10+digits%90))
	}
	return name + strconv.Itoa(int(100+digits%900))
}
