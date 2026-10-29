package httpapi

import (
	"net/http"

	"librehold/internal/app"
)

func (a *api) instantGames(w http.ResponseWriter, r *http.Request, acct *app.Account) {
	list, err := a.svc.InstantGames(r.Context(), acct)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (a *api) play(w http.ResponseWriter, r *http.Request, acct *app.Account) {
	var in struct {
		WorldID int64 `json:"worldId"`
	}
	if err := decode(r, &in); err != nil {
		writeError(w, err)
		return
	}
	res, err := a.svc.Play(r.Context(), acct, in.WorldID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}
