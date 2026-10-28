package httpapi

import (
	"encoding/json"
	"io"
	"net/http"

	"librehold/internal/app"
	"librehold/internal/game"
)

// inWorld parses {wid} and answers the handler's result as JSON, or its error.
func (a *api) inWorld(w http.ResponseWriter, r *http.Request, fn func(wid int64) (any, error)) {
	wid, err := pathID(r, "wid")
	if err != nil {
		writeError(w, err)
		return
	}
	out, err := fn(wid)
	if err != nil {
		writeError(w, err)
		return
	}
	if out == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (a *api) lobby(w http.ResponseWriter, r *http.Request, acct *app.Account) {
	view, err := a.svc.Lobby(r.Context(), acct)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, view)
}

var quadrants = map[string]game.Quadrant{"": game.QuadrantRandom, "random": game.QuadrantRandom,
	"nw": game.QuadrantNW, "ne": game.QuadrantNE, "sw": game.QuadrantSW, "se": game.QuadrantSE}

func (a *api) join(w http.ResponseWriter, r *http.Request, acct *app.Account) {
	a.inWorld(w, r, func(wid int64) (any, error) {
		var in struct {
			Quadrant string `json:"quadrant"`
		}
		if err := decode(r, &in); err != nil {
			return nil, err
		}
		q, ok := quadrants[in.Quadrant]
		if !ok {
			return nil, &app.ErrValidation{Field: "quadrant", Code: "unknown"}
		}
		id, err := a.svc.JoinWorld(r.Context(), acct, wid, q)
		if err != nil {
			return nil, err
		}
		return map[string]game.PlayerID{"playerId": id}, nil
	})
}

func (a *api) leave(w http.ResponseWriter, r *http.Request, acct *app.Account) {
	a.inWorld(w, r, func(wid int64) (any, error) { return nil, a.svc.LeaveWorld(r.Context(), acct, wid) })
}

func (a *api) state(w http.ResponseWriter, r *http.Request, acct *app.Account) {
	a.inWorld(w, r, func(wid int64) (any, error) { return a.svc.State(r.Context(), acct, wid) })
}

func (a *api) order(w http.ResponseWriter, r *http.Request, acct *app.Account) {
	a.inWorld(w, r, func(wid int64) (any, error) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			return nil, &app.ErrValidation{Field: "body", Code: "invalid"}
		}
		return a.svc.Order(r.Context(), acct, wid, r.PathValue("name"), json.RawMessage(body))
	})
}

func (a *api) reports(w http.ResponseWriter, r *http.Request, acct *app.Account) {
	a.inWorld(w, r, func(wid int64) (any, error) {
		before, err := queryInt(r, "before", 0)
		if err != nil {
			return nil, err
		}
		limit, err := queryInt(r, "limit", 50)
		if err != nil {
			return nil, err
		}
		return a.svc.Reports(r.Context(), acct, wid, before, int(limit))
	})
}

func (a *api) mapRegion(w http.ResponseWriter, r *http.Request, acct *app.Account) {
	a.inWorld(w, r, func(wid int64) (any, error) {
		var c [4]int
		for i, name := range []string{"x0", "y0", "x1", "y1"} {
			v, err := queryInt(r, name, 0)
			if err != nil {
				return nil, err
			}
			c[i] = int(v)
		}
		return a.svc.MapRegion(r.Context(), acct, wid, c[0], c[1], c[2], c[3])
	})
}

func (a *api) ranking(w http.ResponseWriter, r *http.Request, acct *app.Account) {
	a.inWorld(w, r, func(wid int64) (any, error) {
		tab := r.URL.Query().Get("tab")
		if tab == "" {
			tab = "points"
		}
		return a.svc.Ranking(r.Context(), acct, wid, tab)
	})
}

func (a *api) profile(w http.ResponseWriter, r *http.Request, acct *app.Account) {
	a.inWorld(w, r, func(wid int64) (any, error) { return a.svc.Profile(r.Context(), acct, wid, r.PathValue("pid")) })
}

func (a *api) farmTargets(w http.ResponseWriter, r *http.Request, acct *app.Account) {
	a.inWorld(w, r, func(wid int64) (any, error) {
		return a.svc.FarmTargets(r.Context(), acct, wid, r.URL.Query().Get("village"))
	})
}

func (a *api) news(w http.ResponseWriter, r *http.Request, acct *app.Account) {
	a.inWorld(w, r, func(wid int64) (any, error) {
		limit, err := queryInt(r, "limit", 50)
		if err != nil {
			return nil, err
		}
		return a.svc.News(r.Context(), acct, wid, int(limit))
	})
}
