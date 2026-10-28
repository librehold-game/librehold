package memory

import (
	"context"
	"time"

	"librehold/internal/app"
)

func (s *Accounts) AddProblemReport(_ context.Context, r *app.ProblemReport) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.reportID++
	r.ID, r.Status, r.ClosedAt = s.reportID, "new", nil
	stored := *r
	stored.Username = ""
	s.reports = append(s.reports, stored)
	return nil
}

func (s *Accounts) ProblemReports(_ context.Context, limit int) ([]app.ProblemReport, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := []app.ProblemReport{}
	for i := len(s.reports) - 1; i >= 0 && len(out) < limit; i-- {
		r := s.reports[i]
		if a := s.accounts[r.AccountID]; a != nil {
			r.Username = a.Username
		}
		out = append(out, r)
	}
	return out, nil
}

func (s *Accounts) SetProblemReportStatus(_ context.Context, id int64, status string, at time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.reports {
		if r := &s.reports[i]; r.ID == id {
			r.Status, r.ClosedAt = status, nil
			if status == "done" {
				r.ClosedAt = &at
			}
			return nil
		}
	}
	return app.ErrNotFound
}

func (s *Accounts) DeleteClosedProblemReports(_ context.Context, closedBefore time.Time) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	kept := s.reports[:0]
	for _, r := range s.reports {
		if r.Status != "done" || r.ClosedAt == nil || !r.ClosedAt.Before(closedBefore) {
			kept = append(kept, r)
		}
	}
	n := len(s.reports) - len(kept)
	s.reports = kept
	return n, nil
}
