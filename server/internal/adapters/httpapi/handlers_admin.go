package httpapi

import (
	"encoding/json"
	"net/http"
	"time"

	"librehold/internal/app"
	"librehold/internal/game"
)

type adminAccountJSON struct {
	accountJSON
	CreatedAt time.Time `json:"createdAt"`
}

type adminWorldJSON struct {
	ID        int64            `json:"id"`
	Name      string           `json:"name"`
	Status    string           `json:"status"`
	Humans    int              `json:"humans"`
	Bots      int              `json:"bots"`
	Paused    bool             `json:"paused"`
	StartedAt int64            `json:"startedAt"`
	Settings  game.WorldConfig `json:"settings"`
}

func (a *api) accounts(w http.ResponseWriter, r *http.Request, acct *app.Account) {
	list, err := a.svc.Accounts(r.Context(), acct)
	if err != nil {
		writeError(w, err)
		return
	}
	out := make([]adminAccountJSON, 0, len(list))
	for i := range list {
		out = append(out, adminAccountJSON{accountJSON: accountOf(&list[i]), CreatedAt: list[i].CreatedAt})
	}
	writeJSON(w, http.StatusOK, out)
}

func (a *api) resetPassword(w http.ResponseWriter, r *http.Request, acct *app.Account) {
	var in struct {
		Password string `json:"password"`
	}
	id, err := pathID(r, "id")
	if err == nil {
		err = decode(r, &in)
	}
	if err == nil {
		err = a.svc.ResetPassword(r.Context(), acct, id, in.Password)
	}
	if err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *api) adminWorlds(w http.ResponseWriter, r *http.Request, acct *app.Account) {
	list, err := a.svc.Worlds(r.Context(), acct)
	if err != nil {
		writeError(w, err)
		return
	}
	out := make([]adminWorldJSON, 0, len(list))
	for _, s := range list {
		out = append(out, adminWorldJSON{ID: s.ID, Name: s.Name, Status: s.Status, Humans: s.Humans, Bots: s.Bots,
			Paused: s.PausedAtMs != nil, StartedAt: s.StartedAtMs, Settings: s.Config})
	}
	writeJSON(w, http.StatusOK, out)
}

// createWorld takes a name and settings; settings left out keep the defaults.
func (a *api) createWorld(w http.ResponseWriter, r *http.Request, acct *app.Account) {
	in := struct {
		Name     string           `json:"name"`
		Settings game.WorldConfig `json:"settings"`
	}{Settings: game.DefaultWorldConfig}
	if err := decode(r, &in); err != nil {
		writeError(w, err)
		return
	}
	id, err := a.svc.CreateWorld(r.Context(), acct, in.Name, in.Settings)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]int64{"id": id})
}

func (a *api) updateWorld(w http.ResponseWriter, r *http.Request, acct *app.Account) {
	var in struct {
		Settings map[string]json.RawMessage `json:"settings"`
		Status   *string                    `json:"status"`
	}
	wid, err := pathID(r, "wid")
	if err == nil {
		err = decode(r, &in)
	}
	if err == nil && len(in.Settings) > 0 {
		err = a.svc.UpdateWorldSettings(r.Context(), acct, wid, app.WorldSettingsPatch(in.Settings))
	}
	if err == nil && in.Status != nil {
		err = a.svc.SetWorldStatus(r.Context(), acct, wid, *in.Status)
	}
	if err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *api) pauseWorld(w http.ResponseWriter, r *http.Request, acct *app.Account) {
	a.inWorld(w, r, func(wid int64) (any, error) { return nil, a.svc.PauseWorld(r.Context(), acct, wid) })
}

func (a *api) resumeWorld(w http.ResponseWriter, r *http.Request, acct *app.Account) {
	a.inWorld(w, r, func(wid int64) (any, error) { return nil, a.svc.ResumeWorld(r.Context(), acct, wid) })
}
