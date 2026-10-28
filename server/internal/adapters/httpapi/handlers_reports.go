package httpapi

import (
	"net/http"

	"librehold/internal/app"
)

func (a *api) submitReport(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Kind      string `json:"kind"`
		Text      string `json:"text"`
		Email     string `json:"email"`
		WorldID   int64  `json:"worldId"`
		Screen    string `json:"screen"`
		Build     string `json:"build"`
		UserAgent string `json:"userAgent"`
	}
	if err := decode(r, &in); err != nil {
		writeError(w, err)
		return
	}
	id, err := a.svc.SubmitReport(r.Context(), a.optionalAccount(r), app.ReportInput{Kind: in.Kind, Text: in.Text,
		Email: in.Email, WorldID: in.WorldID, Screen: in.Screen, Build: in.Build, UserAgent: in.UserAgent, Client: a.clientKey(r)})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]int64{"id": id})
}

func (a *api) problemReports(w http.ResponseWriter, r *http.Request, acct *app.Account) {
	list, err := a.svc.ProblemReports(r.Context(), acct)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (a *api) setReportStatus(w http.ResponseWriter, r *http.Request, acct *app.Account) {
	var in struct {
		Status string `json:"status"`
	}
	id, err := pathID(r, "id")
	if err == nil {
		err = decode(r, &in)
	}
	if err == nil {
		err = a.svc.SetProblemReportStatus(r.Context(), acct, id, in.Status)
	}
	if err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
