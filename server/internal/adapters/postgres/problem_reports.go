package postgres

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"

	"librehold/internal/app"
)

func (s *Accounts) AddProblemReport(ctx context.Context, r *app.ProblemReport) error {
	var account, world *int64
	if r.AccountID != 0 {
		account = &r.AccountID
	}
	if r.WorldID != 0 {
		world = &r.WorldID
	}
	err := s.pool.QueryRow(ctx, `insert into problem_reports (created_at, kind, text, email, account_id, world_id, world_name,
			screen, build, user_agent) values ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10) returning id`,
		r.CreatedAt, r.Kind, r.Text, r.Email, account, world, r.WorldName, r.Screen, r.Build, r.UserAgent).Scan(&r.ID)
	if err != nil {
		return err
	}
	r.Status, r.ClosedAt = "new", nil
	return nil
}

func (s *Accounts) ProblemReports(ctx context.Context, limit int) ([]app.ProblemReport, error) {
	rows, err := s.pool.Query(ctx, `select r.id, r.created_at, r.kind, r.text, r.email, coalesce(r.account_id, 0),
			coalesce(a.username, ''), coalesce(r.world_id, 0), r.world_name, r.screen, r.build, r.user_agent, r.status, r.closed_at
		from problem_reports r left join accounts a on a.id = r.account_id
		order by r.created_at desc, r.id desc limit $1`, limit)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (app.ProblemReport, error) {
		var r app.ProblemReport
		err := row.Scan(&r.ID, &r.CreatedAt, &r.Kind, &r.Text, &r.Email, &r.AccountID, &r.Username, &r.WorldID, &r.WorldName,
			&r.Screen, &r.Build, &r.UserAgent, &r.Status, &r.ClosedAt)
		r.CreatedAt = r.CreatedAt.UTC()
		if r.ClosedAt != nil {
			at := r.ClosedAt.UTC()
			r.ClosedAt = &at
		}
		return r, err
	})
}

func (s *Accounts) SetProblemReportStatus(ctx context.Context, id int64, status string, at time.Time) error {
	var closed *time.Time
	if status == "done" {
		closed = &at
	}
	return affected(s.pool.Exec(ctx, `update problem_reports set status = $2, closed_at = $3 where id = $1`, id, status, closed))
}

func (s *Accounts) DeleteClosedProblemReports(ctx context.Context, closedBefore time.Time) (int, error) {
	tag, err := s.pool.Exec(ctx, `delete from problem_reports where status = 'done' and closed_at < $1`, closedBefore)
	if err != nil {
		return 0, err
	}
	return int(tag.RowsAffected()), nil
}
