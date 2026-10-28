// Package security holds the password hasher and the session token source.
package security

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

const (
	argonMemory  = 64 * 1024 // KiB
	argonTime    = 1
	argonThreads = 4
	argonSalt    = 16
	argonKey     = 32
	// argonSlots is how many hashes run at once; with argonMemory each, a burst of logins waits its turn instead of
	// taking all of the server's memory.
	argonSlots = 4
)

type keyFunc func(pw, salt []byte, time, memory uint32, threads uint8, keyLen uint32) []byte

// Argon2 hashes passwords with argon2id into PHC strings.
type Argon2 struct {
	slots chan struct{}
	key   keyFunc
}

func NewArgon2() *Argon2 { return newArgon2(argonSlots, argon2.IDKey) }

func newArgon2(slots int, key keyFunc) *Argon2 {
	return &Argon2{slots: make(chan struct{}, slots), key: key}
}

// derive waits for a slot as long as ctx lasts, so a request whose client went away leaves the queue.
func (a *Argon2) derive(ctx context.Context, pw, salt []byte, time, memory uint32, threads uint8, keyLen uint32) ([]byte, error) {
	select {
	case a.slots <- struct{}{}:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	defer func() { <-a.slots }()
	return a.key(pw, salt, time, memory, threads, keyLen), nil
}

var b64 = base64.RawStdEncoding

func (a *Argon2) Hash(ctx context.Context, pw string) (string, error) {
	salt := make([]byte, argonSalt)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key, err := a.derive(ctx, []byte(pw), salt, argonTime, argonMemory, argonThreads, argonKey)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s", argon2.Version, argonMemory, argonTime, argonThreads,
		b64.EncodeToString(salt), b64.EncodeToString(key)), nil
}

// Verify checks pw against a PHC string, with the parameters stored in it; a broken string matches nothing.
func (a *Argon2) Verify(ctx context.Context, hash, pw string) (bool, error) {
	parts := strings.Split(hash, "$")
	if len(parts) != 6 || parts[1] != "argon2id" || parts[2] != fmt.Sprintf("v=%d", argon2.Version) {
		return false, nil
	}
	var m uint32
	var t uint32
	var p uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &m, &t, &p); err != nil || m == 0 || t == 0 || p == 0 {
		return false, nil
	}
	salt, err := b64.DecodeString(parts[4])
	if err != nil {
		return false, nil
	}
	want, err := b64.DecodeString(parts[5])
	if err != nil || len(want) == 0 {
		return false, nil
	}
	got, err := a.derive(ctx, []byte(pw), salt, t, m, p, uint32(len(want)))
	if err != nil {
		return false, err
	}
	return subtle.ConstantTimeCompare(got, want) == 1, nil
}
