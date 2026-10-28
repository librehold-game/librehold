package web

import (
	"net/http"
	"strings"
)

// policyFor lets pages run only the build's own scripts. Styles may be inline: the prerendered pages carry style
// attributes and the bundle adds style elements. Images and clips may come from data: and blob: addresses, which the
// sprites and the timelapse recorder make. The site's own WebSocket is named as well when its address is known:
// older Safari does not count ws: and wss: as 'self'.
func policyFor(publicURL string) string {
	connect := "connect-src 'self'"
	if host, ok := strings.CutPrefix(publicURL, "https://"); ok {
		connect += " wss://" + host
	} else if host, ok := strings.CutPrefix(publicURL, "http://"); ok {
		connect += " ws://" + host
	}
	return strings.Join([]string{
		"default-src 'self'",
		"script-src 'self'",
		"style-src 'self' 'unsafe-inline'",
		"img-src 'self' data: blob:",
		"media-src 'self' blob:",
		connect,
		"object-src 'none'",
		"base-uri 'none'",
		"form-action 'self'",
		"frame-ancestors 'none'",
	}, "; ")
}

// SecurityHeaders sets the browser's protections on every response: the content policy, no framing, no type
// sniffing, a short referrer, a window of its own; over https also HSTS. publicURL is the site's origin, or empty.
func SecurityHeaders(next http.Handler, https bool, publicURL string) http.Handler {
	policy := policyFor(publicURL)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Content-Security-Policy", policy)
		h.Set("X-Frame-Options", "DENY")
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		h.Set("Cross-Origin-Opener-Policy", "same-origin")
		if https {
			h.Set("Strict-Transport-Security", "max-age=31536000")
		}
		next.ServeHTTP(w, r)
	})
}
