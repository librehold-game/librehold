package web_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"librehold/internal/adapters/web"
)

func TestSecurityHeadersOnEveryResponse(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/missing" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte("ok"))
	})
	for _, https := range []bool{false, true} {
		h := web.SecurityHeaders(inner, https, "")
		for _, path := range []string{"/", "/api/v1/me", "/missing"} {
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, httptest.NewRequest("GET", path, nil))
			got := rec.Header()
			csp := got.Get("Content-Security-Policy")
			for _, want := range []string{"default-src 'self'", "script-src 'self'", "object-src 'none'", "frame-ancestors 'none'",
				"base-uri 'none'", "form-action 'self'"} {
				if !strings.Contains(csp, want) {
					t.Errorf("%s: policy %q lacks %q", path, csp, want)
				}
			}
			if strings.Contains(strings.Split(strings.SplitAfter(csp, "script-src")[1], ";")[0], "unsafe") {
				t.Errorf("%s: scripts may run inline: %q", path, csp)
			}
			for name, want := range map[string]string{"X-Frame-Options": "DENY", "X-Content-Type-Options": "nosniff",
				"Referrer-Policy": "strict-origin-when-cross-origin", "Cross-Origin-Opener-Policy": "same-origin"} {
				if got.Get(name) != want {
					t.Errorf("%s: %s = %q, want %q", path, name, got.Get(name), want)
				}
			}
			if hsts := got.Get("Strict-Transport-Security"); (hsts != "") != https {
				t.Errorf("%s over https=%v: Strict-Transport-Security %q", path, https, hsts)
			}
		}
	}
}

func TestSecurityHeadersNameTheSitesWebSocket(t *testing.T) {
	ok := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})
	for public, want := range map[string]string{"https://example.com": "connect-src 'self' wss://example.com",
		"http://192.168.1.2:8080": "connect-src 'self' ws://192.168.1.2:8080", "": "connect-src 'self';"} {
		rec := httptest.NewRecorder()
		web.SecurityHeaders(ok, false, public).ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
		if csp := rec.Header().Get("Content-Security-Policy"); !strings.Contains(csp, want) {
			t.Errorf("public URL %q: policy %q lacks %q", public, csp, want)
		}
	}
}
