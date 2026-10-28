package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

var (
	ErrBadCredentials       = errors.New("bad credentials")
	ErrTooManyAttempts      = errors.New("too many login attempts")
	ErrTooManyRegistrations = errors.New("too many registrations from this client")
	ErrUnauthenticated      = errors.New("unauthenticated")
)

var (
	loginLimit       = limit{"login", 5, 15 * time.Minute, false}
	loginClientLimit = limit{"loginClient", 20, 15 * time.Minute, true}
)

const (
	registrationsLimit = 3
	registrationWindow = time.Hour
	sessionRenew       = time.Hour
	minPassword        = 8
)

var usernamePattern = regexp.MustCompile(`^[A-Za-z0-9_-]{3,24}$`)

// RegisterInput is an open registration; Client is the caller's address for the registration limit.
type RegisterInput struct {
	Username, Password, PasswordConfirm, Client, Locale string
}

// MePatch is what a player may change about their own account.
type MePatch struct {
	Locale       *string
	TutorialDone *bool
}

func normalLocale(l string) string {
	if l == "pl" {
		return "pl"
	}
	return "en"
}

func (s *Service) wall() time.Time { return time.UnixMilli(s.clock.NowMs()).UTC() }

func checkPassword(pw, confirm string) error {
	if utf8.RuneCountInString(pw) < minPassword {
		return &ErrValidation{Field: "password", Code: "passwordShort"}
	}
	if pw != confirm {
		return &ErrValidation{Field: "passwordConfirm", Code: "passwordMismatch"}
	}
	return nil
}

// Register opens an account for anyone, at most Config.RegistrationsPerHour per client address within an hour, so a
// script cannot fill the worlds with accounts; refused ones do not count.
func (s *Service) Register(ctx context.Context, in RegisterInput) (string, *Account, error) {
	if !usernamePattern.MatchString(in.Username) {
		return "", nil, &ErrValidation{Field: "username", Code: "usernameFormat"}
	}
	if err := checkPassword(in.Password, in.PasswordConfirm); err != nil {
		return "", nil, err
	}
	if err := s.usernameFree(ctx, in.Username); err != nil {
		return "", nil, err
	}
	switch ok, err := s.take(ctx, s.registrations, in.Client); {
	case err != nil:
		return "", nil, err
	case !ok:
		return "", nil, ErrTooManyRegistrations
	}
	acct, err := s.register(ctx, in)
	if err != nil {
		return "", nil, errors.Join(err, s.giveBack(ctx, s.registrations, in.Client))
	}
	token, err := s.newSession(ctx, acct.ID)
	if err != nil {
		return "", nil, err
	}
	return token, acct, nil
}

func (s *Service) register(ctx context.Context, in RegisterInput) (*Account, error) {
	hash, err := s.hasher.Hash(ctx, in.Password)
	if err != nil {
		return nil, err
	}
	acct := &Account{Username: in.Username, PasswordHash: hash, Locale: normalLocale(in.Locale)}
	switch err := s.accounts.Register(ctx, acct, s.wall()); {
	case errors.Is(err, ErrUsernameTaken):
		return nil, &ErrValidation{Field: "username", Code: "usernameTaken"}
	case err != nil:
		return nil, err
	}
	return acct, nil
}

// usernameFree refuses a taken name before anything counts against the caller's limits.
func (s *Service) usernameFree(ctx context.Context, name string) error {
	switch _, err := s.accounts.ByUsername(ctx, name); {
	case err == nil:
		return &ErrValidation{Field: "username", Code: "usernameTaken"}
	case errors.Is(err, ErrNotFound):
		return nil
	default:
		return err
	}
}

func (s *Service) newSession(ctx context.Context, accountID int64) (string, error) {
	plain, hash := s.tokens.New()
	now := s.wall()
	err := s.accounts.CreateSession(ctx, Session{TokenHash: hash, AccountID: accountID, ExpiresAt: now.Add(s.cfg.SessionTTL), LastSeenAt: now})
	return plain, err
}

// LocalClient is the client key of a caller on the server's own network, as at a LAN party where everyone may
// share one address: the limits per address leave it out.
const LocalClient = "local"

// limit is how many events of one kind a key may have within window; the counts live in Counters, shared by every
// server process. Keys are stored as their SHA-256, which keeps them out of plain sight but can be reversed by
// trying every address; they expire with their window. perClient limits key by the client address alone.
type limit struct {
	kind      string
	max       int
	window    time.Duration
	perClient bool
}

func limitKey(l limit, key string) string {
	sum := sha256.Sum256([]byte(key))
	return l.kind + ":" + hex.EncodeToString(sum[:])
}

// take counts an attempt before any of its work, so attempts sent at once cannot all pass a check none of them
// has counted yet; false when the attempt is over the limit.
func (s *Service) take(ctx context.Context, l limit, key string) (bool, error) {
	if l.perClient && key == LocalClient {
		return true, nil
	}
	n, err := s.counters.Add(ctx, limitKey(l, key), l.window)
	return err == nil && n <= l.max, err
}

// giveBack uncounts an attempt that take let through and that ended up refused for another reason.
func (s *Service) giveBack(ctx context.Context, l limit, key string) error {
	if l.perClient && key == LocalClient {
		return nil
	}
	return s.counters.Undo(ctx, limitKey(l, key))
}

// Login checks the password and opens a session. Every refusal looks the same, ErrBadCredentials, until 5 failures
// for the same username and client, or 20 from the client for any usernames, within 15 minutes; then even the right
// password gets ErrTooManyAttempts. A success clears the username's failures and does not count for the client.
func (s *Service) Login(ctx context.Context, username, password, clientKey string) (string, *Account, error) {
	user := strings.ToLower(username) + "\x00" + clientKey
	for _, c := range []struct {
		l   limit
		key string
	}{{loginClientLimit, clientKey}, {loginLimit, user}} {
		switch ok, err := s.take(ctx, c.l, c.key); {
		case err != nil:
			return "", nil, err
		case !ok:
			return "", nil, ErrTooManyAttempts
		}
	}
	acct, err := s.accounts.ByUsername(ctx, username)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return "", nil, err
	}
	known := acct != nil && acct.PasswordHash != ""
	var hash string
	if known {
		hash = acct.PasswordHash
	} else {
		hash = s.dummyHash()
	}
	switch ok, err := s.hasher.Verify(ctx, hash, password); {
	case err != nil:
		return "", nil, err
	case !ok || !known:
		return "", nil, ErrBadCredentials
	}
	if err := s.counters.Clear(ctx, limitKey(loginLimit, user)); err != nil {
		return "", nil, err
	}
	if err := s.giveBack(ctx, loginClientLimit, clientKey); err != nil {
		return "", nil, err
	}
	token, err := s.newSession(ctx, acct.ID)
	if err != nil {
		return "", nil, err
	}
	return token, acct, nil
}

// dummyHash makes a login for an unknown user cost as much as a wrong password.
func (s *Service) dummyHash() string {
	s.dummyOnce.Do(func() { s.dummy, _ = s.hasher.Hash(context.Background(), "no such account") })
	return s.dummy
}

// Authenticate resolves a session token to its account; the session's expiry slides, renewed at most once
// an hour.
func (s *Service) Authenticate(ctx context.Context, token string) (*Account, error) {
	hash := s.tokens.Hash(token)
	sess, err := s.accounts.Session(ctx, hash)
	if errors.Is(err, ErrNotFound) {
		return nil, ErrUnauthenticated
	}
	if err != nil {
		return nil, err
	}
	now := s.wall()
	if !now.Before(sess.ExpiresAt) {
		_ = s.accounts.DeleteSession(ctx, hash)
		return nil, ErrUnauthenticated
	}
	if now.Sub(sess.LastSeenAt) >= sessionRenew {
		if err := s.accounts.TouchSession(ctx, hash, now, now.Add(s.cfg.SessionTTL)); err != nil {
			return nil, err
		}
		if err := s.accounts.TouchAccount(ctx, sess.AccountID, now); err != nil && !errors.Is(err, ErrNotFound) {
			return nil, err
		}
	}
	acct, err := s.accounts.ByID(ctx, sess.AccountID)
	if errors.Is(err, ErrNotFound) {
		return nil, ErrUnauthenticated
	}
	return acct, err
}

// UpdateMe changes the caller's language or tutorial state and returns the account as stored.
func (s *Service) UpdateMe(ctx context.Context, acct *Account, in MePatch) (*Account, error) {
	if in.Locale != nil && *in.Locale != "en" && *in.Locale != "pl" {
		return nil, &ErrValidation{Field: "locale", Code: "locale"}
	}
	if err := s.accounts.UpdateProfile(ctx, acct.ID, ProfilePatch{Locale: in.Locale, TutorialDone: in.TutorialDone}); err != nil {
		return nil, err
	}
	return s.accounts.ByID(ctx, acct.ID)
}

func (s *Service) Logout(ctx context.Context, token string) error {
	return s.accounts.DeleteSession(ctx, s.tokens.Hash(token))
}

// BootstrapAdmin creates the configured admin when there is no admin yet.
func (s *Service) BootstrapAdmin(ctx context.Context) error {
	ok, err := s.accounts.AdminExists(ctx)
	if err != nil || ok {
		return err
	}
	if s.cfg.AdminUsername == "" {
		return errors.New("no admin account: set the admin username and password")
	}
	if !usernamePattern.MatchString(s.cfg.AdminUsername) || utf8.RuneCountInString(s.cfg.AdminPassword) < minPassword {
		return errors.New("admin: the username must be 3–24 of A–Z a–z 0–9 _ - and the password at least 8 characters")
	}
	hash, err := s.hasher.Hash(ctx, s.cfg.AdminPassword)
	if err != nil {
		return err
	}
	return s.accounts.CreateAdmin(ctx, &Account{Username: s.cfg.AdminUsername, PasswordHash: hash, IsAdmin: true})
}
