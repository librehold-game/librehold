package httpapi

import (
	"net/http"

	"librehold/internal/game"
)

func (a *api) publicStats(w http.ResponseWriter, r *http.Request) {
	stats, err := a.svc.PublicStats(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, stats)
}

func (a *api) publicLive(w http.ResponseWriter, r *http.Request) {
	l, err := a.svc.Live(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, l)
}

func (a *api) publicWorlds(w http.ResponseWriter, r *http.Request) {
	l, err := a.svc.PublicWorlds(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, l)
}

func (a *api) publicWorld(w http.ResponseWriter, r *http.Request) {
	wid, err := pathID(r, "wid")
	if err != nil {
		writeError(w, err)
		return
	}
	pw, err := a.svc.PublicWorld(r.Context(), wid)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, pw)
}

func (a *api) publicMap(w http.ResponseWriter, r *http.Request) {
	wid, err := pathID(r, "wid")
	if err != nil {
		writeError(w, err)
		return
	}
	m, err := a.svc.PublicMap(r.Context(), wid)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, m)
}

func (a *api) publicRanking(w http.ResponseWriter, r *http.Request) {
	wid, err := pathID(r, "wid")
	if err != nil {
		writeError(w, err)
		return
	}
	tab := r.URL.Query().Get("tab")
	if tab == "" {
		tab = "points"
	}
	rows, err := a.svc.PublicRanking(r.Context(), wid, tab)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, rows)
}

func (a *api) publicWorldPlayer(w http.ResponseWriter, r *http.Request) {
	wid, err := pathID(r, "wid")
	if err != nil {
		writeError(w, err)
		return
	}
	p, err := a.svc.PublicWorldPlayer(r.Context(), wid, game.PlayerID(r.PathValue("pid")))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (a *api) publicGame(w http.ResponseWriter, r *http.Request) {
	wid, err := a.svc.ResolveGameRef(r.Context(), r.PathValue("ref"))
	if err != nil {
		writeError(w, err)
		return
	}
	g, err := a.svc.PublicGame(r.Context(), wid)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, g)
}

// publicTimeline sends the stored gzip blob as it is.
func (a *api) publicTimeline(w http.ResponseWriter, r *http.Request) {
	wid, err := a.svc.ResolveGameRef(r.Context(), r.PathValue("ref"))
	if err != nil {
		writeError(w, err)
		return
	}
	blob, err := a.svc.GameTimeline(r.Context(), wid)
	if err != nil {
		writeError(w, err)
		return
	}
	h := w.Header()
	h.Set("Content-Type", "application/json")
	h.Set("Content-Encoding", "gzip")
	h.Set("Cache-Control", "public, max-age=3600")
	_, _ = w.Write(blob)
}

func (a *api) publicProfile(w http.ResponseWriter, r *http.Request) {
	p, err := a.svc.PublicProfile(r.Context(), r.PathValue("username"), a.optionalAccount(r))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, p)
}
