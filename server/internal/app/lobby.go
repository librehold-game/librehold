package app

import (
	"context"
	"strconv"

	"librehold/internal/game"
)

type WorldCard struct {
	ID         int64            `json:"id"`
	Name       string           `json:"name"`
	GameNumber int              `json:"gameNumber,omitempty"`
	Humans     int              `json:"humans"`
	Bots       int              `json:"bots"`
	Paused     bool             `json:"paused"`
	Settings   game.WorldConfig `json:"settings"`
}

type LobbyView struct {
	Mine []WorldCard `json:"mine"`
	Open []WorldCard `json:"open"`
	// Playing are the player worlds the account plays in that have not ended.
	Playing []WorldCard `json:"playing"`
	// Hosted is the account's player world; Host what its host form may offer; PastHosted its ended ones.
	Hosted     *HostedCard `json:"hosted"`
	Host       HostLimits  `json:"host"`
	PastHosted []GameRef   `json:"pastHosted"`
}

const pastHostedShown = 10

// Lobby lists the long worlds the account plays in and the open ones it could join, and its player world.
func (s *Service) Lobby(ctx context.Context, acct *Account) (LobbyView, error) {
	worlds, err := s.worlds.List(ctx)
	if err != nil {
		return LobbyView{}, err
	}
	ms, err := s.worlds.MembershipsOf(ctx, acct.ID)
	if err != nil {
		return LobbyView{}, err
	}
	mine := map[int64]bool{}
	for _, m := range ms {
		mine[m.WorldID] = true
	}
	view := LobbyView{Mine: []WorldCard{}, Open: []WorldCard{}, Playing: []WorldCard{}}
	if view.Host, err = s.hostLimits(ctx, acct, worlds); err != nil {
		return LobbyView{}, err
	}
	if view.Hosted, err = s.Hosted(ctx, acct); err != nil {
		return LobbyView{}, err
	}
	if view.PastHosted, err = s.worlds.HostedGames(ctx, acct.ID, pastHostedShown); err != nil {
		return LobbyView{}, err
	}
	for _, w := range worlds {
		card := WorldCard{ID: w.ID, Name: w.Name, GameNumber: w.GameNumber, Humans: w.Humans, Bots: w.Bots, Paused: w.PausedAtMs != nil, Settings: w.Config}
		if livePlayerWorld(w) && mine[w.ID] {
			view.Playing = append(view.Playing, card)
		}
		if game.IsShort(&w.Config) {
			continue
		}
		switch {
		case mine[w.ID]:
			view.Mine = append(view.Mine, card)
		case w.Status == "open":
			view.Open = append(view.Open, card)
		}
	}
	return view, nil
}

// maxJoins is how many times one account may join a world: leaving and coming back must not become a way to
// trade a village under attack for a fresh, protected one.
const maxJoins = 3

// humanIDFor is h<accountID>, or h<accountID>_<n> with the first n ≥ 2 not taken when the account played
// this world before: a returning player starts fresh beside the one who left. false once it joined maxJoins times.
func humanIDFor(w WorldTx, accountID int64) (game.PlayerID, bool) {
	base := "h" + strconv.FormatInt(accountID, 10)
	id := base
	for n := 2; w.Player(id) != nil; n++ {
		if n > maxJoins {
			return "", false
		}
		id = base + "_" + strconv.Itoa(n)
	}
	return id, true
}

// JoinWorld spawns the account in an open world as a new human named after the account.
func (s *Service) JoinWorld(ctx context.Context, acct *Account, worldID int64, q game.Quadrant) (game.PlayerID, error) {
	target, err := s.worlds.Get(ctx, worldID)
	if err != nil {
		return "", err
	}
	if acct.IsGuest && !game.IsInstant(&target.Config) || target.Config.Kind == game.KindPlayer {
		return "", ErrForbidden
	}
	if !game.IsInstant(&target.Config) {
		return s.joinWorld(ctx, acct, worldID, q)
	}
	var id game.PlayerID
	err = s.worlds.LockAccount(ctx, acct.ID, func() error {
		current, err := s.runningInstantOf(ctx, acct.ID)
		if err != nil {
			return err
		}
		if current != 0 && current != worldID {
			return &ErrValidation{Field: "world", Code: "inAnotherGame"}
		}
		id, err = s.joinWorld(ctx, acct, worldID, q)
		return err
	})
	return id, err
}

func (s *Service) joinWorld(ctx context.Context, acct *Account, worldID int64, q game.Quadrant) (game.PlayerID, error) {
	if _, _, err := s.ProcessDue(ctx, worldID, 0); err != nil {
		return "", err
	}
	var id game.PlayerID
	err := s.worlds.Run(ctx, worldID, func(w WorldTx) error {
		row := w.Row()
		if row.Status != "open" {
			return &ErrValidation{Field: "world", Code: "worldClosed"}
		}
		if row.PausedAtMs != nil {
			return ErrWorldPaused
		}
		if row.State.EndedAt != nil {
			return ErrWorldEnded
		}
		if cfg := &row.Config; game.IsInstant(cfg) {
			humans := 0
			for _, p := range w.Players() {
				if p.IsHuman && p.Status == game.StatusActive {
					humans++
				}
			}
			if humans >= cfg.HumanSeats || WorldNow(row, s.clock.NowMs()) >= int64(cfg.DurationMin)*game.MsPerMinute {
				return &ErrValidation{Field: "world", Code: "worldClosed"}
			}
		}
		for _, p := range w.Players() {
			if p.IsHuman && p.AccountID == acct.ID {
				return &ErrValidation{Field: "world", Code: "alreadyMember"}
			}
		}
		now := WorldNow(row, s.clock.NowMs())
		w.SetNow(now)
		if _, err := processDue(w, now, 0); err != nil {
			return err
		}
		w.SetNow(now)
		var ok bool
		if id, ok = humanIDFor(w, acct.ID); !ok {
			return &ErrValidation{Field: "world", Code: "rejoinLimit"}
		}
		return joinAs(w, id, acct.Username, acct.ID, acct.PlayerKey, acct.IsGuest, q)
	})
	if err != nil {
		return "", err
	}
	return id, nil
}

// LeaveWorld makes the account's player quit; its villages go barbarian later, as a bot's would, and the
// account is no longer a member.
func (s *Service) LeaveWorld(ctx context.Context, acct *Account, worldID int64) error {
	return s.inWorld(ctx, acct, worldID, func(w WorldTx, me game.PlayerID) error {
		leaveAs(w, me)
		return nil
	})
}

// joinAs spawns the human, its join and first snapshot in the action log; the player key names it there.
func joinAs(w WorldTx, id game.PlayerID, name string, accountID int64, key string, guest bool, q game.Quadrant) error {
	if _, err := game.JoinHuman(w, id, name, accountID, q); err != nil {
		return err
	}
	p := w.Player(id)
	p.Key, p.IsGuest = key, guest
	w.LogAction(game.ActorOf(p), false, "join", joinEntry{PlayerID: id, Quadrant: q, Guest: guest})
	game.SnapshotHuman(w, id)
	game.SettleQuests(w, id)
	return nil
}

func leaveAs(w WorldTx, me game.PlayerID) {
	w.LogAction(game.ActorOf(w.Player(me)), false, "leave", struct{}{})
	game.QuitPlayer(w, me)
	w.Player(me).AccountID = 0
}
