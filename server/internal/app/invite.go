package app

import (
	"context"
	"slices"

	"librehold/internal/game"
)

// InviteInfo is what the invite page shows anyone who has the link.
type InviteInfo struct {
	Name           string `json:"name"`
	Host           string `json:"host"`
	Status         string `json:"status"` // waiting | running | ended
	Humans         int    `json:"humans"`
	Seats          int    `json:"seats"`
	Open           bool   `json:"open"`
	Reason         string `json:"reason,omitempty"` // worldFull | joinClosed | worldEnded
	RegisteredOnly bool   `json:"registeredOnly"`
}

// JoinResult is where an invite takes the account: its village, or the host's view (Watch).
type JoinResult struct {
	WorldID int64 `json:"worldId"`
	Watch   bool  `json:"watch,omitempty"`
}

// HostedView is a player world's host and waiting room as its players see them.
type HostedView struct {
	HostName     string `json:"hostName"`
	Waiting      bool   `json:"waiting"`
	Humans       int    `json:"humans"`
	Seats        int    `json:"seats"`
	PausedByHost bool   `json:"pausedByHost"`
	ResumesAtMs  int64  `json:"resumesAt,omitempty"` // wall ms
}

func (s *Service) worldByInvite(ctx context.Context, code string) (*WorldRow, error) {
	if code == "" {
		return nil, ErrNotFound
	}
	return s.worlds.WorldByInvite(ctx, code)
}

func activeHumans(w game.World) int {
	n := 0
	for _, p := range w.Players() {
		if p.IsHuman && p.Status == game.StatusActive {
			n++
		}
	}
	return n
}

// hostPauseEndsAt is when the host's open pause ends by itself (wall ms): ten minutes, or what is left of the thirty.
func hostPauseEndsAt(row *WorldRow) int64 {
	return *row.PausedAtMs + min(hostPauseMaxMs, max(0, hostPauseTotalMs-row.Host.HostPausedMs))
}

// joinRefusal is why nobody new may join now, or "" while the world takes players.
func (s *Service) joinRefusal(w WorldTx) string {
	row := w.Row()
	switch {
	case row.State.EndedAt != nil:
		return "worldEnded"
	case activeHumans(w) >= row.Config.HumanSeats:
		return "worldFull"
	case row.Host.StartedAtMs != nil && (row.Config.JoinWindowMin == 0 ||
		WorldNow(row, s.clock.NowMs()) >= int64(row.Config.JoinWindowMin)*game.MsPerMinute):
		return "joinClosed"
	}
	return ""
}

func (s *Service) InviteInfo(ctx context.Context, code string) (*InviteInfo, error) {
	row, err := s.worldByInvite(ctx, code)
	if err != nil {
		return nil, err
	}
	var info *InviteInfo
	err = s.worlds.Read(ctx, row.ID, func(w WorldTx) error {
		r := w.Row()
		info = &InviteInfo{Name: r.Name, Host: r.Host.HostName, Status: "running", Humans: activeHumans(w), Seats: r.Config.HumanSeats,
			RegisteredOnly: r.Host.RegisteredOnly, Reason: s.joinRefusal(w)}
		switch {
		case r.State.EndedAt != nil:
			info.Status = "ended"
		case r.Host.StartedAtMs == nil:
			info.Status = "waiting"
		}
		info.Open = info.Reason == ""
		return nil
	})
	return info, err
}

// JoinByInvite puts the account into the world the invite opens: the host goes to watch it, a member back to their
// village, anyone else in with a new village while the world takes players (a host pause does not stop joining).
func (s *Service) JoinByInvite(ctx context.Context, acct *Account, code string) (JoinResult, error) {
	row, err := s.worldByInvite(ctx, code)
	if err != nil {
		return JoinResult{}, err
	}
	res := JoinResult{WorldID: row.ID}
	if row.Host.HostID == acct.ID {
		res.Watch = true
		return res, nil
	}
	if _, _, err := s.ProcessDue(ctx, row.ID, 0); err != nil {
		return JoinResult{}, err
	}
	err = s.worlds.Run(ctx, row.ID, func(w WorldTx) error {
		for _, p := range w.Players() {
			if p.IsHuman && p.AccountID == acct.ID && p.Status == game.StatusActive {
				return nil
			}
		}
		r := w.Row()
		switch {
		case r.State.EndedAt != nil:
			return ErrWorldEnded
		case slices.Contains(r.Host.Banned, acct.ID):
			return &ErrValidation{Field: "world", Code: "banned"}
		case r.Host.RegisteredOnly && acct.IsGuest:
			return &ErrValidation{Field: "world", Code: "registeredOnly"}
		}
		if reason := s.joinRefusal(w); reason != "" {
			return &ErrValidation{Field: "world", Code: reason}
		}
		now := WorldNow(r, s.clock.NowMs())
		w.SetNow(now)
		if _, err := processDue(w, now, 0); err != nil {
			return err
		}
		w.SetNow(now)
		id, ok := humanIDFor(w, acct.ID)
		if !ok {
			return &ErrValidation{Field: "world", Code: "rejoinLimit"}
		}
		return joinAs(w, id, acct.Username, acct.ID, acct.PlayerKey, acct.IsGuest, game.QuadrantRandom)
	})
	if err != nil {
		return JoinResult{}, err
	}
	s.wakeWorker(row.ID)
	return res, nil
}

// hostedView is nil outside player worlds.
func hostedView(w WorldTx) *HostedView {
	row := w.Row()
	h := row.Host
	if h == nil {
		return nil
	}
	v := &HostedView{HostName: h.HostName, Waiting: h.StartedAtMs == nil, Humans: activeHumans(w), Seats: row.Config.HumanSeats}
	if h.StartedAtMs != nil && row.PausedAtMs != nil {
		v.PausedByHost, v.ResumesAtMs = true, hostPauseEndsAt(row)
	}
	return v
}
