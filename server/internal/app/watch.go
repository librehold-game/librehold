package app

import (
	"context"
	"sort"
	"strconv"
	"strings"

	"librehold/internal/game"
)

// The god view: a player world's host sees the whole map, every army on the road and every village, and can
// order nothing — the host is not a member, so every order and the player state stay refused.

type WatchVillage struct {
	ID      string         `json:"id"`
	X       int            `json:"x"`
	Y       int            `json:"y"`
	OwnerID game.PlayerRef `json:"ownerId"`
	Points  int            `json:"points"`
}

// SpectatorState is the whole world at world time now, as its host watches it.
type SpectatorState struct {
	WorldID      int64                          `json:"worldId"`
	WorldName    string                         `json:"worldName"`
	Now          int64                          `json:"now"`
	ServerTimeMs int64                          `json:"serverTime"`
	Paused       bool                           `json:"paused"`
	Config       game.WorldConfig               `json:"config"`
	Hosted       *HostedCard                    `json:"hosted"`
	Instant      *InstantView                   `json:"instant,omitempty"`
	Players      map[game.PlayerID]PublicPlayer `json:"players"`
	Villages     []WatchVillage                 `json:"villages"` // stored and generated
	Commands     []CommandView                  `json:"commands"` // every moving army, with units
	Ranking      []PlayerRow                    `json:"ranking"`
}

// WatchedVillage is one village in full, settled to now.
type WatchedVillage struct {
	ID         string          `json:"id"`
	Name       string          `json:"name"`
	X          int             `json:"x"`
	Y          int             `json:"y"`
	OwnerID    game.PlayerRef  `json:"ownerId"`
	OwnerName  string          `json:"ownerName,omitempty"`
	OwnerIsBot bool            `json:"ownerIsBot,omitempty"`
	Points     int             `json:"points"`
	Buildings  game.Buildings  `json:"buildings"`
	UnitsHome  game.UnitCounts `json:"unitsHome"`
}

// IsHost says whether the account hosts the world.
func (s *Service) IsHost(ctx context.Context, acct *Account, worldID int64) (bool, error) {
	row, err := s.worlds.Get(ctx, worldID)
	if err != nil {
		return false, err
	}
	return row.Host != nil && row.Host.HostID == acct.ID, nil
}

// hostView runs fn on committed state at world time now, after the due events, for the world's host only.
func (s *Service) hostView(ctx context.Context, acct *Account, worldID int64, fn func(w WorldTx) error) error {
	host, err := s.IsHost(ctx, acct, worldID)
	if err != nil {
		return err
	}
	if !host {
		return ErrForbidden
	}
	if _, _, err := s.ProcessDue(ctx, worldID, 0); err != nil {
		return err
	}
	return s.worlds.Read(ctx, worldID, func(w WorldTx) error {
		w.SetNow(WorldNow(w.Row(), s.clock.NowMs()))
		return fn(w)
	})
}

func (s *Service) Watch(ctx context.Context, acct *Account, worldID int64) (*SpectatorState, error) {
	var st *SpectatorState
	err := s.hostView(ctx, acct, worldID, func(w WorldTx) error {
		row := w.Row()
		size := w.Config().MapSize
		st = &SpectatorState{WorldID: row.ID, WorldName: row.Name, Now: w.Now(), ServerTimeMs: s.clock.NowMs(),
			Paused: row.PausedAtMs != nil, Config: *w.Config(), Instant: instantView(w), Players: map[game.PlayerID]PublicPlayer{},
			Villages: []WatchVillage{}, Commands: []CommandView{}}
		players := w.Players()
		for _, p := range players {
			st.Players[p.ID] = publicPlayer(p)
		}
		for _, t := range regionTiles(w, 0, 0, size-1, size-1) {
			st.Villages = append(st.Villages, WatchVillage{ID: t.ID, X: t.X, Y: t.Y, OwnerID: t.OwnerID, Points: t.Points})
		}
		for _, c := range w.MovingCommands(w.Now()) {
			st.Commands = append(st.Commands, ownCommand(c))
		}
		st.Ranking = playerRows(players)
		sort.SliceStable(st.Ranking, func(i, j int) bool { return st.Ranking[i].Points > st.Ranking[j].Points })
		return nil
	})
	if err != nil {
		return nil, err
	}
	if st.Hosted, err = s.hostedCard(ctx, worldID); err != nil {
		return nil, err
	}
	return st, nil
}

// villageXY reads a village id's coordinates ("v:x:y").
func villageXY(id string) (x, y int, ok bool) {
	parts := strings.Split(id, ":")
	if len(parts) != 3 || parts[0] != "v" {
		return 0, 0, false
	}
	x, errX := strconv.Atoi(parts[1])
	y, errY := strconv.Atoi(parts[2])
	return x, y, errX == nil && errY == nil
}

// WatchVillage is one village settled to now in memory, never written; a barbarian village nobody has touched yet
// is its generated template.
func (s *Service) WatchVillage(ctx context.Context, acct *Account, worldID int64, villageID string) (*WatchedVillage, error) {
	var out *WatchedVillage
	err := s.hostView(ctx, acct, worldID, func(w WorldTx) error {
		if v := w.Village(villageID); v != nil {
			game.Touch(w, v)
			out = &WatchedVillage{ID: v.ID, Name: v.Name, X: v.X, Y: v.Y, OwnerID: v.OwnerID, Points: v.Points, Buildings: v.Buildings,
				UnitsHome: game.UnitCounts(v.Units)}
			if p := w.Player(game.PlayerID(v.OwnerID)); v.OwnerID != "" && p != nil {
				out.OwnerName, out.OwnerIsBot = p.Name, !p.IsHuman
			}
			return nil
		}
		x, y, ok := villageXY(villageID)
		size := w.Config().MapSize
		if !ok || x < 0 || y < 0 || x >= size || y >= size || !game.TileHasVillage(w.Seed(), w.Config(), x, y) {
			return ErrNotFound
		}
		tp := game.BarbarianTemplate(w.Seed(), x, y)
		out = &WatchedVillage{ID: villageID, X: x, Y: y, Points: tp.Points, Buildings: tp.Buildings}
		return nil
	})
	return out, err
}
