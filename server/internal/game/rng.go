package game

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"math"
	"strconv"
	"unicode/utf16"
)

// Mulberry32 is the game's only source of randomness: a 32-bit generator returning numbers in [0, 1). The
// browser's terrain code has the same generator (src/shared/terrain/rng.ts); uint32 arithmetic matches its
// Math.imul and >>> 0.
func Mulberry32(seed uint32) func() float64 {
	a := seed
	return func() float64 {
		a += 0x6d2b79f5
		t := a
		t = (t ^ (t >> 15)) * (t | 1)
		t ^= t + (t^(t>>7))*(t|61)
		return float64(t^(t>>14)) / 4294967296
	}
}

// HashSeed mixes parts into a 32-bit seed (FNV-1a over UTF-16 code units), as hashSeed in
// src/shared/terrain/rng.ts does: a number
// hashes as "n" + its decimal form, a string s as "s" + s.
func HashSeed(parts ...any) uint32 {
	h := uint32(0x811c9dc5)
	for _, part := range parts {
		for _, c := range utf16.Encode([]rune(seedPart(part))) {
			h ^= uint32(c)
			h *= 0x01000193
		}
		h ^= 0xff
		h *= 0x01000193
	}
	return h
}

func seedPart(part any) string {
	switch p := part.(type) {
	case string:
		return "s" + p
	case int:
		return "n" + strconv.Itoa(p)
	case int64:
		return "n" + strconv.FormatInt(p, 10)
	case uint32:
		return "n" + strconv.FormatUint(uint64(p), 10)
	case float64:
		return "n" + strconv.FormatFloat(p, 'f', -1, 64)
	}
	panic(fmt.Sprintf("HashSeed: unsupported part %T", part))
}

// HiddenSeed seeds what players must not foresee: battle luck, loyalty, traders and the bots' choices. The public
// seed draws the map, so anyone can work it out from the map; the world's secret never leaves the server. A world
// without a secret (the tests' and the fixtures') rolls from its public seed, as HashSeed(seed, parts...).
func HiddenSeed(w World, parts ...any) uint32 {
	secret := w.Secret()
	if len(secret) == 0 {
		return HashSeed(append([]any{w.Seed()}, parts...)...)
	}
	mac := hmac.New(sha256.New, secret)
	for _, part := range parts {
		s := seedPart(part)
		mac.Write(binary.AppendUvarint(nil, uint64(len(s))))
		mac.Write([]byte(s))
	}
	return binary.LittleEndian.Uint32(mac.Sum(nil))
}

// RandInt returns an integer in [min, max] inclusive.
func RandInt(r func() float64, min, max int) int {
	return min + int(math.Floor(r()*float64(max-min+1)))
}

// RandRange returns a float in [min, max).
func RandRange(r func() float64, min, max float64) float64 {
	return min + r()*(max-min)
}
