// Package memory holds in-memory adapters of the app ports: for fast tests, and the limit counters of a server
// that runs alone.
package memory

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"sort"
	"strings"
	"sync"
	"time"

	"librehold/internal/app"
)

type Accounts struct {
	mu       sync.Mutex
	nextID   int64
	accounts map[int64]*app.Account
	sessions map[string]app.Session
	reports  []app.ProblemReport // in the order they came
	reportID int64
}

func NewAccounts() *Accounts {
	return &Accounts{accounts: map[int64]*app.Account{}, sessions: map[string]app.Session{}}
}

func (s *Accounts) byLower(name string) *app.Account {
	for _, a := range s.accounts {
		if strings.EqualFold(a.Username, name) {
			return a
		}
	}
	return nil
}

func (s *Accounts) insert(a *app.Account, at time.Time) error {
	if s.byLower(a.Username) != nil {
		return app.ErrUsernameTaken
	}
	s.nextID++
	a.ID = s.nextID
	if a.CreatedAt.IsZero() {
		a.CreatedAt = at
	}
	if a.LastSeenAt.IsZero() {
		a.LastSeenAt = a.CreatedAt
	}
	if a.Locale == "" {
		a.Locale = "en"
	}
	a.PlayerKey = newPlayerKey()
	cp := *a
	s.accounts[a.ID] = &cp
	return nil
}

func (s *Accounts) Register(_ context.Context, a *app.Account, at time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.insert(a, at)
}

func (s *Accounts) CreateAdmin(_ context.Context, a *app.Account) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	a.IsAdmin = true
	return s.insert(a, time.Time{})
}

func (s *Accounts) AdminExists(context.Context) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, a := range s.accounts {
		if a.IsAdmin {
			return true, nil
		}
	}
	return false, nil
}

func (s *Accounts) ByUsername(_ context.Context, username string) (*app.Account, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if a := s.byLower(username); a != nil {
		cp := *a
		return &cp, nil
	}
	return nil, app.ErrNotFound
}

func (s *Accounts) ByID(_ context.Context, id int64) (*app.Account, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if a := s.accounts[id]; a != nil {
		cp := *a
		return &cp, nil
	}
	return nil, app.ErrNotFound
}

func (s *Accounts) List(context.Context) ([]app.Account, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]app.Account, 0, len(s.accounts))
	for _, a := range s.accounts {
		out = append(out, *a)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func (s *Accounts) SetPasswordHash(_ context.Context, id int64, hash string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	a := s.accounts[id]
	if a == nil {
		return app.ErrNotFound
	}
	a.PasswordHash = hash
	return nil
}

func (s *Accounts) CreateSession(_ context.Context, sess app.Session) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[string(sess.TokenHash)] = sess
	return nil
}

func (s *Accounts) Session(_ context.Context, tokenHash []byte) (*app.Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if sess, ok := s.sessions[string(tokenHash)]; ok {
		return &sess, nil
	}
	return nil, app.ErrNotFound
}

func (s *Accounts) TouchSession(_ context.Context, tokenHash []byte, lastSeen, expires time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	sess, ok := s.sessions[string(tokenHash)]
	if !ok {
		return app.ErrNotFound
	}
	sess.LastSeenAt, sess.ExpiresAt = lastSeen, expires
	s.sessions[string(tokenHash)] = sess
	return nil
}

func (s *Accounts) DeleteSession(_ context.Context, tokenHash []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, string(tokenHash))
	return nil
}

func (s *Accounts) DeleteSessionsOf(_ context.Context, accountID int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.deleteSessionsOf(accountID)
	return nil
}

func (s *Accounts) deleteSessionsOf(accountID int64) {
	for k, sess := range s.sessions {
		if sess.AccountID == accountID {
			delete(s.sessions, k)
		}
	}
}

func (s *Accounts) UpdateProfile(_ context.Context, id int64, p app.ProfilePatch) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	a := s.accounts[id]
	if a == nil {
		return app.ErrNotFound
	}
	if p.Username != nil {
		if other := s.byLower(*p.Username); other != nil && other.ID != id {
			return app.ErrUsernameTaken
		}
		a.Username = *p.Username
	}
	if p.Locale != nil {
		a.Locale = *p.Locale
	}
	if p.TutorialDone != nil {
		a.TutorialDone = *p.TutorialDone
	}
	if p.PasswordHash != nil {
		a.PasswordHash = *p.PasswordHash
	}
	if p.IsGuest != nil {
		a.IsGuest = *p.IsGuest
	}
	return nil
}

func (s *Accounts) TouchAccount(_ context.Context, id int64, at time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	a := s.accounts[id]
	if a == nil {
		return app.ErrNotFound
	}
	a.LastSeenAt = at
	return nil
}

func (s *Accounts) DeleteAccount(_ context.Context, id int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.accounts[id] == nil {
		return app.ErrNotFound
	}
	delete(s.accounts, id)
	s.deleteSessionsOf(id)
	for i := range s.reports {
		if s.reports[i].AccountID == id {
			s.reports[i].AccountID = 0
		}
	}
	return nil
}

func (s *Accounts) GuestsInactiveSince(_ context.Context, t time.Time, limit int) ([]int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var found []*app.Account
	for _, a := range s.accounts {
		if a.IsGuest && a.LastSeenAt.Before(t) {
			found = append(found, a)
		}
	}
	sort.Slice(found, func(i, j int) bool {
		if !found[i].LastSeenAt.Equal(found[j].LastSeenAt) {
			return found[i].LastSeenAt.Before(found[j].LastSeenAt)
		}
		return found[i].ID < found[j].ID
	})
	ids := make([]int64, 0, min(limit, len(found)))
	for _, a := range found[:min(limit, len(found))] {
		ids = append(ids, a.ID)
	}
	return ids, nil
}

func newPlayerKey() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

var _ app.AccountStore = (*Accounts)(nil)
