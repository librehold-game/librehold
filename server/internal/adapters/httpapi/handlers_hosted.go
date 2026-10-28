package httpapi

import (
	"net/http"

	"librehold/internal/app"
	"librehold/internal/game"
)

func (a *api) createHosted(w http.ResponseWriter, r *http.Request, acct *app.Account) {
	var in app.HostForm
	if err := decode(r, &in); err != nil {
		writeError(w, err)
		return
	}
	card, err := a.svc.CreatePlayerWorld(r.Context(), acct, in)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, card)
}

func (a *api) hostSettings(w http.ResponseWriter, r *http.Request, acct *app.Account) {
	if !acct.IsAdmin {
		writeError(w, app.ErrForbidden)
		return
	}
	s, err := a.svc.HostSettings(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, s)
}

func (a *api) setHostSettings(w http.ResponseWriter, r *http.Request, acct *app.Account) {
	var in app.HostSettings
	if err := decode(r, &in); err != nil {
		writeError(w, err)
		return
	}
	s, err := a.svc.SetHostSettings(r.Context(), acct, in)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, s)
}

func (a *api) inviteInfo(w http.ResponseWriter, r *http.Request) {
	info, err := a.svc.InviteInfo(r.Context(), r.PathValue("code"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, info)
}

func (a *api) joinByInvite(w http.ResponseWriter, r *http.Request, acct *app.Account) {
	res, err := a.svc.JoinByInvite(r.Context(), acct, r.PathValue("code"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

// hostAction runs one of the host's controls and answers with the world as the host now sees it.
func (a *api) hostAction(w http.ResponseWriter, r *http.Request, acct *app.Account) {
	wid, err := pathID(r, "wid")
	if err != nil {
		writeError(w, err)
		return
	}
	ctx := r.Context()
	switch r.PathValue("action") {
	case "start":
		err = a.svc.HostStart(ctx, acct, wid)
	case "pause":
		err = a.svc.HostPause(ctx, acct, wid)
	case "resume":
		err = a.svc.HostResume(ctx, acct, wid)
	case "end":
		err = a.svc.HostEnd(ctx, acct, wid)
	case "kick":
		var in struct {
			PlayerID game.PlayerID `json:"playerId"`
		}
		if err = decode(r, &in); err == nil {
			err = a.svc.HostKick(ctx, acct, wid, in.PlayerID)
		}
	case "invite":
		code, err := a.svc.ReplaceInvite(ctx, acct, wid)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"inviteCode": code})
		return
	default:
		err = app.ErrNotFound
	}
	if err != nil {
		writeError(w, err)
		return
	}
	a.writeHostCard(w, r, acct, wid)
}

func (a *api) setJoinRule(w http.ResponseWriter, r *http.Request, acct *app.Account) {
	var in struct {
		RegisteredOnly bool `json:"registeredOnly"`
	}
	wid, err := pathID(r, "wid")
	if err == nil {
		err = decode(r, &in)
	}
	if err == nil {
		err = a.svc.SetRegisteredOnly(r.Context(), acct, wid, in.RegisteredOnly)
	}
	if err != nil {
		writeError(w, err)
		return
	}
	a.writeHostCard(w, r, acct, wid)
}

func (a *api) writeHostCard(w http.ResponseWriter, r *http.Request, acct *app.Account, wid int64) {
	card, err := a.svc.HostCard(r.Context(), acct, wid)
	switch {
	case err != nil:
		writeError(w, err)
	case card == nil:
		w.WriteHeader(http.StatusNoContent)
	default:
		writeJSON(w, http.StatusOK, card)
	}
}

func (a *api) watch(w http.ResponseWriter, r *http.Request, acct *app.Account) {
	wid, err := pathID(r, "wid")
	if err != nil {
		writeError(w, err)
		return
	}
	st, err := a.svc.Watch(r.Context(), acct, wid)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, st)
}

func (a *api) watchVillage(w http.ResponseWriter, r *http.Request, acct *app.Account) {
	wid, err := pathID(r, "wid")
	if err != nil {
		writeError(w, err)
		return
	}
	v, err := a.svc.WatchVillage(r.Context(), acct, wid, r.PathValue("vid"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, v)
}
