package httpapi

import (
	"errors"
	"net/http/httptest"
	"strings"
	"testing"

	"librehold/internal/app"
	"librehold/internal/game"
)

func TestRuleErrorsCarryNoProse(t *testing.T) {
	rec := httptest.NewRecorder()
	writeError(rec, game.Rule("tooLate", map[string]any{"ms": 65000}))
	body := rec.Body.String()
	if rec.Code != 409 || !strings.Contains(body, `"code":"tooLate"`) || !strings.Contains(body, `"params":{"ms":65000}`) ||
		strings.Contains(body, `"message"`) {
		t.Fatalf("%d %s", rec.Code, body)
	}
	rec = httptest.NewRecorder()
	writeError(rec, &app.ErrValidation{Field: "username", Code: "usernameTaken"})
	if body := rec.Body.String(); strings.Contains(body, `"message"`) || !strings.Contains(body, `"field":"username"`) {
		t.Fatalf("validation %s", body)
	}
}

func TestUnknownErrorsAreWithheld(t *testing.T) {
	rec := httptest.NewRecorder()
	writeError(rec, errors.New("database connection lost: password=secret"))
	if rec.Code != 500 || !strings.Contains(rec.Body.String(), `"code":"internal"`) || strings.Contains(rec.Body.String(), "secret") {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
}
