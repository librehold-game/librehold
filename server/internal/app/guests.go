package app

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"time"

	"librehold/internal/game"
)

var ErrTooManyGuests = errors.New("too many guests from this client")

const (
	guestsLimit      = 10
	guestWindow      = time.Hour
	guestInactivity  = 30 * 24 * time.Hour
	guestNameRetries = 20
	purgeBatch       = 200
)

func randomSeed() uint32 {
	var b [4]byte
	_, _ = rand.Read(b[:])
	return binary.LittleEndian.Uint32(b[:])
}

// CreateGuest opens a password-less account with a random free nickname and a session, at most
// Config.GuestsPerHour per client address within an hour.
func (s *Service) CreateGuest(ctx context.Context, client, locale string) (string, *Account, error) {
	switch ok, err := s.take(ctx, s.guests, client); {
	case err != nil:
		return "", nil, err
	case !ok:
		return "", nil, ErrTooManyGuests
	}
	acct, err := s.newGuest(ctx, locale)
	if err != nil {
		return "", nil, errors.Join(err, s.giveBack(ctx, s.guests, client))
	}
	token, err := s.newSession(ctx, acct.ID)
	if err != nil {
		return "", nil, err
	}
	return token, acct, nil
}

func (s *Service) newGuest(ctx context.Context, locale string) (*Account, error) {
	now := s.wall()
	seed := randomSeed()
	var acct *Account
	for n := range guestNameRetries {
		a := &Account{Username: game.Nickname(seed, n), IsGuest: true, Locale: normalLocale(locale)}
		err := s.accounts.Register(ctx, a, now)
		if errors.Is(err, ErrUsernameTaken) {
			continue
		}
		if err != nil {
			return nil, err
		}
		acct = a
		break
	}
	if acct == nil {
		return nil, ErrUsernameTaken
	}
	return acct, nil
}

// ClaimGuest turns a guest into a player: a username and a password on the same account, so its session, games
// and tally carry on. It counts towards the registration limit.
func (s *Service) ClaimGuest(ctx context.Context, acct *Account, in RegisterInput) (*Account, error) {
	if !acct.IsGuest {
		return nil, ErrForbidden
	}
	if !usernamePattern.MatchString(in.Username) {
		return nil, &ErrValidation{Field: "username", Code: "usernameFormat"}
	}
	if err := checkPassword(in.Password, in.PasswordConfirm); err != nil {
		return nil, err
	}
	if err := s.usernameFree(ctx, in.Username); err != nil {
		return nil, err
	}
	switch ok, err := s.take(ctx, s.registrations, in.Client); {
	case err != nil:
		return nil, err
	case !ok:
		return nil, ErrTooManyRegistrations
	}
	if err := s.claim(ctx, acct, in); err != nil {
		return nil, errors.Join(err, s.giveBack(ctx, s.registrations, in.Client))
	}
	if err := s.renameInWorlds(ctx, acct.ID, in.Username, true); err != nil {
		return nil, err
	}
	return s.accounts.ByID(ctx, acct.ID)
}

func (s *Service) claim(ctx context.Context, acct *Account, in RegisterInput) error {
	hash, err := s.hasher.Hash(ctx, in.Password)
	if err != nil {
		return err
	}
	no := false
	switch err := s.accounts.UpdateProfile(ctx, acct.ID, ProfilePatch{Username: &in.Username, PasswordHash: &hash, IsGuest: &no}); {
	case errors.Is(err, ErrUsernameTaken):
		return &ErrValidation{Field: "username", Code: "usernameTaken"}
	default:
		return err
	}
}

// RenameGuest gives a guest another nickname, in its account and the games it plays.
func (s *Service) RenameGuest(ctx context.Context, acct *Account, username string) (*Account, error) {
	if !acct.IsGuest {
		return nil, ErrForbidden
	}
	if !usernamePattern.MatchString(username) {
		return nil, &ErrValidation{Field: "username", Code: "usernameFormat"}
	}
	switch err := s.accounts.UpdateProfile(ctx, acct.ID, ProfilePatch{Username: &username}); {
	case errors.Is(err, ErrUsernameTaken):
		return nil, &ErrValidation{Field: "username", Code: "usernameTaken"}
	case err != nil:
		return nil, err
	}
	if err := s.renameInWorlds(ctx, acct.ID, username, false); err != nil {
		return nil, err
	}
	return s.accounts.ByID(ctx, acct.ID)
}

// renameInWorlds renames the account's player in every world it plays in; guests play only instant worlds. A
// claimed guest stops being a guest there too, which the action log records.
func (s *Service) renameInWorlds(ctx context.Context, accountID int64, name string, claimed bool) error {
	members, err := s.worlds.MembershipsOf(ctx, accountID)
	if err != nil {
		return err
	}
	for _, m := range members {
		err := s.worlds.Run(ctx, m.WorldID, func(w WorldTx) error {
			if p := w.Player(m.PlayerID); p != nil {
				p.Name = name
				if claimed && p.IsGuest {
					p.IsGuest = false
					w.SetNow(WorldNow(w.Row(), s.clock.NowMs()))
					w.LogAction(game.ActorOf(p), false, "claim", struct{}{})
				}
			}
			return nil
		})
		if err != nil {
			return err
		}
	}
	return nil
}

// PurgeGuests deletes guests not seen for 30 days that no longer play in any world.
func (s *Service) PurgeGuests(ctx context.Context) (int, error) {
	ids, err := s.accounts.GuestsInactiveSince(ctx, s.wall().Add(-guestInactivity), purgeBatch)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, id := range ids {
		members, err := s.worlds.MembershipsOf(ctx, id)
		if err != nil {
			return n, err
		}
		if len(members) > 0 {
			continue
		}
		acct, err := s.accounts.ByID(ctx, id)
		if errors.Is(err, ErrNotFound) {
			continue
		}
		if err != nil {
			return n, err
		}
		if err := s.worlds.ForgetAccount(ctx, id, acct.PlayerKey); err != nil {
			return n, err
		}
		if err := s.accounts.DeleteAccount(ctx, id); err != nil && !errors.Is(err, ErrNotFound) {
			return n, err
		}
		n++
	}
	return n, nil
}
