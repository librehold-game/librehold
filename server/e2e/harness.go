//go:build e2e

// Package e2e runs the built server binary against Postgres and talks to it like the web client does.
package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/jackc/pgx/v5/pgxpool"

	"librehold/internal/adapters/postgres/pgtest"
	"librehold/internal/app"
	"librehold/internal/game"
)

// The server's clock runs this many times faster than the wall clock: a world minute is 0.6 s.
const timeScale = 100

const (
	adminName = "admin"
	adminPass = "admin-password"
	password  = "long-password"
)

var binary string

func root() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..")
}

func webDir() string { return filepath.Join(root(), "..", "dist") }

// setup checks the web build, builds the server binary and starts Postgres around the tests.
func setup(m *testing.M) {
	if _, err := os.Stat(filepath.Join(webDir(), "account", "account.html")); err != nil {
		fmt.Fprintln(os.Stderr, "e2e: build the web first: npm run build:web")
		os.Exit(1)
	}
	dir, err := os.MkdirTemp("", "vge2e")
	if err == nil {
		binary = filepath.Join(dir, "server")
		cmd := exec.Command("go", "build", "-o", binary, "./cmd/server")
		cmd.Dir = root()
		var out []byte
		if out, err = cmd.CombinedOutput(); err != nil {
			fmt.Fprintln(os.Stderr, string(out))
		}
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "e2e:", err)
		os.Exit(1)
	}
	pgtest.Main(m)
}

// ---- server processes ----

type server struct {
	t     *testing.T
	db    string
	port  int
	epoch int64
	scale int
	env   []string // extra settings, NAME=value
	cmd   *exec.Cmd
	log   *bytes.Buffer
	url   string
}

func freePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

// newDB is a fresh migrated database's name; the test connects on its own when it needs to.
func newDB(t *testing.T) string {
	t.Helper()
	p := pgtest.New(t)
	name := p.Config().ConnConfig.Database
	p.Close()
	return name
}

func startServer(t *testing.T, db string, epoch int64) *server {
	t.Helper()
	return startServerAt(t, db, epoch, timeScale)
}

// startServerAt runs the server with its clock scale times faster than the wall clock (1 = real time).
func startServerAt(t *testing.T, db string, epoch int64, scale int, env ...string) *server {
	t.Helper()
	s := &server{t: t, db: db, port: freePort(t), epoch: epoch, scale: scale, env: env}
	s.url = fmt.Sprintf("http://127.0.0.1:%d", s.port)
	s.start()
	t.Cleanup(func() {
		if s.cmd != nil && s.cmd.ProcessState == nil {
			_ = s.cmd.Process.Kill()
			_ = s.cmd.Wait()
		}
		if t.Failed() {
			t.Logf("server log (%s):\n%s", s.db, s.log.String())
		}
	})
	return s
}

func (s *server) start() {
	s.t.Helper()
	s.log = &bytes.Buffer{}
	cmd := exec.Command(binary)
	cmd.Env = append(os.Environ(),
		"LH_DATABASE_URL="+pgtest.URL(s.db),
		"LH_HTTP_ADDR="+fmt.Sprintf("127.0.0.1:%d", s.port),
		"LH_WEB_DIR="+webDir(),
		"LH_COOKIE_SECURE=false",
		"LH_ADMIN_USERNAME="+adminName,
		"LH_ADMIN_PASSWORD="+adminPass,
		"LH_REGISTRATIONS_PER_HOUR=1000",
		"LH_GUESTS_PER_HOUR=1000",
		"LH_WORLD_REQUESTS_PER_MINUTE=1000000",
		"LH_TIME_SCALE="+strconv.Itoa(s.scale),
		"LH_TIME_EPOCH_MS="+strconv.FormatInt(s.epoch, 10),
	)
	cmd.Env = append(cmd.Env, s.env...)
	cmd.Stdout, cmd.Stderr = s.log, s.log
	if err := cmd.Start(); err != nil {
		s.t.Fatal(err)
	}
	s.cmd = cmd
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		if res, err := http.Get(s.url + "/api/v1/me"); err == nil {
			res.Body.Close()
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	s.t.Fatalf("server did not start:\n%s", s.log.String())
}

// kill9 ends the process without any chance to clean up.
func (s *server) kill9() {
	_ = s.cmd.Process.Signal(syscall.SIGKILL)
	_ = s.cmd.Wait()
}

// stop is a graceful SIGTERM shutdown.
func (s *server) stop() {
	_ = s.cmd.Process.Signal(syscall.SIGTERM)
	_ = s.cmd.Wait()
}

// ---- clients ----

type client struct {
	t     *testing.T
	s     *server
	token string
	name  string
	// forwarded is the X-Forwarded-For the client sends: a visitor from the internet behind a proxy the server trusts.
	forwarded string

	mu     sync.Mutex
	bodies []string // every response body, for the hidden-information checks
}

type reply struct {
	status int
	body   []byte
}

func (r reply) code() string {
	var e struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	_ = json.Unmarshal(r.body, &e)
	return e.Error.Code
}

func (c *client) do(method, path string, body any) reply {
	c.t.Helper()
	var rd io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			c.t.Fatal(err)
		}
		rd = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, c.s.url+path, rd)
	if err != nil {
		c.t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", c.s.url)
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	if c.forwarded != "" {
		req.Header.Set("X-Forwarded-For", c.forwarded)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return reply{status: 0, body: []byte(err.Error())}
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	c.mu.Lock()
	c.bodies = append(c.bodies, string(b))
	c.mu.Unlock()
	return reply{status: res.StatusCode, body: b}
}

// ok sends a request that must succeed and decodes its body into out (when not nil).
func (c *client) ok(method, path string, body, out any) {
	c.t.Helper()
	r := c.do(method, path, body)
	if r.status < 200 || r.status > 299 {
		c.t.Fatalf("%s %s %v: %d %s", method, path, body, r.status, r.body)
	}
	if out != nil {
		if err := json.Unmarshal(r.body, out); err != nil {
			c.t.Fatalf("%s %s: %v in %s", method, path, err, r.body)
		}
	}
}

func (s *server) login(t *testing.T, name, pw string) *client {
	t.Helper()
	c := &client{t: t, s: s, name: name}
	var out struct {
		Token string `json:"token"`
	}
	c.ok("POST", "/api/v1/auth/login", map[string]string{"username": name, "password": pw}, &out)
	c.token = out.Token
	return c
}

func (s *server) admin(t *testing.T) *client { return s.login(t, adminName, adminPass) }

func (s *server) register(t *testing.T, name string) *client {
	t.Helper()
	c := &client{t: t, s: s, name: name}
	var out struct {
		Token string `json:"token"`
	}
	c.ok("POST", "/api/v1/auth/register", map[string]string{"username": name, "password": password,
		"passwordConfirm": password}, &out)
	c.token = out.Token
	return c
}

// guest opens a guest account, as the Play button does for a visitor.
func (s *server) guest(t *testing.T) *client {
	t.Helper()
	c := &client{t: t, s: s}
	var out struct {
		Account struct {
			Username string `json:"username"`
		} `json:"account"`
		Token string `json:"token"`
	}
	c.ok("POST", "/api/v1/auth/guest", map[string]string{"locale": "en"}, &out)
	c.token, c.name = out.Token, out.Account.Username
	return c
}

// play asks for a seat in an instant game: a world id, or how long to wait for one.
func (c *client) play(t *testing.T) (worldID, waitMs int64) {
	t.Helper()
	var out app.PlayResult
	c.ok("POST", "/api/v1/instant/play", map[string]string{}, &out)
	return out.WorldID, out.WaitMs
}

// playIn joins the running instant game worldID.
func (c *client) playIn(t *testing.T, worldID int64) {
	t.Helper()
	var out app.PlayResult
	c.ok("POST", "/api/v1/instant/play", map[string]int64{"worldId": worldID}, &out)
	if out.WorldID != worldID {
		t.Fatalf("joined %d, wanted %d", out.WorldID, worldID)
	}
}

// claim turns the guest into a player named name, with the harness password.
func (c *client) claim(t *testing.T, name string) {
	t.Helper()
	c.ok("POST", "/api/v1/auth/claim", map[string]string{"username": name, "password": password, "passwordConfirm": password}, nil)
	c.name = name
}

// world creates a world from the default settings with the given changes; returns its id.
func (a *client) world(change func(cfg *game.WorldConfig)) int64 {
	a.t.Helper()
	cfg := game.DefaultWorldConfig
	change(&cfg)
	var out struct {
		ID int64 `json:"id"`
	}
	a.ok("POST", "/api/v1/admin/worlds", map[string]any{"name": "e2e Café World", "settings": cfg}, &out)
	return out.ID
}

func wpath(wid int64, rest string) string { return fmt.Sprintf("/api/v1/worlds/%d%s", wid, rest) }

func (c *client) join(wid int64) { c.joinIn(wid, "") }

// joinIn joins in a quadrant of the map: nw, ne, sw, se, or "" for anywhere.
func (c *client) joinIn(wid int64, quadrant string) {
	c.t.Helper()
	c.ok("POST", wpath(wid, "/join"), map[string]string{"quadrant": quadrant}, nil)
}

func (c *client) state(wid int64) *app.PlayerState {
	c.t.Helper()
	var st app.PlayerState
	c.ok("GET", wpath(wid, "/state"), nil, &st)
	return &st
}

func (c *client) home(wid int64) *game.Village {
	c.t.Helper()
	st := c.state(wid)
	for _, id := range st.Player.VillageIDs {
		if v := st.Villages[id]; v != nil {
			return v
		}
	}
	c.t.Fatal("no village")
	return nil
}

func (c *client) order(wid int64, name string, body any) reply {
	c.t.Helper()
	return c.do("POST", wpath(wid, "/orders/"+name), body)
}

func (c *client) mustOrder(wid int64, name string, body any) *app.PlayerState {
	c.t.Helper()
	var st app.PlayerState
	c.ok("POST", wpath(wid, "/orders/"+name), body, &st)
	return &st
}

func (c *client) reports(wid int64) []game.Report {
	c.t.Helper()
	var out []game.Report
	c.ok("GET", wpath(wid, "/reports?limit=100"), nil, &out)
	return out
}

func (c *client) tiles(wid int64, x, y, r int) []app.MapTile {
	c.t.Helper()
	var out []app.MapTile
	c.ok("GET", wpath(wid, fmt.Sprintf("/map?x0=%d&y0=%d&x1=%d&y1=%d", x-r, y-r, x+r, y+r)), nil, &out)
	return out
}

// ---- player worlds ----

func ptr[T any](v T) *T { return &v }

// host creates the account's player world; it waits for the host's Start.
func (c *client) host(t *testing.T, form app.HostForm) *app.HostedCard {
	t.Helper()
	var card app.HostedCard
	c.ok("POST", "/api/v1/hosted", form, &card)
	return &card
}

// joinInvite opens an invite link as the account; the result is set when the reply is 200.
func (c *client) joinInvite(t *testing.T, code string) (app.JoinResult, reply) {
	t.Helper()
	r := c.do("POST", "/api/v1/invites/"+code+"/join", map[string]string{})
	var res app.JoinResult
	if r.status == 200 {
		if err := json.Unmarshal(r.body, &res); err != nil {
			t.Fatal(err)
		}
	}
	return res, r
}

// hostAction is one of the host's controls: start, pause, resume, end, kick or invite.
func (c *client) hostAction(wid int64, action string, body any) reply {
	return c.do("POST", wpath(wid, "/host/"+action), body)
}

// eventually retries cond every 100 ms until it holds.
func eventually(t *testing.T, within time.Duration, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(within)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// ---- the live socket ----

type socket struct {
	conn *websocket.Conn
	msgs chan map[string]any
}

func (c *client) live(wid int64) *socket {
	c.t.Helper()
	url := strings.Replace(c.s.url, "http", "ws", 1) + fmt.Sprintf("/api/v1/live?world=%d", wid)
	conn, _, err := websocket.Dial(context.Background(), url,
		&websocket.DialOptions{HTTPHeader: http.Header{"Authorization": {"Bearer " + c.token}}})
	if err != nil {
		c.t.Fatal(err)
	}
	s := &socket{conn: conn, msgs: make(chan map[string]any, 256)}
	go func() {
		for {
			var m map[string]any
			if err := wsjson.Read(context.Background(), conn, &m); err != nil {
				close(s.msgs)
				return
			}
			s.msgs <- m
		}
	}()
	c.t.Cleanup(func() { _ = conn.CloseNow() })
	return s
}

// ---- preparing villages ----

// boost changes a stored village under the world's lock, like the server's own transactions do.
func boost(t *testing.T, db string, wid int64, vid string, change func(v *game.Village)) {
	t.Helper()
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, pgtest.URL(db))
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `select pg_advisory_xact_lock(42, $1::int)`, wid); err != nil {
		t.Fatal(err)
	}
	var data []byte
	if err := tx.QueryRow(ctx, `select data from villages where world_id = $1 and id = $2`, wid, vid).Scan(&data); err != nil {
		t.Fatal(err)
	}
	var v game.Village
	if err := json.Unmarshal(data, &v); err != nil {
		t.Fatal(err)
	}
	change(&v)
	game.RecomputePoints(&v)
	data, _ = json.Marshal(&v)
	if _, err := tx.Exec(ctx, `update villages set data = $3, points = $4, name = $5 where world_id = $1 and id = $2`,
		wid, vid, data, v.Points, v.Name); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
}

// army makes a village strong and rich enough for every order of the play loop.
func army(v *game.Village) {
	for b, lv := range map[game.BuildingID]int{game.BMain: 20, game.BBarracks: 20, game.BStable: 15, game.BWorkshop: 5,
		game.BAcademy: 1, game.BSmithy: 20, game.BMarket: 10, game.BWood: 20, game.BClay: 20,
		game.BIron: 20, game.BFarm: 30, game.BStorage: 30, game.BWall: 10} {
		v.Buildings[b] = lv
	}
	v.Resources = game.Resources{Wood: 300_000, Clay: 300_000, Iron: 300_000}
	for u, n := range map[game.UnitID]int{game.USpear: 1000, game.UAxe: 1000, game.UScout: 200, game.ULightCav: 500, game.URam: 50, game.UNoble: 3} {
		v.Units[u] = n
	}
	for _, u := range game.UnitIDs {
		v.Research[u] = 1
	}
}

// clonedDB copies a database nobody is connected to and returns the copy's name.
func clonedDB(t *testing.T, from string) string {
	t.Helper()
	p := pgtest.Clone(t, from)
	name := p.Config().ConnConfig.Database
	p.Close()
	return name
}
