// Package httpapi is the JSON API of the server under /api/v1.
package httpapi

import (
	"net/http"
	"net/netip"
	"sync"

	"librehold/internal/app"
)

type Options struct {
	CookieSecure bool
	// TrustedProxies are the reverse proxies whose X-Forwarded-For names the client.
	TrustedProxies []netip.Prefix
}

type api struct {
	svc          *app.Service
	opts         Options
	proxyWarning sync.Once
}

func New(svc *app.Service, opts Options) http.Handler {
	a := &api{svc: svc, opts: opts}
	mux := http.NewServeMux()

	mux.HandleFunc("POST /api/v1/auth/register", a.register)
	mux.HandleFunc("POST /api/v1/auth/login", a.login)
	mux.HandleFunc("POST /api/v1/auth/logout", a.logout)
	mux.HandleFunc("POST /api/v1/auth/guest", a.guest)
	mux.HandleFunc("POST /api/v1/auth/claim", a.authed(a.claim))
	mux.HandleFunc("GET /api/v1/me", a.authed(a.me))
	mux.HandleFunc("PATCH /api/v1/me", a.authed(a.updateMe))

	mux.HandleFunc("GET /api/v1/public/stats", a.publicStats)
	mux.HandleFunc("GET /api/v1/public/live", a.publicLive)
	mux.HandleFunc("GET /api/v1/public/worlds", a.publicWorlds)
	mux.HandleFunc("GET /api/v1/public/worlds/{wid}", a.publicWorld)
	mux.HandleFunc("GET /api/v1/public/worlds/{wid}/villages", a.publicMap)
	mux.HandleFunc("GET /api/v1/public/worlds/{wid}/ranking", a.publicRanking)
	mux.HandleFunc("GET /api/v1/public/worlds/{wid}/players/{pid}", a.publicWorldPlayer)
	mux.HandleFunc("GET /api/v1/public/games/{ref}", a.publicGame)
	mux.HandleFunc("GET /api/v1/public/games/{ref}/timeline", a.publicTimeline)
	mux.HandleFunc("GET /api/v1/public/profiles/{username}", a.publicProfile)

	mux.HandleFunc("GET /api/v1/instant/games", a.authed(a.instantGames))
	mux.HandleFunc("POST /api/v1/instant/play", a.authed(a.play))
	mux.HandleFunc("POST /api/v1/hosted", a.authed(a.createHosted))
	mux.HandleFunc("GET /api/v1/public/invites/{code}", a.inviteInfo)
	mux.HandleFunc("POST /api/v1/invites/{code}/join", a.authed(a.joinByInvite))
	mux.HandleFunc("POST /api/v1/worlds/{wid}/host/{action}", a.authed(a.hostAction))
	mux.HandleFunc("PATCH /api/v1/worlds/{wid}/host", a.authed(a.setJoinRule))
	mux.HandleFunc("GET /api/v1/worlds/{wid}/watch", a.authed(a.watch))
	mux.HandleFunc("GET /api/v1/worlds/{wid}/watch/villages/{vid}", a.authed(a.watchVillage))
	mux.HandleFunc("GET /api/v1/worlds", a.authed(a.lobby))
	mux.HandleFunc("POST /api/v1/worlds/{wid}/join", a.authed(a.join))
	mux.HandleFunc("POST /api/v1/worlds/{wid}/leave", a.authed(a.leave))
	mux.HandleFunc("GET /api/v1/worlds/{wid}/state", a.authed(a.state))
	mux.HandleFunc("POST /api/v1/worlds/{wid}/orders/{name}", a.authed(a.order))
	mux.HandleFunc("GET /api/v1/worlds/{wid}/reports", a.authed(a.reports))
	mux.HandleFunc("GET /api/v1/worlds/{wid}/map", a.authed(a.mapRegion))
	mux.HandleFunc("GET /api/v1/worlds/{wid}/ranking", a.authed(a.ranking))
	mux.HandleFunc("GET /api/v1/worlds/{wid}/players/{pid}", a.authed(a.profile))
	mux.HandleFunc("GET /api/v1/worlds/{wid}/news", a.authed(a.news))
	mux.HandleFunc("GET /api/v1/worlds/{wid}/farm-targets", a.authed(a.farmTargets))

	mux.HandleFunc("GET /api/v1/admin/accounts", a.authed(a.accounts))
	mux.HandleFunc("POST /api/v1/admin/accounts/{id}/password", a.authed(a.resetPassword))
	mux.HandleFunc("GET /api/v1/admin/worlds", a.authed(a.adminWorlds))
	mux.HandleFunc("POST /api/v1/admin/worlds", a.authed(a.createWorld))
	mux.HandleFunc("PATCH /api/v1/admin/worlds/{wid}", a.authed(a.updateWorld))
	mux.HandleFunc("POST /api/v1/admin/worlds/{wid}/pause", a.authed(a.pauseWorld))
	mux.HandleFunc("POST /api/v1/admin/worlds/{wid}/resume", a.authed(a.resumeWorld))
	mux.HandleFunc("GET /api/v1/admin/reports", a.authed(a.problemReports))
	mux.HandleFunc("GET /api/v1/admin/host-settings", a.authed(a.hostSettings))
	mux.HandleFunc("PUT /api/v1/admin/host-settings", a.authed(a.setHostSettings))
	mux.HandleFunc("PATCH /api/v1/admin/reports/{id}", a.authed(a.setReportStatus))

	mux.HandleFunc("POST /api/v1/reports", a.submitReport)

	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		writeAPIError(w, http.StatusNotFound, apiError{Code: "notFound"})
	})
	return a.guard(mux)
}
