package memory

import (
	"context"

	"librehold/internal/app"
)

// cloneRow is a copy of a world row that shares no host with it, so a unit of work changes only its own.
func cloneRow(r app.WorldRow) app.WorldRow {
	r.Host = r.Host.Clone()
	return r
}

func (s *Worlds) HostSettings(context.Context) (app.HostSettings, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.hostSettings == nil {
		return app.DefaultHostSettings, nil
	}
	return *s.hostSettings, nil
}

func (s *Worlds) SaveHostSettings(_ context.Context, h app.HostSettings) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.hostSettings = &h
	return nil
}

func (s *Worlds) WorldByInvite(_ context.Context, code string) (*app.WorldRow, error) {
	return s.worldWhere(func(h *app.HostedWorld) bool { return code != "" && h.InviteCode == code })
}

func (s *Worlds) WorldByResultsCode(_ context.Context, code string) (*app.WorldRow, error) {
	return s.worldWhere(func(h *app.HostedWorld) bool { return code != "" && h.ResultsCode == code })
}

func (s *Worlds) worldWhere(match func(h *app.HostedWorld) bool) (*app.WorldRow, error) {
	for _, w := range s.all() {
		w.mu.Lock()
		row := cloneRow(w.row)
		w.mu.Unlock()
		if row.Host != nil && match(row.Host) {
			return &row, nil
		}
	}
	return nil, app.ErrNotFound
}
