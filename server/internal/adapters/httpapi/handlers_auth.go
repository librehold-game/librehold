package httpapi

import (
	"net/http"

	"librehold/internal/app"
)

type accountJSON struct {
	ID           int64  `json:"id"`
	Username     string `json:"username"`
	IsAdmin      bool   `json:"isAdmin"`
	IsGuest      bool   `json:"isGuest"`
	Locale       string `json:"locale"`
	TutorialDone bool   `json:"tutorialDone"`
}

func accountOf(a *app.Account) accountJSON {
	return accountJSON{ID: a.ID, Username: a.Username, IsAdmin: a.IsAdmin, IsGuest: a.IsGuest, Locale: a.Locale,
		TutorialDone: a.TutorialDone}
}

type sessionJSON struct {
	Account accountJSON `json:"account"`
	Token   string      `json:"token"`
}

func (a *api) register(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Username        string `json:"username"`
		Password        string `json:"password"`
		PasswordConfirm string `json:"passwordConfirm"`
		Locale          string `json:"locale"`
	}
	if err := decode(r, &in); err != nil {
		writeError(w, err)
		return
	}
	token, acct, err := a.svc.Register(r.Context(), app.RegisterInput{Username: in.Username, Password: in.Password,
		PasswordConfirm: in.PasswordConfirm, Client: a.clientKey(r), Locale: in.Locale})
	if err != nil {
		writeError(w, err)
		return
	}
	a.setCookie(w, token)
	writeJSON(w, http.StatusCreated, sessionJSON{Account: accountOf(acct), Token: token})
}

func (a *api) login(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := decode(r, &in); err != nil {
		writeError(w, err)
		return
	}
	token, acct, err := a.svc.Login(r.Context(), in.Username, in.Password, a.clientKey(r))
	if err != nil {
		writeError(w, err)
		return
	}
	a.setCookie(w, token)
	writeJSON(w, http.StatusOK, sessionJSON{Account: accountOf(acct), Token: token})
}

func (a *api) guest(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Locale string `json:"locale"`
	}
	if err := decode(r, &in); err != nil {
		writeError(w, err)
		return
	}
	token, acct, err := a.svc.CreateGuest(r.Context(), a.clientKey(r), in.Locale)
	if err != nil {
		writeError(w, err)
		return
	}
	a.setCookie(w, token)
	writeJSON(w, http.StatusCreated, sessionJSON{Account: accountOf(acct), Token: token})
}

func (a *api) claim(w http.ResponseWriter, r *http.Request, acct *app.Account) {
	var in struct {
		Username        string `json:"username"`
		Password        string `json:"password"`
		PasswordConfirm string `json:"passwordConfirm"`
	}
	if err := decode(r, &in); err != nil {
		writeError(w, err)
		return
	}
	claimed, err := a.svc.ClaimGuest(r.Context(), acct, app.RegisterInput{Username: in.Username, Password: in.Password,
		PasswordConfirm: in.PasswordConfirm, Client: a.clientKey(r)})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, accountOf(claimed))
}

func (a *api) logout(w http.ResponseWriter, r *http.Request) {
	info := tokenOf(r)
	if info.token != "" {
		if err := a.svc.Logout(r.Context(), info.token); err != nil {
			writeError(w, err)
			return
		}
	}
	if !info.bearer {
		a.clearCookie(w)
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *api) me(w http.ResponseWriter, _ *http.Request, acct *app.Account) {
	writeJSON(w, http.StatusOK, accountOf(acct))
}

func (a *api) updateMe(w http.ResponseWriter, r *http.Request, acct *app.Account) {
	var in struct {
		Locale       *string `json:"locale"`
		TutorialDone *bool   `json:"tutorialDone"`
		Username     *string `json:"username"`
	}
	if err := decode(r, &in); err != nil {
		writeError(w, err)
		return
	}
	if in.Username != nil {
		renamed, err := a.svc.RenameGuest(r.Context(), acct, *in.Username)
		if err != nil {
			writeError(w, err)
			return
		}
		acct = renamed
	}
	updated, err := a.svc.UpdateMe(r.Context(), acct, app.MePatch{Locale: in.Locale, TutorialDone: in.TutorialDone})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, accountOf(updated))
}
