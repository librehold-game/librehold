package postgres

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"librehold/internal/app"
)

type Accounts struct{ pool *pgxpool.Pool }

func NewAccounts(pool *pgxpool.Pool) *Accounts { return &Accounts{pool: pool} }

func isUniqueViolation(err error) bool {
	var pg *pgconn.PgError
	return errors.As(err, &pg) && pg.Code == "23505"
}

func notFound(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return app.ErrNotFound
	}
	return err
}

func (s *Accounts) Register(ctx context.Context, a *app.Account, at time.Time) error {
	if a.Locale == "" {
		a.Locale = "en"
	}
	err := s.pool.QueryRow(ctx, `insert into accounts (username, username_lower, password_hash, is_admin, created_at,
		locale, is_guest, last_seen_at) values ($1, $2, $3, false, $4, $5, $6, $4) returning id, player_key`,
		a.Username, strings.ToLower(a.Username), a.PasswordHash, at, a.Locale, a.IsGuest).Scan(&a.ID, &a.PlayerKey)
	if isUniqueViolation(err) {
		return app.ErrUsernameTaken
	}
	if err != nil {
		return err
	}
	a.CreatedAt, a.LastSeenAt = at, at
	return nil
}

func (s *Accounts) CreateAdmin(ctx context.Context, a *app.Account) error {
	a.IsAdmin = true
	err := s.pool.QueryRow(ctx, `insert into accounts (username, username_lower, password_hash, is_admin)
		values ($1, $2, $3, true) returning id, created_at, player_key, locale, last_seen_at`, a.Username,
		strings.ToLower(a.Username), a.PasswordHash).Scan(&a.ID, &a.CreatedAt, &a.PlayerKey, &a.Locale, &a.LastSeenAt)
	if isUniqueViolation(err) {
		return app.ErrUsernameTaken
	}
	return err
}

func (s *Accounts) AdminExists(ctx context.Context) (bool, error) {
	var ok bool
	err := s.pool.QueryRow(ctx, `select exists (select from accounts where is_admin)`).Scan(&ok)
	return ok, err
}

const accountCols = `id, username, password_hash, is_admin, created_at, locale, is_guest, player_key, tutorial_done,
	last_seen_at`

func scanAccount(row pgx.Row) (*app.Account, error) {
	var a app.Account
	if err := row.Scan(&a.ID, &a.Username, &a.PasswordHash, &a.IsAdmin, &a.CreatedAt, &a.Locale, &a.IsGuest, &a.PlayerKey,
		&a.TutorialDone, &a.LastSeenAt); err != nil {
		return nil, notFound(err)
	}
	return &a, nil
}

func (s *Accounts) ByUsername(ctx context.Context, username string) (*app.Account, error) {
	return scanAccount(s.pool.QueryRow(ctx, `select `+accountCols+` from accounts where username_lower = $1`, strings.ToLower(username)))
}

func (s *Accounts) ByID(ctx context.Context, id int64) (*app.Account, error) {
	return scanAccount(s.pool.QueryRow(ctx, `select `+accountCols+` from accounts where id = $1`, id))
}

func (s *Accounts) List(ctx context.Context) ([]app.Account, error) {
	rows, err := s.pool.Query(ctx, `select `+accountCols+` from accounts order by id`)
	if err != nil {
		return nil, err
	}
	var out []app.Account
	for rows.Next() {
		a, err := scanAccount(rows)
		if err != nil {
			rows.Close()
			return nil, err
		}
		out = append(out, *a)
	}
	return out, rows.Err()
}

func affected(tag pgconn.CommandTag, err error) error {
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return app.ErrNotFound
	}
	return nil
}

func (s *Accounts) SetPasswordHash(ctx context.Context, id int64, hash string) error {
	return affected(s.pool.Exec(ctx, `update accounts set password_hash = $1 where id = $2`, hash, id))
}

func (s *Accounts) CreateSession(ctx context.Context, sess app.Session) error {
	_, err := s.pool.Exec(ctx, `insert into sessions (token_hash, account_id, expires_at, last_seen_at) values ($1, $2, $3, $4)`,
		sess.TokenHash, sess.AccountID, sess.ExpiresAt, sess.LastSeenAt)
	return err
}

func (s *Accounts) Session(ctx context.Context, tokenHash []byte) (*app.Session, error) {
	var sess app.Session
	err := s.pool.QueryRow(ctx, `select token_hash, account_id, expires_at, last_seen_at from sessions where token_hash = $1`, tokenHash).
		Scan(&sess.TokenHash, &sess.AccountID, &sess.ExpiresAt, &sess.LastSeenAt)
	if err != nil {
		return nil, notFound(err)
	}
	return &sess, nil
}

func (s *Accounts) TouchSession(ctx context.Context, tokenHash []byte, lastSeen, expires time.Time) error {
	return affected(s.pool.Exec(ctx, `update sessions set last_seen_at = $1, expires_at = $2 where token_hash = $3`, lastSeen, expires, tokenHash))
}

func (s *Accounts) DeleteSession(ctx context.Context, tokenHash []byte) error {
	_, err := s.pool.Exec(ctx, `delete from sessions where token_hash = $1`, tokenHash)
	return err
}

func (s *Accounts) DeleteSessionsOf(ctx context.Context, accountID int64) error {
	_, err := s.pool.Exec(ctx, `delete from sessions where account_id = $1`, accountID)
	return err
}

func (s *Accounts) UpdateProfile(ctx context.Context, id int64, p app.ProfilePatch) error {
	var lower *string
	if p.Username != nil {
		l := strings.ToLower(*p.Username)
		lower = &l
	}
	err := affected(s.pool.Exec(ctx, `update accounts set
			username = coalesce($2, username), username_lower = coalesce($3, username_lower),
			locale = coalesce($4, locale), tutorial_done = coalesce($5, tutorial_done),
			password_hash = coalesce($6, password_hash), is_guest = coalesce($7, is_guest)
		where id = $1`, id, p.Username, lower, p.Locale, p.TutorialDone, p.PasswordHash, p.IsGuest))
	if isUniqueViolation(err) {
		return app.ErrUsernameTaken
	}
	return err
}

func (s *Accounts) TouchAccount(ctx context.Context, id int64, at time.Time) error {
	return affected(s.pool.Exec(ctx, `update accounts set last_seen_at = $2 where id = $1`, id, at))
}

func (s *Accounts) DeleteAccount(ctx context.Context, id int64) error {
	return affected(s.pool.Exec(ctx, `delete from accounts where id = $1`, id))
}

func (s *Accounts) GuestsInactiveSince(ctx context.Context, t time.Time, limit int) ([]int64, error) {
	rows, err := s.pool.Query(ctx, `select id from accounts where is_guest and last_seen_at < $1
		order by last_seen_at, id limit $2`, t, limit)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowTo[int64])
}

var _ app.AccountStore = (*Accounts)(nil)
