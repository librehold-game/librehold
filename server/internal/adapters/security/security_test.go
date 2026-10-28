package security_test

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"librehold/internal/adapters/security"
)

func TestArgonRoundTrip(t *testing.T) {
	ctx := context.Background()
	h := security.NewArgon2()
	verify := func(hash, pw string) bool {
		ok, err := h.Verify(ctx, hash, pw)
		if err != nil {
			t.Fatal(err)
		}
		return ok
	}
	hash, err := h.Hash(ctx, "tajne hasło")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(hash, "$argon2id$v=19$m=65536,t=1,p=4$") {
		t.Fatalf("not a PHC argon2id string: %s", hash)
	}
	if !verify(hash, "tajne hasło") {
		t.Fatal("the right password is refused")
	}
	if verify(hash, "tajne haslo") || verify("garbage", "tajne hasło") {
		t.Fatal("a wrong password or a broken hash is accepted")
	}
	again, _ := h.Hash(ctx, "tajne hasło")
	if again == hash {
		t.Fatal("two hashes share a salt")
	}
}

func TestTokens(t *testing.T) {
	ts := security.NewTokens()
	plain, hash := ts.New()
	if len(plain) != 43 || strings.ContainsAny(plain, "+/=") {
		t.Fatalf("not 32 bytes of base64url: %q", plain)
	}
	if len(hash) != 32 || !bytes.Equal(hash, ts.Hash(plain)) {
		t.Fatal("hash is not the SHA-256 of the token")
	}
	other, _ := ts.New()
	if other == plain {
		t.Fatal("tokens repeat")
	}
}
