package config_test

import (
	"net/netip"
	"slices"
	"testing"
	"time"

	"librehold/internal/config"
)

func TestConfigRequiresDatabaseURL(t *testing.T) {
	t.Setenv("LH_DATABASE_URL", "")
	if _, err := config.Load(); err == nil {
		t.Fatal("no error without LH_DATABASE_URL")
	}
}

func TestConfigRefusesPlaceholderPasswords(t *testing.T) {
	t.Setenv("LH_VALKEY_URL", "")
	t.Setenv("LH_DATABASE_URL", "postgres://librehold:change-me@db:5432/librehold")
	if _, err := config.Load(); err == nil {
		t.Fatal("no error with the example database password")
	}
	t.Setenv("LH_DATABASE_URL", "postgres://x")
	t.Setenv("LH_VALKEY_URL", "redis://:change-me-too@valkey:6379/0")
	if _, err := config.Load(); err == nil {
		t.Fatal("no error with the example Valkey password")
	}
}

func TestConfigDefaultsAndOverrides(t *testing.T) {
	t.Setenv("LH_DATABASE_URL", "postgres://x")
	for _, k := range []string{"LH_HTTP_ADDR", "LH_WEB_DIR", "LH_COOKIE_SECURE", "LH_TRUSTED_PROXIES", "LH_VALKEY_URL", "LH_ADMIN_USERNAME", "LH_ADMIN_PASSWORD", "LH_RECORD_DECISIONS"} {
		t.Setenv(k, "")
	}
	c, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if c.HTTPAddr != ":8080" || c.WebDir != "/app/web" || !c.CookieSecure || len(c.TrustedProxies) != 0 || c.ValkeyURL != "" || c.DatabaseURL != "postgres://x" ||
		c.RecordDecisions {
		t.Fatalf("defaults %+v", c)
	}
	t.Setenv("LH_COOKIE_SECURE", "false")
	t.Setenv("LH_VALKEY_URL", "redis://valkey:6379/0")
	t.Setenv("LH_ADMIN_USERNAME", "admin")
	t.Setenv("LH_ADMIN_PASSWORD", "secret")
	t.Setenv("LH_RECORD_DECISIONS", "true")
	c, _ = config.Load()
	if !c.RecordDecisions || c.CookieSecure || c.ValkeyURL != "redis://valkey:6379/0" || c.AdminUsername != "admin" || c.AdminPassword != "secret" {
		t.Fatalf("overrides %+v", c)
	}
	t.Setenv("LH_COOKIE_SECURE", "maybe")
	if _, err := config.Load(); err == nil {
		t.Fatal("a bad boolean is accepted")
	}
}

func TestConfigTimeScale(t *testing.T) {
	t.Setenv("LH_DATABASE_URL", "postgres://x")
	t.Setenv("LH_TIME_SCALE", "")
	t.Setenv("LH_TIME_EPOCH_MS", "")
	c, err := config.Load()
	if err != nil || c.TimeScale != 1 {
		t.Fatalf("default scale %v %v", c.TimeScale, err)
	}
	t.Setenv("LH_TIME_SCALE", "100")
	if _, err := config.Load(); err == nil {
		t.Fatal("a time scale without its epoch is accepted")
	}
	t.Setenv("LH_TIME_EPOCH_MS", "1800000000000")
	c, err = config.Load()
	if err != nil || c.TimeScale != 100 || c.TimeEpochMs != 1_800_000_000_000 {
		t.Fatalf("scale %+v %v", c, err)
	}
	t.Setenv("LH_TIME_SCALE", "0")
	if _, err := config.Load(); err == nil {
		t.Fatal("scale 0 accepted")
	}
}

func TestConfigInstantKeepAndSweep(t *testing.T) {
	t.Setenv("LH_DATABASE_URL", "postgres://x")
	t.Setenv("LH_INSTANT_KEEP_MIN", "")
	t.Setenv("LH_INSTANT_SWEEP_SEC", "")
	if c, err := config.Load(); err != nil || c.InstantKeep != 0 || c.InstantSweepEvery != 0 {
		t.Fatalf("unset: %v %v %v", c.InstantKeep, c.InstantSweepEvery, err)
	}
	t.Setenv("LH_INSTANT_KEEP_MIN", "90")
	t.Setenv("LH_INSTANT_SWEEP_SEC", "2")
	if c, err := config.Load(); err != nil || c.InstantKeep != 90*time.Minute || c.InstantSweepEvery != 2*time.Second {
		t.Fatalf("set: %v %v %v", c.InstantKeep, c.InstantSweepEvery, err)
	}
	t.Setenv("LH_INSTANT_SWEEP_SEC", "0")
	if _, err := config.Load(); err == nil {
		t.Fatal("a zero sweep is accepted")
	}
}

func TestConfigGuestsPerHour(t *testing.T) {
	t.Setenv("LH_DATABASE_URL", "postgres://x")
	t.Setenv("LH_GUESTS_PER_HOUR", "")
	if c, err := config.Load(); err != nil || c.GuestsPerHour != 0 {
		t.Fatalf("unset: %d %v", c.GuestsPerHour, err)
	}
	t.Setenv("LH_GUESTS_PER_HOUR", "40")
	if c, err := config.Load(); err != nil || c.GuestsPerHour != 40 {
		t.Fatalf("set: %d %v", c.GuestsPerHour, err)
	}
	t.Setenv("LH_GUESTS_PER_HOUR", "-1")
	if _, err := config.Load(); err == nil {
		t.Fatal("a negative number is accepted")
	}
}

func TestConfigRegistrationsPerHour(t *testing.T) {
	t.Setenv("LH_DATABASE_URL", "postgres://x")
	t.Setenv("LH_REGISTRATIONS_PER_HOUR", "")
	if c, err := config.Load(); err != nil || c.RegistrationsPerHour != 0 {
		t.Fatalf("unset: %d %v", c.RegistrationsPerHour, err)
	}
	t.Setenv("LH_REGISTRATIONS_PER_HOUR", "50")
	if c, err := config.Load(); err != nil || c.RegistrationsPerHour != 50 {
		t.Fatalf("set: %d %v", c.RegistrationsPerHour, err)
	}
	t.Setenv("LH_REGISTRATIONS_PER_HOUR", "lots")
	if _, err := config.Load(); err == nil {
		t.Fatal("a non-number is accepted")
	}
}

func TestConfigPublicURL(t *testing.T) {
	t.Setenv("LH_DATABASE_URL", "postgres://x")
	t.Setenv("LH_PUBLIC_URL", "")
	if c, err := config.Load(); err != nil || c.PublicURL != "" {
		t.Fatalf("unset: %q %v", c.PublicURL, err)
	}
	for v, want := range map[string]string{"https://example.com/": "https://example.com", "http://localhost:8080": "http://localhost:8080"} {
		t.Setenv("LH_PUBLIC_URL", v)
		if c, err := config.Load(); err != nil || c.PublicURL != want {
			t.Fatalf("%s: %q %v", v, c.PublicURL, err)
		}
	}
	for _, v := range []string{"example.com", "https://example.com/x", "ftp://x", "https://example.com?a=1", "https://example.com/#x", "https://"} {
		t.Setenv("LH_PUBLIC_URL", v)
		if _, err := config.Load(); err == nil {
			t.Errorf("%q is accepted", v)
		}
	}
}

func TestPlayerMaxWorldsFromEnv(t *testing.T) {
	t.Setenv("LH_DATABASE_URL", "postgres://x")
	t.Setenv("LH_PLAYER_MAX_WORLDS", "")
	if c, err := config.Load(); err != nil || c.PlayerMaxWorlds != 0 {
		t.Fatalf("unset: %d %v", c.PlayerMaxWorlds, err)
	}
	t.Setenv("LH_PLAYER_MAX_WORLDS", "12")
	if c, err := config.Load(); err != nil || c.PlayerMaxWorlds != 12 {
		t.Fatalf("set: %d %v", c.PlayerMaxWorlds, err)
	}
	t.Setenv("LH_PLAYER_MAX_WORLDS", "-3")
	if _, err := config.Load(); err == nil {
		t.Fatal("a negative number is accepted")
	}
}

func TestPlayerSweepFromEnv(t *testing.T) {
	t.Setenv("LH_DATABASE_URL", "postgres://x")
	t.Setenv("LH_PLAYER_SWEEP_SEC", "")
	if c, err := config.Load(); err != nil || c.PlayerSweepEvery != 0 {
		t.Fatalf("unset: %v %v", c.PlayerSweepEvery, err)
	}
	t.Setenv("LH_PLAYER_SWEEP_SEC", "3")
	if c, err := config.Load(); err != nil || c.PlayerSweepEvery != 3*time.Second {
		t.Fatalf("set: %v %v", c.PlayerSweepEvery, err)
	}
}

func TestConfigTrustedProxies(t *testing.T) {
	t.Setenv("LH_DATABASE_URL", "postgres://x")
	t.Setenv("LH_TRUSTED_PROXIES", "172.30.0.10, 10.0.0.0/8,fd00::1")
	c, err := config.Load()
	want := []netip.Prefix{netip.MustParsePrefix("172.30.0.10/32"), netip.MustParsePrefix("10.0.0.0/8"),
		netip.MustParsePrefix("fd00::1/128")}
	if err != nil || !slices.Equal(c.TrustedProxies, want) {
		t.Fatalf("%v %v", c.TrustedProxies, err)
	}
	t.Setenv("LH_TRUSTED_PROXIES", "caddy")
	if _, err := config.Load(); err == nil {
		t.Fatal("a host name is accepted as a proxy address")
	}
}

func TestConfigWorldRequestsPerMinute(t *testing.T) {
	t.Setenv("LH_DATABASE_URL", "postgres://x")
	t.Setenv("LH_WORLD_REQUESTS_PER_MINUTE", "")
	if c, err := config.Load(); err != nil || c.WorldRequestsPerMinute != 0 {
		t.Fatalf("unset: %d %v", c.WorldRequestsPerMinute, err)
	}
	t.Setenv("LH_WORLD_REQUESTS_PER_MINUTE", "600")
	if c, err := config.Load(); err != nil || c.WorldRequestsPerMinute != 600 {
		t.Fatalf("set: %d %v", c.WorldRequestsPerMinute, err)
	}
	t.Setenv("LH_WORLD_REQUESTS_PER_MINUTE", "-1")
	if _, err := config.Load(); err == nil {
		t.Fatal("a negative number is accepted")
	}
}
