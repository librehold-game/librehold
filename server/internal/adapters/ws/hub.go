// Package ws pushes refresh hints to players over WebSocket: what changed, never the data itself.
package ws

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"

	"librehold/internal/app"
	"librehold/internal/game"
)

const (
	pingEvery    = 25_000 // ms of the clock
	reauthEvery  = 60_000
	tick         = 250 * time.Millisecond
	sendBuffer   = 64
	writeTimeout = 10 * time.Second

	closeUnauthenticated websocket.StatusCode = 4401
	closeNotMember       websocket.StatusCode = 4403
	closeTooSlow         websocket.StatusCode = 4408
)

type message struct {
	Type       string   `json:"type"`
	Topics     []string `json:"topics,omitempty"`
	VillageIDs []string `json:"villageIds,omitempty"`
	ServerTime *int64   `json:"serverTime,omitempty"`
	WorldNow   *int64   `json:"worldNow,omitempty"`
}

type client struct {
	acct   *app.Account
	token  string
	world  int64
	player game.PlayerID
	// spectator is a player world's host watching it: no player, every update of the world.
	spectator bool
	send      chan message
	kick      chan websocket.StatusCode
}

// Hub keeps the live connections and forwards every update to the players it is for.
type Hub struct {
	svc     *app.Service
	updates app.Updates
	clock   app.Clock
	ready   chan struct{}
	done    chan struct{} // closed when Run returns; every connection then closes

	mu      sync.Mutex
	clients map[int64]map[*client]struct{}
}

func NewHub(svc *app.Service, updates app.Updates, clock app.Clock) *Hub {
	return &Hub{svc: svc, updates: updates, clock: clock, ready: make(chan struct{}), done: make(chan struct{}),
		clients: map[int64]map[*client]struct{}{}}
}

// Ready is closed once Run is subscribed to the updates.
func (h *Hub) Ready() <-chan struct{} { return h.ready }

// Run forwards updates until ctx ends.
func (h *Hub) Run(ctx context.Context) error {
	defer close(h.done)
	ch, err := h.updates.Subscribe(ctx)
	if err != nil {
		return err
	}
	close(h.ready)
	for {
		select {
		case <-ctx.Done():
			return nil
		case u, ok := <-ch:
			if !ok {
				return nil
			}
			h.forward(ctx, u)
		}
	}
}

func (h *Hub) forward(ctx context.Context, u app.Update) {
	h.mu.Lock()
	var targets []*client
	for c := range h.clients[u.WorldID] {
		if u.PlayerID == "" || c.spectator || c.player == u.PlayerID {
			targets = append(targets, c)
		}
	}
	h.mu.Unlock()
	if len(targets) == 0 {
		return
	}
	now, wall, err := h.svc.WorldTime(ctx, u.WorldID)
	if err != nil {
		slog.Warn("live: world time", "world", u.WorldID, "err", err)
		return
	}
	m := message{Type: "update", Topics: u.Topics, VillageIDs: u.VillageIDs, ServerTime: &wall, WorldNow: &now}
	for _, c := range targets {
		select {
		case c.send <- m:
		default:
			c.close(closeTooSlow)
		}
	}
}

func (c *client) close(code websocket.StatusCode) {
	select {
	case c.kick <- code:
	default:
	}
}

// tokenOf reads the session from the Authorization header, else the cookie; never from the address, which ends up
// in logs.
func tokenOf(r *http.Request) string {
	if h := r.Header.Get("Authorization"); strings.HasPrefix(h, "Bearer ") {
		return strings.TrimPrefix(h, "Bearer ")
	}
	if c, err := r.Cookie("lh_session"); err == nil {
		return c.Value
	}
	return ""
}

// ServeHTTP is GET /api/v1/live?world={wid}: a member's connection to one world, or its host's.
func (h *Hub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	world, err := strconv.ParseInt(r.URL.Query().Get("world"), 10, 64)
	if err != nil {
		http.Error(w, "world?", http.StatusBadRequest)
		return
	}
	token := tokenOf(r)
	acct, err := h.svc.Authenticate(r.Context(), token)
	if err != nil {
		http.Error(w, "unauthenticated", http.StatusUnauthorized)
		return
	}
	player, err := h.svc.MemberOf(r.Context(), acct, world)
	spectator := false
	if errors.Is(err, app.ErrNotMember) {
		spectator, err = h.svc.IsHost(r.Context(), acct, world)
		if err == nil && !spectator {
			http.Error(w, "not a member", http.StatusForbidden)
			return
		}
	}
	if err != nil {
		http.Error(w, "error", http.StatusInternalServerError)
		return
	}
	conn, err := websocket.Accept(w, r, nil)
	if err != nil {
		return
	}
	c := &client{acct: acct, token: token, world: world, player: player, spectator: spectator, send: make(chan message, sendBuffer),
		kick: make(chan websocket.StatusCode, 1)}
	h.add(c)
	defer h.remove(c)
	h.serve(r.Context(), conn, c)
}

func (h *Hub) add(c *client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.clients[c.world] == nil {
		h.clients[c.world] = map[*client]struct{}{}
	}
	h.clients[c.world][c] = struct{}{}
}

func (h *Hub) remove(c *client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.clients[c.world], c)
	if len(h.clients[c.world]) == 0 {
		delete(h.clients, c.world)
	}
}

func write(ctx context.Context, conn *websocket.Conn, m message) error {
	ctx, cancel := context.WithTimeout(ctx, writeTimeout)
	defer cancel()
	return wsjson.Write(ctx, conn, m)
}

// serve writes the connection's messages; the clock decides when to ping and when to check the session again.
func (h *Hub) serve(ctx context.Context, conn *websocket.Conn, c *client) {
	ctx = conn.CloseRead(ctx)
	now, wall, err := h.svc.WorldTime(ctx, c.world)
	if err != nil || write(ctx, conn, message{Type: "hello", ServerTime: &wall, WorldNow: &now}) != nil {
		_ = conn.CloseNow()
		return
	}
	lastPing, lastAuth := h.clock.NowMs(), h.clock.NowMs()
	ticker := time.NewTicker(tick)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			_ = conn.CloseNow()
			return
		case <-h.done:
			_ = conn.Close(websocket.StatusGoingAway, "")
			return
		case code := <-c.kick:
			_ = conn.Close(code, "")
			return
		case m := <-c.send:
			if write(ctx, conn, m) != nil {
				_ = conn.CloseNow()
				return
			}
		case <-ticker.C:
			now := h.clock.NowMs()
			if now-lastAuth >= reauthEvery {
				lastAuth = now
				if code, ok := h.recheck(ctx, c); !ok {
					_ = conn.Close(code, "")
					return
				}
			}
			if now-lastPing >= pingEvery {
				lastPing = now
				if write(ctx, conn, message{Type: "ping"}) != nil {
					_ = conn.CloseNow()
					return
				}
			}
		}
	}
}

// recheck confirms the session and the membership (or, for a spectator, the hosting) are still valid.
func (h *Hub) recheck(ctx context.Context, c *client) (websocket.StatusCode, bool) {
	acct, err := h.svc.Authenticate(ctx, c.token)
	if err != nil {
		if errors.Is(err, app.ErrUnauthenticated) {
			return closeUnauthenticated, false
		}
		return 0, true
	}
	if c.spectator {
		if host, err := h.svc.IsHost(ctx, acct, c.world); err == nil && !host {
			return closeNotMember, false
		}
		return 0, true
	}
	player, err := h.svc.MemberOf(ctx, acct, c.world)
	if errors.Is(err, app.ErrNotMember) || (err == nil && player != c.player) {
		return closeNotMember, false
	}
	return 0, true
}
