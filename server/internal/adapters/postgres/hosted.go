package postgres

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"

	"librehold/internal/app"
)

// hostArgs are a row's host columns: host_account_id, invite_code, results_code and host; all null without a host.
func hostArgs(h *app.HostedWorld) []any {
	if h == nil {
		return []any{nil, nil, nil, nil}
	}
	return []any{accountArg(h.HostID), nullIfEmpty(h.InviteCode), nullIfEmpty(h.ResultsCode), marshal(h)}
}

func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// scanHost builds a row's host from its columns; nil when the host column is null.
func scanHost(hostID *int64, invite, results *string, data []byte) (*app.HostedWorld, error) {
	if data == nil {
		return nil, nil
	}
	h := &app.HostedWorld{}
	if err := json.Unmarshal(data, h); err != nil {
		return nil, err
	}
	if hostID != nil {
		h.HostID = *hostID
	}
	if invite != nil {
		h.InviteCode = *invite
	}
	if results != nil {
		h.ResultsCode = *results
	}
	return h, nil
}

// hostKeys are the host's columns that its JSON leaves out, so a change to them is still written.
func hostKeys(h *app.HostedWorld) []any {
	if h == nil {
		return nil
	}
	return []any{h.HostID, h.InviteCode, h.ResultsCode}
}

func (s *Worlds) HostSettings(ctx context.Context) (app.HostSettings, error) {
	h := app.DefaultHostSettings
	err := s.pool.QueryRow(ctx, `select max_seats, max_bots from host_settings where id = 1`).Scan(&h.MaxSeats, &h.MaxBots)
	if errors.Is(err, pgx.ErrNoRows) {
		return app.DefaultHostSettings, nil
	}
	return h, err
}

func (s *Worlds) SaveHostSettings(ctx context.Context, h app.HostSettings) error {
	_, err := s.pool.Exec(ctx, `insert into host_settings (id, max_seats, max_bots) values (1, $1, $2)
		on conflict (id) do update set max_seats = excluded.max_seats, max_bots = excluded.max_bots`, h.MaxSeats, h.MaxBots)
	return err
}

func (s *Worlds) WorldByInvite(ctx context.Context, code string) (*app.WorldRow, error) {
	return scanWorldRow(s.pool.QueryRow(ctx, `select `+worldCols+` from worlds where invite_code = $1`, code))
}

func (s *Worlds) WorldByResultsCode(ctx context.Context, code string) (*app.WorldRow, error) {
	return scanWorldRow(s.pool.QueryRow(ctx, `select `+worldCols+` from worlds where results_code = $1`, code))
}
