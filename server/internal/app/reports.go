package app

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

var reportLimit = limit{"report", reportsPerHour, time.Hour, true}

const (
	reportsPerHour      = 5
	reportTextMin       = 10
	reportTextMax       = 2000
	reportEmailMax      = 254
	reportKeep          = 365 * 24 * time.Hour
	problemReportsShown = 200
)

// ProblemReport is a message to the admin: a bug, a player to look at, a question.
type ProblemReport struct {
	ID        int64      `json:"id"`
	CreatedAt time.Time  `json:"createdAt"`
	Kind      string     `json:"kind"` // bug | player | question | other
	Text      string     `json:"text"`
	Email     string     `json:"email"`
	AccountID int64      `json:"accountId,omitempty"`
	Username  string     `json:"username"` // filled on list; "" = a visitor or a deleted account
	WorldID   int64      `json:"worldId,omitempty"`
	WorldName string     `json:"worldName"`
	Screen    string     `json:"screen"`
	Build     string     `json:"build"`
	UserAgent string     `json:"userAgent"`
	Status    string     `json:"status"` // new | read | done
	ClosedAt  *time.Time `json:"closedAt,omitempty"`
}

type ProblemReportStore interface {
	AddProblemReport(ctx context.Context, r *ProblemReport) error           // sets ID; Status "new"
	ProblemReports(ctx context.Context, limit int) ([]ProblemReport, error) // newest first
	// SetProblemReportStatus: "done" sets ClosedAt = at, any other status clears it; ErrNotFound for a missing id.
	SetProblemReportStatus(ctx context.Context, id int64, status string, at time.Time) error
	DeleteClosedProblemReports(ctx context.Context, closedBefore time.Time) (int, error)
}

type ReportInput struct {
	Kind, Text, Email, Screen, Build, UserAgent, Client string
	WorldID                                             int64
}

var ErrTooManyReports = errors.New("too many reports from this client")

var (
	reportKinds    = map[string]bool{"bug": true, "player": true, "question": true, "other": true}
	reportStatuses = map[string]bool{"new": true, "read": true, "done": true}
	emailPattern   = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)
)

// cut trims s and keeps at most n characters of it.
func cut(s string, n int) string {
	s = strings.TrimSpace(s)
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	return string([]rune(s)[:n])
}

// SubmitReport stores a report from anyone, acct nil for a visitor: at most reportsPerHour stored ones per client
// address within an hour; refused ones do not count.
func (s *Service) SubmitReport(ctx context.Context, acct *Account, in ReportInput) (int64, error) {
	now := s.wall()
	if !reportKinds[in.Kind] {
		return 0, &ErrValidation{Field: "kind", Code: "reportKind"}
	}
	text := strings.TrimSpace(in.Text)
	if n := utf8.RuneCountInString(text); n < reportTextMin || n > reportTextMax {
		return 0, &ErrValidation{Field: "text", Code: "reportText"}
	}
	email := strings.TrimSpace(in.Email)
	if email != "" && (utf8.RuneCountInString(email) > reportEmailMax || !emailPattern.MatchString(email)) {
		return 0, &ErrValidation{Field: "email", Code: "reportEmail"}
	}
	r := &ProblemReport{CreatedAt: now, Kind: in.Kind, Text: text, Email: email, Screen: cut(in.Screen, 40),
		Build: cut(in.Build, 64), UserAgent: cut(in.UserAgent, 400)}
	if acct != nil {
		r.AccountID = acct.ID
	}
	if in.WorldID > 0 {
		r.WorldID = in.WorldID
		if row, err := s.worlds.Get(ctx, in.WorldID); err == nil {
			r.WorldName = row.Name
		}
	}
	switch ok, err := s.take(ctx, reportLimit, in.Client); {
	case err != nil:
		return 0, err
	case !ok:
		return 0, ErrTooManyReports
	}
	if err := s.accounts.AddProblemReport(ctx, r); err != nil {
		return 0, errors.Join(err, s.giveBack(ctx, reportLimit, in.Client))
	}
	return r.ID, nil
}

// ProblemReports are the latest problemReportsShown reports, newest first; for the admin.
func (s *Service) ProblemReports(ctx context.Context, actor *Account) ([]ProblemReport, error) {
	if err := requireAdmin(actor); err != nil {
		return nil, err
	}
	list, err := s.accounts.ProblemReports(ctx, problemReportsShown)
	if list == nil && err == nil {
		list = []ProblemReport{}
	}
	return list, err
}

func (s *Service) SetProblemReportStatus(ctx context.Context, actor *Account, id int64, status string) error {
	if err := requireAdmin(actor); err != nil {
		return err
	}
	if !reportStatuses[status] {
		return &ErrValidation{Field: "status", Code: "statusInvalid"}
	}
	return s.accounts.SetProblemReportStatus(ctx, id, status, s.wall())
}

// PurgeClosedReports deletes the reports closed more than reportKeep ago.
func (s *Service) PurgeClosedReports(ctx context.Context) (int, error) {
	return s.accounts.DeleteClosedProblemReports(ctx, s.wall().Add(-reportKeep))
}
