package httpapi

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"librehold/internal/app"
	"librehold/internal/game"
)

// apiError carries codes and values only; the client words them in the player's language.
type apiError struct {
	Code     string         `json:"code"`
	Params   map[string]any `json:"params,omitempty"`
	Blockers []game.Blocker `json:"blockers,omitempty"`
	Field    string         `json:"field,omitempty"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeAPIError(w http.ResponseWriter, status int, e apiError) {
	writeJSON(w, status, map[string]apiError{"error": e})
}

// writeError maps an error of the service to its status and body; anything unexpected is logged and
// answered without details.
func writeError(w http.ResponseWriter, err error) {
	var blocked *game.BlockedError
	var rule *game.RuleError
	var invalid *app.ErrValidation
	switch {
	case errors.As(err, &blocked):
		writeAPIError(w, http.StatusConflict, apiError{Code: "blocked", Blockers: blocked.Blockers})
	case errors.As(err, &rule):
		writeAPIError(w, http.StatusConflict, apiError{Code: rule.Code, Params: rule.Params})
	case errors.As(err, &invalid):
		writeAPIError(w, http.StatusBadRequest, apiError{Code: invalid.Code, Field: invalid.Field})
	case errors.Is(err, app.ErrBadCredentials):
		writeAPIError(w, http.StatusUnauthorized, apiError{Code: "badCredentials"})
	case errors.Is(err, app.ErrUnauthenticated):
		writeAPIError(w, http.StatusUnauthorized, apiError{Code: "unauthenticated"})
	case errors.Is(err, app.ErrForbidden):
		writeAPIError(w, http.StatusForbidden, apiError{Code: "forbidden"})
	case errors.Is(err, app.ErrNotMember):
		writeAPIError(w, http.StatusForbidden, apiError{Code: "notMember"})
	case errors.Is(err, app.ErrTooManyGuests):
		writeAPIError(w, http.StatusTooManyRequests, apiError{Code: "tooManyGuests"})
	case errors.Is(err, app.ErrTooManyRegistrations):
		writeAPIError(w, http.StatusTooManyRequests, apiError{Code: "tooManyRegistrations"})
	case errors.Is(err, app.ErrTooManyReports):
		writeAPIError(w, http.StatusTooManyRequests, apiError{Code: "tooManyReports"})
	case errors.Is(err, app.ErrTooFast):
		writeAPIError(w, http.StatusTooManyRequests, apiError{Code: "tooFast"})
	case errors.Is(err, app.ErrTooManyAttempts):
		writeAPIError(w, http.StatusTooManyRequests, apiError{Code: "tooManyAttempts"})
	case errors.Is(err, app.ErrWorldEnded):
		writeAPIError(w, http.StatusGone, apiError{Code: "worldEnded"})
	case errors.Is(err, app.ErrWorldPaused):
		writeAPIError(w, http.StatusLocked, apiError{Code: "worldPaused"})
	case errors.Is(err, app.ErrServerFull):
		writeAPIError(w, http.StatusServiceUnavailable, apiError{Code: "serverFull"})
	case errors.Is(err, app.ErrNotFound):
		writeAPIError(w, http.StatusNotFound, apiError{Code: "notFound"})
	default:
		slog.Error("api", "err", err)
		writeAPIError(w, http.StatusInternalServerError, apiError{Code: "internal"})
	}
}
