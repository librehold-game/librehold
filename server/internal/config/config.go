// Package config reads the server's settings from the environment.
package config

import (
	"errors"
	"fmt"
	"net/netip"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	DatabaseURL  string
	HTTPAddr     string
	WebDir       string
	CookieSecure bool
	// TrustedProxies are the reverse proxies whose X-Forwarded-For names the client; empty trusts none.
	TrustedProxies []netip.Prefix
	// ValkeyURL is where the limits are counted, shared by every server process; empty counts them in this one.
	ValkeyURL     string
	AdminUsername string
	AdminPassword string
	// RegistrationsPerHour caps new accounts per client address; 0 keeps the server's default.
	RegistrationsPerHour int
	// GuestsPerHour caps new guests per client address; 0 keeps the server's default.
	GuestsPerHour int
	// WorldRequestsPerMinute caps state reads and orders per account; 0 keeps the server's default.
	WorldRequestsPerMinute int
	// InstantMaxWorlds caps instant worlds running at once; 0 keeps the server's default.
	InstantMaxWorlds int
	// PlayerMaxWorlds caps player-hosted worlds waiting or running at once; 0 keeps the server's default.
	PlayerMaxWorlds int
	// PlayerSweepEvery is how often player worlds are tended (capped pauses, abandoned worlds); 0 keeps the default.
	PlayerSweepEvery time.Duration
	// InstantKeep is how long an ended instant world stays readable; InstantSweepEvery how often the instant
	// worlds are tended. 0 keeps the server's defaults.
	InstantKeep       time.Duration
	InstantSweepEvery time.Duration
	// TimeScale > 1 runs the server's clock that many times faster from TimeEpochMs on — for end-to-end tests
	// only; a fixed epoch keeps a restarted process on the same timeline.
	TimeScale   float64
	TimeEpochMs int64
	// PublicURL is the site's origin (https://example.com) for canonical links and previews; empty uses the
	// request's own scheme and host.
	PublicURL string
}

func env(name, def string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return def
}

func boolEnv(name string, def bool) (bool, error) {
	v := os.Getenv(name)
	if v == "" {
		return def, nil
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return false, fmt.Errorf("%s: %q is not true or false", name, v)
	}
	return b, nil
}

func Load() (Config, error) {
	c := Config{
		DatabaseURL:   os.Getenv("LH_DATABASE_URL"),
		HTTPAddr:      env("LH_HTTP_ADDR", ":8080"),
		WebDir:        env("LH_WEB_DIR", "/app/web"),
		AdminUsername: os.Getenv("LH_ADMIN_USERNAME"),
		AdminPassword: os.Getenv("LH_ADMIN_PASSWORD"),
	}
	if c.DatabaseURL == "" {
		return c, errors.New("LH_DATABASE_URL is required")
	}
	var err error
	if c.CookieSecure, err = boolEnv("LH_COOKIE_SECURE", true); err != nil {
		return c, err
	}
	if c.TrustedProxies, err = prefixes("LH_TRUSTED_PROXIES"); err != nil {
		return c, err
	}
	c.ValkeyURL = os.Getenv("LH_VALKEY_URL")
	for name, url := range map[string]string{"LH_DATABASE_URL": c.DatabaseURL, "LH_VALKEY_URL": c.ValkeyURL} {
		if strings.Contains(url, "change-me") {
			return c, fmt.Errorf("%s still has the example password from .env.example; set a real one", name)
		}
	}
	if c.RegistrationsPerHour, err = positiveInt("LH_REGISTRATIONS_PER_HOUR"); err != nil {
		return c, err
	}
	if c.GuestsPerHour, err = positiveInt("LH_GUESTS_PER_HOUR"); err != nil {
		return c, err
	}
	if c.WorldRequestsPerMinute, err = positiveInt("LH_WORLD_REQUESTS_PER_MINUTE"); err != nil {
		return c, err
	}
	if c.InstantMaxWorlds, err = positiveInt("LH_INSTANT_MAX_WORLDS"); err != nil {
		return c, err
	}
	if c.PlayerMaxWorlds, err = positiveInt("LH_PLAYER_MAX_WORLDS"); err != nil {
		return c, err
	}
	playerSweepSec, err := positiveInt("LH_PLAYER_SWEEP_SEC")
	if err != nil {
		return c, err
	}
	c.PlayerSweepEvery = time.Duration(playerSweepSec) * time.Second
	keepMin, err := positiveInt("LH_INSTANT_KEEP_MIN")
	if err != nil {
		return c, err
	}
	sweepSec, err := positiveInt("LH_INSTANT_SWEEP_SEC")
	if err != nil {
		return c, err
	}
	c.InstantKeep, c.InstantSweepEvery = time.Duration(keepMin)*time.Minute, time.Duration(sweepSec)*time.Second
	if c.PublicURL, err = origin("LH_PUBLIC_URL"); err != nil {
		return c, err
	}
	c.TimeScale = 1
	if v := os.Getenv("LH_TIME_SCALE"); v != "" {
		if c.TimeScale, err = strconv.ParseFloat(v, 64); err != nil || c.TimeScale <= 0 {
			return c, fmt.Errorf("LH_TIME_SCALE: %q is not a positive number", v)
		}
		if c.TimeEpochMs, err = strconv.ParseInt(os.Getenv("LH_TIME_EPOCH_MS"), 10, 64); err != nil {
			return c, errors.New("LH_TIME_SCALE needs LH_TIME_EPOCH_MS")
		}
	}
	return c, nil
}

// origin reads an optional http(s) origin with no path, query or fragment; a trailing slash is dropped.
func origin(name string) (string, error) {
	v := os.Getenv(name)
	if v == "" {
		return "", nil
	}
	u, err := url.Parse(v)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || (u.Path != "" && u.Path != "/") ||
		u.RawQuery != "" || u.Fragment != "" || u.ForceQuery || strings.HasSuffix(v, "#") || u.User != nil {
		return "", fmt.Errorf("%s: %q is not an origin such as https://example.com", name, v)
	}
	return u.Scheme + "://" + u.Host, nil
}

// prefixes reads an optional comma-separated list of IP addresses and CIDR ranges; an address is a range of one.
func prefixes(name string) ([]netip.Prefix, error) {
	var out []netip.Prefix
	for _, part := range strings.Split(os.Getenv(name), ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		p, err := netip.ParsePrefix(part)
		if err != nil {
			addr, aerr := netip.ParseAddr(part)
			if aerr != nil {
				return nil, fmt.Errorf("%s: %q is not an IP address or a CIDR range", name, part)
			}
			p = netip.PrefixFrom(addr, addr.BitLen())
		}
		out = append(out, p.Masked())
	}
	return out, nil
}

// positiveInt reads an optional whole-number setting; unset is 0.
func positiveInt(name string) (int, error) {
	v := os.Getenv(name)
	if v == "" {
		return 0, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("%s: %q is not a positive number", name, v)
	}
	return n, nil
}
