package httpapi

import (
	"net/http"

	"librehold/internal/app"
)

func (a *api) play(w http.ResponseWriter, r *http.Request, acct *app.Account) {
	res, err := a.svc.Play(r.Context(), acct)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}
