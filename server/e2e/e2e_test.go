//go:build e2e

package e2e

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"librehold/internal/game"
)

func TestMain(m *testing.M) { setup(m) }

func newServer(t *testing.T) *server {
	t.Helper()
	return startServer(t, newDB(t), time.Now().UnixMilli())
}

// fast is a quick world for playing: speed 100, marches at 10×, a small map, no beginner protection.
func fast(bots int) func(cfg *game.WorldConfig) {
	return func(cfg *game.WorldConfig) {
		cfg.WorldSpeed, cfg.UnitSpeed, cfg.MapSize, cfg.BotCount, cfg.BeginnerProtectionDays = 100, 10, 100, bots, 0
	}
}

// DoD 1
func TestFreshStartAdminWorld(t *testing.T) {
	s := newServer(t)
	a := s.admin(t)
	wid := a.world(func(cfg *game.WorldConfig) { cfg.BotCount = 100 })
	var all []struct {
		ID       int64            `json:"id"`
		Status   string           `json:"status"`
		Bots     int              `json:"bots"`
		Settings game.WorldConfig `json:"settings"`
	}
	a.ok("GET", "/api/v1/admin/worlds", nil, &all)
	worlds := all[:0]
	for _, w := range all {
		if !game.IsInstant(&w.Settings) {
			worlds = append(worlds, w)
		}
	}
	if len(worlds) != 1 || worlds[0].ID != wid || worlds[0].Bots != 100 || worlds[0].Status != "open" {
		t.Fatalf("worlds %+v", worlds)
	}
	expect404 := (&client{t: t, s: s}).do("GET", "/api/v1/admin/codes", nil)
	if expect404.status != 401 && expect404.status != 404 {
		t.Fatalf("access codes are still served: %d", expect404.status)
	}
}

// DoD 2
func TestFriendRegistersJoinsSpawnsProtected(t *testing.T) {
	s := newServer(t)
	a := s.admin(t)
	wid := a.world(func(cfg *game.WorldConfig) { cfg.BotCount = 20 })
	ala := s.register(t, "ala")
	s.register(t, "bob")
	ala.join(wid)
	st := ala.state(wid)
	if len(st.Villages) != 1 || st.Player.ProtectedUntil <= st.Now || st.Player.Name != "ala" {
		t.Fatalf("after joining: %d villages, protected until %d at %d", len(st.Villages), st.Player.ProtectedUntil, st.Now)
	}
}

// DoD 4
func TestTwoDevicesLiveSync(t *testing.T) {
	s := newServer(t)
	a := s.admin(t)
	wid := a.world(fast(0))
	s.register(t, "ala").join(wid)
	phone, laptop := s.login(t, "ala", password), s.login(t, "ala", password)
	sock := laptop.live(wid)
	if m := <-sock.msgs; m["type"] != "hello" {
		t.Fatalf("first message %v", m)
	}
	v := phone.home(wid)
	phone.mustOrder(wid, "build", map[string]string{"villageId": v.ID, "building": "wood"})
	deadline := time.After(time.Second)
	for {
		select {
		case m, ok := <-sock.msgs:
			if !ok {
				t.Fatal("socket closed")
			}
			if m["type"] == "update" && strings.Contains(strings.Join(anyStrings(m["topics"]), ","), "state") {
				return
			}
		case <-deadline:
			t.Fatal("no state update on the other device within 1 s")
		}
	}
}

func anyStrings(v any) []string {
	var out []string
	for _, x := range v.([]any) {
		out = append(out, x.(string))
	}
	return out
}

// DoD 6
func TestGate(t *testing.T) {
	s := newServer(t)
	get := func(path, accept string) (int, string, string) {
		req, _ := http.NewRequest("GET", s.url+path, nil)
		if accept != "" {
			req.Header.Set("Accept", accept)
		}
		res, err := (&http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}).Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		b, _ := io.ReadAll(res.Body)
		return res.StatusCode, string(b), res.Header.Get("Location")
	}
	status, body, _ := get("/", "text/html")
	if status != 200 || !strings.Contains(body, "<title>") {
		t.Fatalf("/: %d %.200s", status, body)
	}
	if status, _, loc := get("/game/", "text/html"); status != 302 || loc != "/" {
		t.Fatalf("/game/: %d → %s", status, loc)
	}
	js, _ := filepath.Glob(filepath.Join(webDir(), "game", "assets", "*.js"))
	if len(js) == 0 {
		t.Fatal("no game script in dist/game/assets")
	}
	if status, _, _ := get("/game/assets/"+filepath.Base(js[0]), "*/*"); status != 401 {
		t.Fatalf("game script without a session: %d", status)
	}
	if status, _, _ := get("/account.html", "text/html"); status != 404 {
		t.Fatalf("/account.html: %d", status)
	}
	files, _ := filepath.Glob(filepath.Join(webDir(), "account", "assets", "*"))
	for _, f := range files {
		if strings.HasSuffix(f, ".br") || strings.HasSuffix(f, ".gz") {
			continue
		}
		rel := strings.TrimPrefix(f, filepath.Join(webDir(), "account"))
		status, body, _ := get(rel, "")
		if status != 200 {
			t.Fatalf("%s: %d", rel, status)
		}
		if disk, _ := os.ReadFile(f); string(disk) != body {
			t.Fatalf("%s is not the built file", rel)
		}
	}
}

// DoD 8
func TestForeignOrdersRejected(t *testing.T) {
	s := newServer(t)
	a := s.admin(t)
	wid := a.world(fast(0))
	ala, bob := s.register(t, "ala"), s.register(t, "bob")
	ala.join(wid)
	bob.join(wid)
	bv := bob.home(wid)
	boost(t, s.db, wid, bv.ID, army)
	for name, body := range map[string]map[string]any{
		"build":            {"villageId": bv.ID, "building": "wood"},
		"send":             {"originId": bv.ID, "x": bv.X + 1, "y": bv.Y, "units": map[string]int{"axe": 10}, "type": "attack"},
		"withdraw-support": {"hostId": bv.ID, "homeId": bv.ID},
		"mint":             {"villageId": bv.ID},
		"recruit":          {"villageId": bv.ID, "unit": "spear", "count": 1},
		"rename-village":   {"villageId": bv.ID, "name": "Mine"},
	} {
		if r := ala.order(wid, name, body); r.status != 409 || r.code() != "blocked" {
			t.Errorf("%s on bob's village: %d %s", name, r.status, r.body)
		}
	}
	if got := bob.home(wid); got.Name != bv.Name || got.Units[game.UAxe] != 1000 {
		t.Fatalf("bob's village changed: %s, axes %d", got.Name, got.Units[game.UAxe])
	}
}
