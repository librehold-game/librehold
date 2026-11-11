package web_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"librehold/internal/adapters/web"
)

func TestHealth(t *testing.T) {
	for _, tc := range []struct {
		ping   error
		status int
		body   string
	}{{nil, 200, "ok"}, {errors.New("connection refused"), 503, "database unreachable\n"}} {
		var within time.Duration
		res, body := get(t, web.Health(func(ctx context.Context) error {
			if dl, ok := ctx.Deadline(); ok {
				within = time.Until(dl)
			}
			return tc.ping
		}), "/healthz")
		if res.StatusCode != tc.status || body != tc.body || res.Header.Get("Cache-Control") != "no-store" {
			t.Fatalf("ping %v: %d %q %v", tc.ping, res.StatusCode, body, res.Header)
		}
		if within <= 0 || within > 2*time.Second {
			t.Fatalf("ping %v: deadline in %v", tc.ping, within)
		}
	}
}
