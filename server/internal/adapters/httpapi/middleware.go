package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"mime"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"time"

	"librehold/internal/app"
)

const (
	cookieName  = "lh_session"
	cookieAge   = 30 * 24 * time.Hour
	maxBodySize = 1 << 20
)

// bodyTimeout is how long a request may take to send its body; a client that stalls is cut off.
var bodyTimeout = 30 * time.Second

type ctxKey struct{}

type authInfo struct {
	token  string
	bearer bool
}

// tokenOf reads the session token from the Authorization header, else the cookie.
func tokenOf(r *http.Request) authInfo {
	if h := r.Header.Get("Authorization"); strings.HasPrefix(h, "Bearer ") {
		return authInfo{token: strings.TrimPrefix(h, "Bearer "), bearer: true}
	}
	if c, err := r.Cookie(cookieName); err == nil {
		return authInfo{token: c.Value}
	}
	return authInfo{}
}

// sameOrigin is the CSRF guard of every state-changing request a browser could send: a JSON body and an
// Origin naming this host. Requests carrying a bearer token are not sent by browsers on their own.
func sameOrigin(r *http.Request) bool {
	if r.Method == http.MethodGet || r.Method == http.MethodHead || tokenOf(r).bearer {
		return true
	}
	if r.ContentLength != 0 || r.Header.Get("Content-Type") != "" {
		mt, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if err != nil || mt != "application/json" {
			return false
		}
	}
	origin, err := url.Parse(r.Header.Get("Origin"))
	return err == nil && origin.Host != "" && origin.Host == r.Host
}

func (a *api) guard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !sameOrigin(r) {
			writeAPIError(w, http.StatusForbidden, apiError{Code: "csrf"})
			return
		}
		_ = http.NewResponseController(w).SetReadDeadline(time.Now().Add(bodyTimeout))
		r.Body = http.MaxBytesReader(w, r.Body, maxBodySize)
		next.ServeHTTP(w, r)
	})
}

// authed resolves the session and passes the account on; without one it answers 401.
func (a *api) authed(h func(w http.ResponseWriter, r *http.Request, acct *app.Account)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		info := tokenOf(r)
		if info.token == "" {
			writeError(w, app.ErrUnauthenticated)
			return
		}
		acct, err := a.svc.Authenticate(r.Context(), info.token)
		if err != nil {
			if errors.Is(err, app.ErrUnauthenticated) && !info.bearer {
				a.clearCookie(w)
			}
			writeError(w, err)
			return
		}
		h(w, r.WithContext(context.WithValue(r.Context(), ctxKey{}, acct)), acct)
	}
}

// optionalAccount is the session's account, or nil without one; a stale cookie is left alone.
func (a *api) optionalAccount(r *http.Request) *app.Account {
	info := tokenOf(r)
	if info.token == "" {
		return nil
	}
	acct, err := a.svc.Authenticate(r.Context(), info.token)
	if err != nil {
		return nil
	}
	return acct
}

func (a *api) setCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{Name: cookieName, Value: token, Path: "/", MaxAge: int(cookieAge.Seconds()),
		HttpOnly: true, SameSite: http.SameSiteLaxMode, Secure: a.opts.CookieSecure})
}

func (a *api) clearCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{Name: cookieName, Value: "", Path: "/", MaxAge: -1,
		HttpOnly: true, SameSite: http.SameSiteLaxMode, Secure: a.opts.CookieSecure})
}

// clientKey identifies the client for the limits: the connection's address, or, when that is a trusted proxy, the
// nearest address in X-Forwarded-For that is not one; entries left of it could be written by anyone. An IPv6
// client counts by its /64, which one household holds whole. A client on the server's own network, as at a LAN
// party, is app.LocalClient: everyone there may come through one gateway address.
func (a *api) clientKey(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	remote, err := netip.ParseAddr(host)
	if err != nil {
		return host
	}
	remote = remote.Unmap()
	forwarded := strings.Join(r.Header.Values("X-Forwarded-For"), ",")
	if !a.trusted(remote) {
		if forwarded != "" && local(remote) {
			a.proxyWarning.Do(func() {
				slog.Warn("X-Forwarded-For from an untrusted proxy: set LH_TRUSTED_PROXIES to its address, or every client behind it shares one limit", "proxy", remote)
			})
			return clientOf(remote)
		}
		if local(remote) || onLink(r, remote) {
			return app.LocalClient
		}
		return clientOf(remote)
	}
	hops := strings.Split(forwarded, ",")
	for i := len(hops) - 1; i >= 0; i-- {
		hop, err := netip.ParseAddr(strings.TrimSpace(hops[i]))
		if err != nil {
			a.proxyWarning.Do(func() {
				slog.Warn("a trusted proxy sent no usable X-Forwarded-For: every client behind it shares one limit", "proxy", remote)
			})
			return clientOf(remote)
		}
		if hop = hop.Unmap(); !a.trusted(hop) || i == 0 {
			if local(hop) {
				return app.LocalClient
			}
			return clientOf(hop)
		}
	}
	return clientOf(remote)
}

func clientOf(addr netip.Addr) string {
	if addr.Is6() {
		return netip.PrefixFrom(addr.WithZone(""), 64).Masked().String()
	}
	return addr.String()
}

func local(addr netip.Addr) bool {
	return addr.IsPrivate() || addr.IsLoopback() || addr.IsLinkLocalUnicast()
}

// onLink is an IPv6 client in the /64 of the address it reached the server on: a neighbour on the same network.
func onLink(r *http.Request, client netip.Addr) bool {
	at, ok := r.Context().Value(http.LocalAddrContextKey).(net.Addr)
	if !ok || !client.Is6() {
		return false
	}
	server, err := netip.ParseAddrPort(at.String())
	if err != nil {
		return false
	}
	return netip.PrefixFrom(server.Addr().WithZone(""), 64).Masked().Contains(client.WithZone(""))
}

func (a *api) trusted(addr netip.Addr) bool {
	addr = addr.Unmap()
	for _, p := range a.opts.TrustedProxies {
		if p.Contains(addr) {
			return true
		}
	}
	return false
}

func decode(r *http.Request, into any) error {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return &app.ErrValidation{Field: "body", Code: "invalid"}
	}
	if len(strings.TrimSpace(string(body))) == 0 {
		return nil
	}
	if err := json.Unmarshal(body, into); err != nil {
		return &app.ErrValidation{Field: "body", Code: "invalid"}
	}
	return nil
}

func pathID(r *http.Request, name string) (int64, error) {
	id, err := strconv.ParseInt(r.PathValue(name), 10, 64)
	if err != nil {
		return 0, &app.ErrValidation{Field: name, Code: "invalid"}
	}
	return id, nil
}

// queryInt reads an optional integer query parameter.
func queryInt(r *http.Request, name string, def int64) (int64, error) {
	s := r.URL.Query().Get(name)
	if s == "" {
		return def, nil
	}
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, &app.ErrValidation{Field: name, Code: "invalid"}
	}
	return v, nil
}
