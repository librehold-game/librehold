package ws_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"

	"librehold/internal/adapters/httpapi"
	"librehold/internal/adapters/memory"
	"librehold/internal/adapters/security"
	"librehold/internal/adapters/ws"
	"librehold/internal/app"
	"librehold/internal/game"
	"librehold/internal/game/gametest"
)

type clock struct{ ms atomic.Int64 }

func (c *clock) NowMs() int64 { return c.ms.Load() }

type plainHasher struct{}

func (plainHasher) Hash(_ context.Context, pw string) (string, error) { return "plain:" + pw, nil }
func (plainHasher) Verify(_ context.Context, hash, pw string) (bool, error) {
	return hash == "plain:"+pw, nil
}

type env struct {
	srv    *httptest.Server
	svc    *app.Service
	worlds *memory.Worlds
	clock  *clock
	world  int64
	tokens map[string]string // username → session token
	homes  map[string]string // username → home village
	ids    map[string]game.PlayerID
}

var admin = &app.Account{ID: 1000, Username: "admin", IsAdmin: true}

// newEnv is a world without bots, with ala and bob joined, and carol registered but outside.
func newEnv(t *testing.T) *env {
	t.Helper()
	ctx := context.Background()
	e := &env{worlds: memory.NewWorlds(), clock: &clock{}, tokens: map[string]string{}, homes: map[string]string{},
		ids: map[string]game.PlayerID{}}
	e.clock.ms.Store(time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC).UnixMilli())
	accounts := memory.NewAccounts()
	e.svc = app.NewService(accounts, e.worlds, memory.NewCounters(e.clock), e.clock, plainHasher{}, security.NewTokens(), app.Config{})
	id, err := e.svc.CreateWorld(ctx, admin, "Café World", gametest.TestConfig())
	if err != nil {
		t.Fatal(err)
	}
	e.world = id
	for _, name := range []string{"ala", "bob", "carol"} {
		token, acct, err := e.svc.Register(ctx, app.RegisterInput{Username: name, Password: "long-password",
			PasswordConfirm: "long-password", Client: name})
		if err != nil {
			t.Fatal(err)
		}
		e.tokens[name] = token
		if name != "carol" {
			if _, err := e.svc.JoinWorld(ctx, acct, id, game.QuadrantRandom); err != nil {
				t.Fatal(err)
			}
		}
	}
	_ = e.worlds.Read(ctx, id, func(w app.WorldTx) error {
		for _, p := range w.Players() {
			e.homes[p.Name] = p.VillageIDs[0]
			e.ids[p.Name] = p.ID
		}
		return nil
	})
	hubCtx, cancel := context.WithCancel(context.Background())
	hub := ws.NewHub(e.svc, e.worlds.Updates(), e.clock)
	done := make(chan struct{})
	go func() { _ = hub.Run(hubCtx); close(done) }()
	mux := http.NewServeMux()
	mux.Handle("/api/v1/live", hub)
	mux.Handle("/api/", httpapi.New(e.svc, httpapi.Options{}))
	e.srv = httptest.NewServer(mux)
	t.Cleanup(func() { e.srv.Close(); cancel(); <-done })
	<-hub.Ready()
	return e
}

type message struct {
	Type       string   `json:"type"`
	Topics     []string `json:"topics"`
	VillageIDs []string `json:"villageIds"`
	ServerTime int64    `json:"serverTime"`
	WorldNow   *int64   `json:"worldNow"`
}

type socket struct {
	t    *testing.T
	conn *websocket.Conn
}

func (e *env) dial(t *testing.T, user string) (*socket, error) {
	t.Helper()
	url := "ws" + e.srv.URL[len("http"):] + "/api/v1/live?world=" + strconv.FormatInt(e.world, 10)
	conn, _, err := websocket.Dial(context.Background(), url,
		&websocket.DialOptions{HTTPHeader: http.Header{"Authorization": {"Bearer " + e.tokens[user]}}})
	if err != nil {
		return nil, err
	}
	s := &socket{t: t, conn: conn}
	t.Cleanup(func() { _ = conn.CloseNow() })
	return s, nil
}

func (e *env) connect(t *testing.T, user string) *socket {
	t.Helper()
	s, err := e.dial(t, user)
	if err != nil {
		t.Fatal(err)
	}
	if m := s.next(); m.Type != "hello" || m.ServerTime != e.clock.NowMs() || m.WorldNow == nil || *m.WorldNow != 0 {
		t.Fatalf("hello %+v", m)
	}
	return s
}

// next is the next non-ping message.
func (s *socket) next() message {
	s.t.Helper()
	for {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		var m message
		err := wsjson.Read(ctx, s.conn, &m)
		cancel()
		if err != nil {
			s.t.Fatalf("read: %v", err)
		}
		if m.Type != "ping" {
			return m
		}
	}
}

func (e *env) rename(t *testing.T, user, name string) {
	t.Helper()
	err := e.worlds.Run(context.Background(), e.world, func(w app.WorldTx) error {
		w.Village(e.homes[user]).Name = name
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func (e *env) touchHuman(t *testing.T, user string) {
	t.Helper()
	err := e.worlds.Run(context.Background(), e.world, func(w app.WorldTx) error {
		w.Human(e.ids[user]).AggressorUntil++
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestHelloAndTargetedUpdate(t *testing.T) {
	e := newEnv(t)
	ala, bob := e.connect(t, "ala"), e.connect(t, "bob")
	e.touchHuman(t, "ala")
	if m := ala.next(); m.Type != "update" || len(m.Topics) != 1 || m.Topics[0] != app.TopicState {
		t.Fatalf("ala got %+v", m)
	}
	e.rename(t, "bob", "Fortress")
	m := bob.next()
	if m.Type != "update" || m.Topics[0] != app.TopicState {
		t.Fatalf("bob's first message is not his own update: %+v", m)
	}
}

func TestWorldWideUpdate(t *testing.T) {
	e := newEnv(t)
	ala, bob := e.connect(t, "ala"), e.connect(t, "bob")
	e.clock.ms.Add(5000)
	e.rename(t, "ala", "Café Hill")
	if m := ala.next(); m.Topics[0] != app.TopicState {
		t.Fatalf("ala's own update first: %+v", m)
	}
	for _, s := range []*socket{ala, bob} {
		m := s.next()
		if m.Type != "update" || m.Topics[0] != app.TopicMap || len(m.VillageIDs) != 1 || m.VillageIDs[0] != e.homes["ala"] {
			t.Fatalf("world update %+v", m)
		}
		if m.WorldNow == nil || *m.WorldNow != 5000 || m.ServerTime != e.clock.NowMs() {
			t.Fatalf("times %+v", m)
		}
	}
}

func TestTokenInTheAddressRefused(t *testing.T) {
	e := newEnv(t)
	url := "ws" + e.srv.URL[len("http"):] + "/api/v1/live?world=" + strconv.FormatInt(e.world, 10) + "&token=" + e.tokens["ala"]
	if _, resp, err := websocket.Dial(context.Background(), url, nil); err == nil || resp == nil || resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("a token in the query string: %v", err)
	}
}

func TestNonMemberRejected(t *testing.T) {
	e := newEnv(t)
	if _, err := e.dial(t, "carol"); err == nil {
		t.Fatal("a non-member connected")
	}
	e.tokens["nobody"] = "made-up"
	if _, err := e.dial(t, "nobody"); err == nil {
		t.Fatal("an unknown token connected")
	}
}

func TestSocketClosesAfterLogout(t *testing.T) {
	e := newEnv(t)
	ala := e.connect(t, "ala")
	req, _ := http.NewRequest("POST", e.srv.URL+"/api/v1/auth/logout", nil)
	req.Header.Set("Authorization", "Bearer "+e.tokens["ala"])
	res, err := http.DefaultClient.Do(req)
	if err != nil || res.StatusCode != 204 {
		t.Fatalf("logout %v %v", res, err)
	}
	e.clock.ms.Add(60_000)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for {
		var m message
		err := wsjson.Read(ctx, ala.conn, &m)
		if err == nil {
			continue
		}
		var ce websocket.CloseError
		if !errors.As(err, &ce) || ce.Code != 4401 {
			t.Fatalf("closed with %v", err)
		}
		return
	}
}

func TestSpectatorReceivesWorldUpdates(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	carol, err := e.svc.Authenticate(ctx, e.tokens["carol"])
	if err != nil {
		t.Fatal(err)
	}
	ala, err := e.svc.Authenticate(ctx, e.tokens["ala"])
	if err != nil {
		t.Fatal(err)
	}
	card, err := e.svc.CreatePlayerWorld(ctx, carol, app.HostForm{Name: "Watched"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.JoinByInvite(ctx, ala, card.InviteCode); err != nil {
		t.Fatal(err)
	}
	e.world = card.WorldID
	host := e.connect(t, "carol")
	if _, err := e.dial(t, "bob"); err == nil {
		t.Fatal("a stranger connected to the player world")
	}
	err = e.worlds.Run(ctx, card.WorldID, func(w app.WorldTx) error {
		for _, p := range w.Players() {
			if p.IsHuman {
				w.Human(p.ID).AggressorUntil++
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if m := host.next(); m.Type != "update" || len(m.Topics) != 1 || m.Topics[0] != app.TopicState {
		t.Fatalf("the host got %+v", m)
	}
}
