package httpapi_test

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/netip"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"librehold/internal/adapters/httpapi"
	"librehold/internal/adapters/memory"
	"librehold/internal/adapters/security"
	"librehold/internal/app"
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
	srv      *httptest.Server
	svc      *app.Service
	accounts *memory.Accounts
	clock    *clock
}

// newEnv registers many accounts from one test address, so the registration limit is lifted.
func newEnv(t *testing.T) *env {
	return newEnvWith(t, app.Config{RegistrationsPerHour: 1000})
}

func newEnvWith(t *testing.T, cfg app.Config) *env {
	t.Helper()
	c := &clock{}
	c.ms.Store(time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC).UnixMilli())
	e := &env{accounts: memory.NewAccounts(), clock: c}
	cfg.AdminUsername, cfg.AdminPassword = "admin", "admin-password"
	e.svc = app.NewService(e.accounts, memory.NewWorlds(), memory.NewCounters(c), c, plainHasher{}, security.NewTokens(), cfg)
	if err := e.svc.BootstrapAdmin(context.Background()); err != nil {
		t.Fatal(err)
	}
	e.srv = httptest.NewServer(httpapi.New(e.svc, httpapi.Options{TrustedProxies: []netip.Prefix{netip.MustParsePrefix("127.0.0.1/32")}}))
	t.Cleanup(e.srv.Close)
	return e
}

// client is a browser: a cookie jar, the server's own Origin and JSON bodies. Every client comes from one internet
// address through a proxy on the test server's loopback, so the limits per address apply.
type client struct {
	t      *testing.T
	e      *env
	http   *http.Client
	bearer string
	origin string
	ctype  string
}

func (e *env) browser(t *testing.T) *client {
	jar, _ := cookiejar.New(nil)
	return &client{t: t, e: e, http: &http.Client{Jar: jar}, origin: e.srv.URL, ctype: "application/json"}
}

func (e *env) bot(t *testing.T, token string) *client {
	return &client{t: t, e: e, http: &http.Client{}, bearer: token, ctype: "application/json"}
}

type reply struct {
	status int
	body   map[string]any
	raw    string
	header http.Header
}

func (c *client) do(method, path string, body any) reply {
	c.t.Helper()
	var r io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		r = bytes.NewReader(b)
	}
	req, _ := http.NewRequest(method, c.e.srv.URL+path, r)
	if c.ctype != "" && body != nil {
		req.Header.Set("Content-Type", c.ctype)
	}
	req.Header.Set("X-Forwarded-For", "203.0.113.7")
	if c.origin != "" {
		req.Header.Set("Origin", c.origin)
	}
	if c.bearer != "" {
		req.Header.Set("Authorization", "Bearer "+c.bearer)
	}
	res, err := c.http.Do(req)
	if err != nil {
		c.t.Fatal(err)
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	out := reply{status: res.StatusCode, raw: string(raw), header: res.Header}
	_ = json.Unmarshal(raw, &out.body)
	return out
}

func (r reply) errCode() string {
	e, _ := r.body["error"].(map[string]any)
	code, _ := e["code"].(string)
	return code
}

func expect(t *testing.T, r reply, status int, code string) {
	t.Helper()
	if r.status != status || (code != "" && r.errCode() != code) {
		t.Fatalf("got %d %s, want %d %s", r.status, r.raw, status, code)
	}
}

func register(t *testing.T, c *client, name string) reply {
	t.Helper()
	r := c.do("POST", "/api/v1/auth/register", map[string]string{"username": name, "password": "long-password",
		"passwordConfirm": "long-password"})
	expect(t, r, 201, "")
	return r
}

func TestGuestFlow(t *testing.T) {
	e := newEnv(t)
	c := e.browser(t)
	r := c.do("POST", "/api/v1/auth/guest", map[string]string{"locale": "pl"})
	expect(t, r, 201, "")
	if !strings.Contains(r.header.Get("Set-Cookie"), "lh_session=") {
		t.Fatalf("no cookie: %v", r.header)
	}
	me := c.do("GET", "/api/v1/me", nil)
	if me.body["isGuest"] != true || me.body["locale"] != "pl" {
		t.Fatalf("me %s", me.raw)
	}
	nick := me.body["username"].(string)
	expect(t, c.do("POST", "/api/v1/auth/login", map[string]string{"username": nick, "password": ""}), 401, "badCredentials")
	expect(t, c.do("PATCH", "/api/v1/me", map[string]any{"username": "Gosc_Nowy"}), 200, "")
	r = c.do("POST", "/api/v1/auth/claim", map[string]string{"username": "Player_A", "password": "long-password", "passwordConfirm": "long-password"})
	expect(t, r, 200, "")
	if r.body["isGuest"] != false || r.body["username"] != "Player_A" {
		t.Fatalf("claim %s", r.raw)
	}
	expect(t, c.do("POST", "/api/v1/auth/login", map[string]string{"username": "Player_A", "password": "long-password"}), 200, "")
}

func TestInstantGamesRoute(t *testing.T) {
	e := newEnv(t)
	a, b := e.browser(t), e.browser(t)
	expect(t, a.do("GET", "/api/v1/instant/games", nil), 401, "unauthenticated")
	expect(t, a.do("POST", "/api/v1/auth/guest", map[string]string{}), 201, "")
	expect(t, b.do("POST", "/api/v1/auth/guest", map[string]string{}), 201, "")
	wid := a.do("POST", "/api/v1/instant/play", map[string]string{}).body["worldId"]
	list := b.do("GET", "/api/v1/instant/games", nil)
	games, _ := list.body["games"].([]any)
	if list.status != 200 || len(games) != 1 || list.body["canStart"] != true {
		t.Fatalf("list %d %s", list.status, list.raw)
	}
	r := b.do("POST", "/api/v1/instant/play", map[string]any{"worldId": wid})
	expect(t, r, 200, "")
	if r.body["worldId"] != wid {
		t.Fatalf("joined %s, wanted world %v", r.raw, wid)
	}
	other := a.do("POST", "/api/v1/instant/play", map[string]any{"worldId": 999999})
	if other.status != 400 {
		t.Fatalf("unknown world: %d %s", other.status, other.raw)
	}
}

func TestPlayRoute(t *testing.T) {
	e := newEnv(t)
	c := e.browser(t)
	expect(t, c.do("POST", "/api/v1/instant/play", map[string]string{}), 401, "unauthenticated")
	expect(t, c.do("POST", "/api/v1/auth/guest", map[string]string{}), 201, "")
	r := c.do("POST", "/api/v1/instant/play", map[string]string{})
	expect(t, r, 200, "")
	wid, ok := r.body["worldId"].(float64)
	if !ok || wid == 0 {
		t.Fatalf("play %s", r.raw)
	}
	again := c.do("POST", "/api/v1/instant/play", map[string]string{})
	if again.body["worldId"] != r.body["worldId"] {
		t.Fatalf("second play %s", again.raw)
	}
	st := c.do("GET", "/api/v1/worlds/"+strconv.Itoa(int(wid))+"/state", nil)
	inst, _ := st.body["instant"].(map[string]any)
	if st.status != 200 || inst == nil || inst["endsAt"] == nil {
		t.Fatalf("state %d %s", st.status, st.raw)
	}
}

func TestPublicRoutesNeedNoSession(t *testing.T) {
	e := newEnv(t)
	c := e.browser(t)
	r := c.do("GET", "/api/v1/public/stats", nil)
	expect(t, r, 200, "")
	if r.body["visible"] != false || r.body["allTime"] == nil {
		t.Fatalf("stats %s", r.raw)
	}
	l := c.do("GET", "/api/v1/public/live", nil)
	expect(t, l, 200, "")
	if _, ok := l.body["events"]; !ok {
		t.Fatalf("live %s", l.raw)
	}
	ws := c.do("GET", "/api/v1/public/worlds", nil)
	expect(t, ws, 200, "")
	if ws.body["running"] == nil || ws.body["long"] == nil || ws.body["finished"] == nil {
		t.Fatalf("worlds %s", ws.raw)
	}
	expect(t, c.do("GET", "/api/v1/public/worlds/abc", nil), 400, "")
	expect(t, c.do("GET", "/api/v1/public/worlds/999", nil), 404, "notFound")
	expect(t, c.do("GET", "/api/v1/public/worlds/999/villages", nil), 404, "notFound")
	expect(t, c.do("GET", "/api/v1/public/games/999", nil), 404, "notFound")
	expect(t, c.do("GET", "/api/v1/public/profiles/nobody", nil), 404, "notFound")
	p := c.do("GET", "/api/v1/public/profiles/admin", nil)
	expect(t, p, 200, "")
	if p.body["username"] != "admin" || p.body["games"] == nil {
		t.Fatalf("profile %s", p.raw)
	}
}

func TestReportRoutes(t *testing.T) {
	e := newEnv(t)
	visitor := e.browser(t)
	report := map[string]any{"kind": "bug", "text": "The map stays black", "screen": "map", "userAgent": "Firefox"}
	first := visitor.do("POST", "/api/v1/reports", report)
	expect(t, first, 201, "")
	id, ok := first.body["id"].(float64)
	if !ok || id == 0 {
		t.Fatalf("report %s", first.raw)
	}
	for range 4 {
		expect(t, visitor.do("POST", "/api/v1/reports", report), 201, "")
	}
	expect(t, visitor.do("POST", "/api/v1/reports", report), 429, "tooManyReports")
	visitor.origin = ""
	expect(t, visitor.do("POST", "/api/v1/reports", report), 403, "csrf")

	player := e.browser(t)
	register(t, player, "ala")
	expect(t, player.do("GET", "/api/v1/admin/reports", nil), 403, "forbidden")
	admin := e.browser(t)
	expect(t, admin.do("POST", "/api/v1/auth/login", map[string]string{"username": "admin", "password": "admin-password"}), 200, "")
	list := admin.do("GET", "/api/v1/admin/reports", nil)
	expect(t, list, 200, "")
	if !strings.Contains(list.raw, `"text":"The map stays black"`) || !strings.Contains(list.raw, `"status":"new"`) {
		t.Fatalf("list %s", list.raw)
	}
	path := "/api/v1/admin/reports/" + strconv.Itoa(int(id))
	expect(t, admin.do("PATCH", path, map[string]string{"status": "done"}), 204, "")
	expect(t, admin.do("PATCH", path, map[string]string{"status": "gone"}), 400, "statusInvalid")
	expect(t, admin.do("PATCH", "/api/v1/admin/reports/999", map[string]string{"status": "read"}), 404, "notFound")
}

func TestPublicTimelineIsGzipJSON(t *testing.T) {
	e := newEnv(t)
	c := e.browser(t)
	expect(t, c.do("POST", "/api/v1/auth/guest", map[string]string{"locale": "en"}), 201, "")
	play := c.do("POST", "/api/v1/instant/play", map[string]string{})
	expect(t, play, 200, "")
	wid := int64(play.body["worldId"].(float64))
	e.clock.ms.Add(61 * time.Minute.Milliseconds())
	if _, _, err := e.svc.ProcessDue(context.Background(), wid, 0); err != nil {
		t.Fatal(err)
	}
	res, err := (&http.Transport{DisableCompression: true}).RoundTrip(httptest.NewRequest("GET", e.srv.URL+"/api/v1/public/games/"+strconv.FormatInt(wid, 10)+"/timeline", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 || res.Header.Get("Content-Encoding") != "gzip" || res.Header.Get("Content-Type") != "application/json" {
		t.Fatalf("timeline %d %v", res.StatusCode, res.Header)
	}
	zr, err := gzip.NewReader(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := io.ReadAll(zr)
	if !strings.Contains(string(raw), `"v":1`) {
		t.Fatalf("body %.200s", raw)
	}
}

func TestMePatchLocale(t *testing.T) {
	e := newEnv(t)
	c := e.browser(t)
	register(t, c, "ala")
	if me := c.do("GET", "/api/v1/me", nil); me.body["locale"] != "en" || me.body["isGuest"] != false || me.body["tutorialDone"] != false {
		t.Fatalf("me %s", me.raw)
	}
	r := c.do("PATCH", "/api/v1/me", map[string]any{"locale": "pl", "tutorialDone": true})
	expect(t, r, 200, "")
	if me := c.do("GET", "/api/v1/me", nil); me.body["locale"] != "pl" || me.body["tutorialDone"] != true {
		t.Fatalf("after patch %s", me.raw)
	}
	expect(t, c.do("PATCH", "/api/v1/me", map[string]any{"locale": "de"}), 400, "locale")
}

func TestCookieFlow(t *testing.T) {
	e := newEnv(t)
	c := e.browser(t)
	r := register(t, c, "ala")
	acct := r.body["account"].(map[string]any)
	if acct["username"] != "ala" || acct["isAdmin"] != false || r.body["token"] == "" {
		t.Fatalf("register %s", r.raw)
	}
	cookie := r.header.Get("Set-Cookie")
	for _, attr := range []string{"lh_session=", "HttpOnly", "SameSite=Lax", "Path=/", "Max-Age=2592000"} {
		if !strings.Contains(cookie, attr) {
			t.Errorf("cookie %q lacks %s", cookie, attr)
		}
	}
	if strings.Contains(cookie, "Secure") {
		t.Error("Secure without CookieSecure")
	}
	expect(t, c.do("GET", "/api/v1/me", nil), 200, "")
	expect(t, c.do("POST", "/api/v1/auth/logout", map[string]string{}), 204, "")
	expect(t, c.do("GET", "/api/v1/me", nil), 401, "unauthenticated")
	r = c.do("POST", "/api/v1/auth/login", map[string]string{"username": "ala", "password": "long-password"})
	expect(t, r, 200, "")
	if me := c.do("GET", "/api/v1/me", nil); me.body["username"] != "ala" {
		t.Fatalf("me %s", me.raw)
	}
	lobby := c.do("GET", "/api/v1/worlds", nil)
	if _, ok := lobby.body["open"]; lobby.status != 200 || !ok {
		t.Fatalf("lobby %s", lobby.raw)
	}
}

func TestBearerFlow(t *testing.T) {
	e := newEnv(t)
	r := register(t, e.browser(t), "ala")
	token := r.body["token"].(string)
	b := e.bot(t, token)
	if me := b.do("GET", "/api/v1/me", nil); me.status != 200 || me.body["username"] != "ala" {
		t.Fatalf("me %s", me.raw)
	}
	expect(t, b.do("POST", "/api/v1/auth/logout", nil), 204, "")
	expect(t, b.do("GET", "/api/v1/me", nil), 401, "unauthenticated")
}

func TestCSRF(t *testing.T) {
	e := newEnv(t)
	c := e.browser(t)
	register(t, c, "ala")
	c.ctype = "text/plain"
	expect(t, c.do("POST", "/api/v1/auth/logout", map[string]string{}), 403, "csrf")
	c.ctype, c.origin = "application/json", "https://evil.example"
	expect(t, c.do("POST", "/api/v1/auth/logout", map[string]string{}), 403, "csrf")
	c.origin = ""
	expect(t, c.do("POST", "/api/v1/auth/logout", map[string]string{}), 403, "csrf")
	c.origin = e.srv.URL
	expect(t, c.do("GET", "/api/v1/me", nil), 200, "")
}

// world creates a world as the admin and returns its id with a browser of a joined player.
func world(t *testing.T, e *env, adminC *client) string {
	t.Helper()
	cfg := gametest.TestConfig()
	r := adminC.do("POST", "/api/v1/admin/worlds", map[string]any{"name": "Café World", "settings": cfg})
	expect(t, r, 201, "")
	return jsonNumber(r.body["id"])
}

func jsonNumber(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

func admin(t *testing.T, e *env) *client {
	t.Helper()
	c := e.browser(t)
	expect(t, c.do("POST", "/api/v1/auth/login", map[string]string{"username": "admin", "password": "admin-password"}), 200, "")
	return c
}

func TestErrorMapping(t *testing.T) {
	e := newEnv(t)
	adm := admin(t, e)
	wid := world(t, e, adm)
	ala, bob := e.browser(t), e.browser(t)
	register(t, ala, "ala")
	register(t, bob, "bob")
	expect(t, ala.do("POST", "/api/v1/worlds/"+wid+"/join", map[string]string{"quadrant": "ne"}), 200, "")
	expect(t, bob.do("POST", "/api/v1/worlds/"+wid+"/join", map[string]string{}), 200, "")
	st := ala.do("GET", "/api/v1/worlds/"+wid+"/state", nil)
	expect(t, st, 200, "")
	alaVillage := ""
	for id := range st.body["villages"].(map[string]any) {
		alaVillage = id
	}

	r := bob.do("POST", "/api/v1/worlds/"+wid+"/orders/build", map[string]string{"villageId": alaVillage, "building": "wood"})
	expect(t, r, 409, "blocked")
	if bl, _ := r.body["error"].(map[string]any)["blockers"].([]any); len(bl) == 0 {
		t.Fatalf("no blockers: %s", r.raw)
	}
	expect(t, ala.do("POST", "/api/v1/worlds/"+wid+"/orders/claim-quest", map[string]string{"questId": "nie_ma", "villageId": alaVillage}), 409, "unknownQuest")
	r = e.browser(t).do("POST", "/api/v1/auth/register", map[string]string{"username": "ewa", "password": "short",
		"passwordConfirm": "short"})
	expect(t, r, 400, "passwordShort")
	if r.body["error"].(map[string]any)["field"] != "password" {
		t.Fatalf("field %s", r.raw)
	}
	expect(t, ala.do("POST", "/api/v1/worlds/"+wid+"/orders/teleport", map[string]string{}), 400, "unknown")
	stranger := e.browser(t)
	expect(t, stranger.do("POST", "/api/v1/auth/login", map[string]string{"username": "ala", "password": "wrong"}), 401, "badCredentials")
	expect(t, stranger.do("GET", "/api/v1/worlds", nil), 401, "unauthenticated")
	expect(t, ala.do("GET", "/api/v1/admin/accounts", nil), 403, "forbidden")
	carol := e.browser(t)
	register(t, carol, "carol")
	expect(t, carol.do("GET", "/api/v1/worlds/"+wid+"/state", nil), 403, "notMember")
	expect(t, carol.do("GET", "/api/v1/worlds/abc/state", nil), 400, "invalid")
	expect(t, adm.do("POST", "/api/v1/admin/worlds/"+wid+"/pause", map[string]string{}), 204, "")
	expect(t, ala.do("POST", "/api/v1/worlds/"+wid+"/orders/build", map[string]string{"villageId": alaVillage, "building": "wood"}), 423, "worldPaused")
	expect(t, adm.do("POST", "/api/v1/admin/worlds/"+wid+"/resume", map[string]string{}), 204, "")
	expect(t, ala.do("POST", "/api/v1/worlds/"+wid+"/orders/build", map[string]string{"villageId": alaVillage, "building": "wood"}), 200, "")
	for range 5 {
		stranger.do("POST", "/api/v1/auth/login", map[string]string{"username": "bob", "password": "wrong"})
	}
	expect(t, stranger.do("POST", "/api/v1/auth/login", map[string]string{"username": "bob", "password": "long-password"}), 429, "tooManyAttempts")
}

func TestGameViews(t *testing.T) {
	e := newEnv(t)
	adm := admin(t, e)
	wid := world(t, e, adm)
	ala := e.browser(t)
	register(t, ala, "ala")
	expect(t, ala.do("POST", "/api/v1/worlds/"+wid+"/join", map[string]string{}), 200, "")
	base := "/api/v1/worlds/" + wid
	me := ala.do("GET", base+"/state", nil).body["me"].(string)
	home := ""
	for id := range ala.do("GET", base+"/state", nil).body["villages"].(map[string]any) {
		home = id
	}
	for _, path := range []string{"/reports?limit=10", "/map?x0=90&y0=90&x1=110&y1=110", "/ranking?tab=points",
		"/players/" + me, "/news?limit=5", "/farm-targets?village=" + home} {
		if r := ala.do("GET", base+path, nil); r.status != 200 || !strings.HasPrefix(strings.TrimSpace(r.raw), "[") && !strings.HasPrefix(r.raw, "{") {
			t.Errorf("%s: %d %s", path, r.status, r.raw)
		}
	}
	expect(t, ala.do("GET", base+"/map?x0=0&y0=0&x1=150&y1=1", nil), 400, "tooLarge")
	expect(t, ala.do("GET", base+"/map?x0=a", nil), 400, "invalid")
	expect(t, ala.do("GET", base+"/players/h77", nil), 404, "notFound")
	expect(t, ala.do("POST", base+"/leave", map[string]string{}), 204, "")
	expect(t, ala.do("GET", base+"/state", nil), 403, "notMember")
}

func TestAdminRoutes(t *testing.T) {
	e := newEnv(t)
	adm := admin(t, e)
	expect(t, adm.do("GET", "/api/v1/admin/codes", nil), 404, "notFound")
	ala := e.browser(t)
	register(t, ala, "ala")
	accounts := adm.do("GET", "/api/v1/admin/accounts", nil)
	if accounts.status != 200 || !strings.Contains(accounts.raw, `"username":"ala"`) {
		t.Fatalf("accounts %s", accounts.raw)
	}
	var alaID string
	var rows []map[string]any
	_ = json.Unmarshal([]byte(accounts.raw), &rows)
	for _, row := range rows {
		if row["username"] == "ala" {
			alaID = jsonNumber(row["id"])
		}
	}
	expect(t, adm.do("POST", "/api/v1/admin/accounts/"+alaID+"/password", map[string]string{"password": "new-password-1"}), 204, "")
	expect(t, ala.do("GET", "/api/v1/me", nil), 401, "unauthenticated")

	wid := world(t, e, adm)
	expect(t, adm.do("PATCH", "/api/v1/admin/worlds/"+wid, map[string]any{"settings": map[string]any{"worldSpeed": 3}, "status": "closed"}), 204, "")
	expect(t, adm.do("PATCH", "/api/v1/admin/worlds/"+wid, map[string]any{"settings": map[string]any{"mapSize": 400}}), 400, "fixedAtCreation")
	worlds := adm.do("GET", "/api/v1/admin/worlds", nil)
	if worlds.status != 200 || !strings.Contains(worlds.raw, `"status":"closed"`) || !strings.Contains(worlds.raw, `"worldSpeed":3`) {
		t.Fatalf("worlds %s", worlds.raw)
	}
}

func TestRegistrationIsOpenButLimitedPerAddress(t *testing.T) {
	e := newEnvWith(t, app.Config{})
	for _, name := range []string{"ala", "bob", "carol"} {
		register(t, e.browser(t), name)
	}
	r := e.browser(t).do("POST", "/api/v1/auth/register", map[string]string{"username": "dave", "password": "long-password",
		"passwordConfirm": "long-password"})
	expect(t, r, 429, "tooManyRegistrations")
}

func TestHostRoutes(t *testing.T) {
	e := newEnvWith(t, app.Config{RegistrationsPerHour: 1000, PlayerMaxWorlds: 1})
	ala := e.browser(t)
	register(t, ala, "ala")
	created := ala.do("POST", "/api/v1/hosted", map[string]any{"name": "Friday", "seats": 6})
	expect(t, created, 201, "")
	if created.body["status"] != "waiting" || created.body["seats"] != float64(6) || len(created.body["inviteCode"].(string)) != 20 {
		t.Fatalf("created %s", created.raw)
	}
	expect(t, ala.do("POST", "/api/v1/hosted", map[string]any{"name": "Again"}), 400, "alreadyHosting")
	ola := e.browser(t)
	register(t, ola, "ola")
	expect(t, ola.do("POST", "/api/v1/hosted", map[string]any{"name": "Mine"}), 503, "serverFull")
	guest := e.browser(t)
	expect(t, guest.do("POST", "/api/v1/auth/guest", map[string]string{"locale": "en"}), 201, "")
	expect(t, guest.do("POST", "/api/v1/hosted", map[string]any{"name": "Guest"}), 403, "forbidden")

	expect(t, ala.do("GET", "/api/v1/admin/host-settings", nil), 403, "forbidden")
	expect(t, ala.do("PUT", "/api/v1/admin/host-settings", map[string]int{"maxSeats": 4, "maxBots": 1}), 403, "forbidden")
	adm := e.browser(t)
	expect(t, adm.do("POST", "/api/v1/auth/login", map[string]string{"username": "admin", "password": "admin-password"}), 200, "")
	if got := adm.do("GET", "/api/v1/admin/host-settings", nil); got.status != 200 || got.body["maxSeats"] != float64(10) || got.body["maxBots"] != float64(5) {
		t.Fatalf("settings %s", got.raw)
	}
	expect(t, adm.do("PUT", "/api/v1/admin/host-settings", map[string]int{"maxSeats": 50, "maxBots": 1}), 400, "")
	if got := adm.do("PUT", "/api/v1/admin/host-settings", map[string]int{"maxSeats": 4, "maxBots": 1}); got.status != 200 || got.body["maxSeats"] != float64(4) {
		t.Fatalf("saved %s", got.raw)
	}
	if lobby := ala.do("GET", "/api/v1/worlds", nil); lobby.status != 200 || !strings.Contains(lobby.raw, `"hosted":{`) ||
		!strings.Contains(lobby.raw, `"canHost":true`) {
		t.Fatalf("lobby %s", lobby.raw)
	}
}

func TestInviteRoutes(t *testing.T) {
	e := newEnv(t)
	host := e.browser(t)
	register(t, host, "ala")
	created := host.do("POST", "/api/v1/hosted", map[string]any{"name": "Friday", "seats": 2})
	expect(t, created, 201, "")
	code := created.body["inviteCode"].(string)
	anon := e.browser(t)
	expect(t, anon.do("GET", "/api/v1/public/invites/nosuchcodenosuchcode", nil), 404, "notFound")
	info := anon.do("GET", "/api/v1/public/invites/"+code, nil)
	if info.status != 200 || info.body["name"] != "Friday" || info.body["host"] != "ala" || info.body["open"] != true {
		t.Fatalf("info %s", info.raw)
	}
	expect(t, anon.do("POST", "/api/v1/invites/"+code+"/join", map[string]any{}), 401, "unauthenticated")
	for i := range 2 {
		guest := e.browser(t)
		expect(t, guest.do("POST", "/api/v1/auth/guest", map[string]string{"locale": "en"}), 201, "")
		expect(t, guest.do("POST", "/api/v1/invites/nosuchcodenosuchcode/join", map[string]any{}), 404, "notFound")
		joined := guest.do("POST", "/api/v1/invites/"+code+"/join", map[string]any{})
		if joined.status != 200 || joined.body["worldId"] != created.body["worldId"] {
			t.Fatalf("guest %d: %s", i, joined.raw)
		}
	}
	third := e.browser(t)
	register(t, third, "ola")
	expect(t, third.do("POST", "/api/v1/invites/"+code+"/join", map[string]any{}), 400, "worldFull")
	if watch := host.do("POST", "/api/v1/invites/"+code+"/join", map[string]any{}); watch.status != 200 || watch.body["watch"] != true {
		t.Fatalf("host %s", watch.raw)
	}
}

func TestHostControlRoutes(t *testing.T) {
	e := newEnv(t)
	host := e.browser(t)
	register(t, host, "ala")
	created := host.do("POST", "/api/v1/hosted", map[string]any{"name": "Friday"})
	expect(t, created, 201, "")
	code := created.body["inviteCode"].(string)
	base := "/api/v1/worlds/" + jsonNumber(created.body["worldId"]) + "/host"
	guest := e.browser(t)
	expect(t, guest.do("POST", "/api/v1/auth/guest", map[string]string{"locale": "en"}), 201, "")
	expect(t, guest.do("POST", "/api/v1/invites/"+code+"/join", map[string]any{}), 200, "")
	expect(t, guest.do("POST", base+"/start", map[string]any{}), 403, "forbidden")
	for _, step := range []struct{ action, status string }{{"start", "running"}, {"pause", "paused"}, {"resume", "running"}} {
		if r := host.do("POST", base+"/"+step.action, map[string]any{}); r.status != 200 || r.body["status"] != step.status {
			t.Fatalf("%s %s", step.action, r.raw)
		}
	}
	expect(t, host.do("POST", base+"/resume", map[string]any{}), 400, "notPaused")
	invite := host.do("POST", base+"/invite", map[string]any{})
	if invite.status != 200 || invite.body["inviteCode"] == code || len(invite.body["inviteCode"].(string)) != 20 {
		t.Fatalf("invite %s", invite.raw)
	}
	expect(t, e.browser(t).do("GET", "/api/v1/public/invites/"+code, nil), 404, "notFound")
	if r := host.do("PATCH", base, map[string]any{"registeredOnly": true}); r.status != 200 || r.body["registeredOnly"] != true {
		t.Fatalf("join rule %s", r.raw)
	}
	lobby := host.do("GET", "/api/v1/worlds", nil)
	players := lobby.body["hosted"].(map[string]any)["players"].([]any)
	kick := host.do("POST", base+"/kick", map[string]any{"playerId": players[0].(map[string]any)["id"]})
	if kick.status != 200 || kick.body["humans"] != float64(0) {
		t.Fatalf("kick %s", kick.raw)
	}
	expect(t, host.do("POST", base+"/kick", map[string]any{"playerId": "bot_0"}), 400, "notAPlayer")
	expect(t, host.do("POST", base+"/dance", map[string]any{}), 404, "notFound")
	if r := host.do("POST", base+"/end", map[string]any{}); r.status != 200 || r.body["status"] != "ended" || r.body["resultsCode"] == "" {
		t.Fatalf("end %s", r.raw)
	}
}

func TestPublicListsSkipPlayerWorlds(t *testing.T) {
	e := newEnv(t)
	host := e.browser(t)
	register(t, host, "ala")
	created := host.do("POST", "/api/v1/hosted", map[string]any{"name": "Secret Friday"})
	expect(t, created, 201, "")
	wid := jsonNumber(created.body["worldId"])
	invite := created.body["inviteCode"].(string)
	ola := e.browser(t)
	register(t, ola, "ola")
	expect(t, ola.do("POST", "/api/v1/invites/"+invite+"/join", map[string]any{}), 200, "")
	expect(t, host.do("POST", "/api/v1/worlds/"+wid+"/host/start", map[string]any{}), 200, "")
	ended := host.do("POST", "/api/v1/worlds/"+wid+"/host/end", map[string]any{})
	expect(t, ended, 200, "")
	results := ended.body["resultsCode"].(string)
	if err := e.svc.ArchiveEnded(context.Background()); err != nil {
		t.Fatal(err)
	}
	anon := e.browser(t)
	for _, path := range []string{"/api/v1/public/worlds", "/api/v1/public/live", "/api/v1/public/stats"} {
		r := anon.do("GET", path, nil)
		if r.status != 200 || strings.Contains(r.raw, "Secret Friday") || strings.Contains(r.raw, invite) || strings.Contains(r.raw, results) {
			t.Fatalf("%s: %s", path, r.raw)
		}
	}
	if g := anon.do("GET", "/api/v1/public/games/"+results, nil); g.status != 200 || g.body["name"] != "Secret Friday" {
		t.Fatalf("by code %s", g.raw)
	}
	expect(t, anon.do("GET", "/api/v1/public/games/"+wid, nil), 404, "notFound")
	expect(t, anon.do("GET", "/api/v1/public/games/"+wid+"/timeline", nil), 404, "notFound")
	if tl := anon.do("GET", "/api/v1/public/games/"+results+"/timeline", nil); tl.status != 200 {
		t.Fatalf("timeline by code %d %s", tl.status, tl.raw)
	}
	own := ola.do("GET", "/api/v1/public/profiles/ola", nil)
	other := anon.do("GET", "/api/v1/public/profiles/ola", nil)
	if own.status != 200 || other.status != 200 || !strings.Contains(own.raw, results) || strings.Contains(other.raw, results) ||
		strings.Contains(other.raw, "Secret Friday") {
		t.Fatalf("profile own %s, others %s", own.raw, other.raw)
	}
}

func TestWatchRoutes(t *testing.T) {
	e := newEnv(t)
	host := e.browser(t)
	register(t, host, "ala")
	created := host.do("POST", "/api/v1/hosted", map[string]any{"name": "Friday", "bots": 2})
	expect(t, created, 201, "")
	base := "/api/v1/worlds/" + jsonNumber(created.body["worldId"]) + "/watch"
	guest := e.browser(t)
	expect(t, guest.do("POST", "/api/v1/auth/guest", map[string]string{"locale": "en"}), 201, "")
	expect(t, guest.do("POST", "/api/v1/invites/"+created.body["inviteCode"].(string)+"/join", map[string]any{}), 200, "")
	watch := host.do("GET", base, nil)
	if watch.status != 200 || watch.body["worldName"] != "Friday" || len(watch.body["villages"].([]any)) == 0 || len(watch.body["ranking"].([]any)) != 3 {
		t.Fatalf("watch %s", watch.raw)
	}
	expect(t, guest.do("GET", base, nil), 403, "forbidden")
	village := watch.body["villages"].([]any)[0].(map[string]any)["id"].(string)
	if v := host.do("GET", base+"/villages/"+village, nil); v.status != 200 || v.body["id"] != village || v.body["buildings"] == nil {
		t.Fatalf("village %s", v.raw)
	}
	expect(t, host.do("GET", base+"/villages/nonsense", nil), 404, "notFound")
	expect(t, guest.do("GET", base+"/villages/"+village, nil), 403, "forbidden")
}
