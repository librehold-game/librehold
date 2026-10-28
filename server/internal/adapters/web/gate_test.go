package web_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"librehold/internal/adapters/web"
)

const routesJSON = `[
 {"id":"home","en":"/","pl":"/pl/","index":"yes","build":"prerender"},
 {"id":"privacy","en":"/privacy","pl":"/pl/prywatnosc","index":"yes","build":"prerender"},
 {"id":"unit","en":"/units/:unit","pl":"/pl/jednostki/:unit","index":"yes","build":"prerender"},
 {"id":"world","en":"/worlds/:id","pl":"/pl/swiaty/:id","index":"yes","build":"dynamic"},
 {"id":"game","en":"/games/:id","pl":"/pl/gry/:id","index":"yes","build":"dynamic"},
 {"id":"player","en":"/players/:name","pl":"/pl/gracze/:name","index":"registered","build":"dynamic"},
 {"id":"lobby","en":"/lobby","pl":"/pl/lobby","index":"no","build":"app"}
]`

const pagesJSON = `[
 {"route":"home","en":"/","pl":"/pl/"},
 {"route":"privacy","en":"/privacy","pl":"/pl/prywatnosc"},
 {"route":"unit","en":"/units/spearman","pl":"/pl/jednostki/pikinier"}
]`

const headTextJSON = `{"en":{"app.name":"LibreHold","head.imageAlt":"A village"},"pl":{"app.name":"LibreHold","head.imageAlt":"Village"}}`

func page(lang, path string) string {
	return `<html lang="` + lang + `"><head><link rel="canonical" href="__SITE_URL__` + path + `"></head><body>` + path + `</body></html>`
}

// site writes a fake build: the account site with its route files and pages, and the game.
func site(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for path, body := range map[string]string{
		"account/routes.json":                      routesJSON,
		"account/pages.json":                       pagesJSON,
		"account/head-text.json":                   headTextJSON,
		"account/index.html":                       page("en", "/"),
		"account/pl/index.html":                    page("pl", "/pl/"),
		"account/privacy/index.html":               page("en", "/privacy"),
		"account/pl/prywatnosc/index.html":         page("pl", "/pl/prywatnosc"),
		"account/units/spearman/index.html":        page("en", "/units/spearman"),
		"account/pl/jednostki/pikinier/index.html": page("pl", "/pl/jednostki/pikinier"),
		"account/404.html":                         `<html lang="en"><body>missing __SITE_URL__</body></html>`,
		"account/pl/404.html":                      `<html lang="pl"><body>brak</body></html>`,
		"account/account.html":                     `<html lang="__LANG__"><head><!--dynamic-head--></head><body><div id="root"></div></body></html>`,
		"account/assets/a.js":                      "console.log('account')",
		"account/og/home.png":                      "PNG",
		"game/game.html":                           "<html>gra</html>",
		"game/assets/game.js":                      "console.log('gra')",
		"game/graphic/main.png":                    "PNG",
		"secret.txt":                               "not for anyone",
	} {
		p := filepath.Join(dir, path)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

type fakeHeads struct{}

func (fakeHeads) Head(_ context.Context, route, _ string, params map[string]string) (web.PageHead, bool, error) {
	if route != "game" {
		return web.PageHead{}, false, nil
	}
	switch params["id"] {
	case "5":
		return web.PageHead{Title: "Game #5 — x", Description: "A game"}, true, nil
	case "7":
		return web.PageHead{}, false, errors.New("boom")
	case "8":
		return web.PageHead{Title: "Hidden", Description: "d", NoIndex: true}, true, nil
	}
	return web.PageHead{}, false, nil
}

func authAs(k web.AuthKind) func(*http.Request) web.AuthKind {
	return func(*http.Request) web.AuthKind { return k }
}

func gate(t *testing.T, k web.AuthKind, publicURL string) http.Handler {
	t.Helper()
	return web.New(site(t), authAs(k), fakeHeads{}, publicURL)
}

type request struct {
	method, path, accept, cookie, host, encoding string
}

func do(t *testing.T, h http.Handler, q request) (*http.Response, string) {
	t.Helper()
	if q.method == "" {
		q.method = "GET"
	}
	req := httptest.NewRequest(q.method, q.path, nil)
	if q.accept != "" {
		req.Header.Set("Accept", q.accept)
	}
	if q.cookie != "" {
		req.Header.Set("Cookie", q.cookie)
	}
	if q.host != "" {
		req.Host = q.host
	}
	if q.encoding != "" {
		req.Header.Set("Accept-Encoding", q.encoding)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	res := rec.Result()
	body, _ := io.ReadAll(res.Body)
	return res, string(body)
}

func get(t *testing.T, h http.Handler, path string) (*http.Response, string) {
	t.Helper()
	return do(t, h, request{path: path, accept: "text/html"})
}

func TestPrerenderedPagesGetTheSiteAddress(t *testing.T) {
	res, body := get(t, gate(t, web.AuthNone, "https://example.com"), "/privacy")
	if res.StatusCode != 200 || !strings.Contains(body, `href="https://example.com/privacy"`) || strings.Contains(body, "__SITE_URL__") {
		t.Fatalf("public url: %d %q", res.StatusCode, body)
	}
	if ct := res.Header.Get("Content-Type"); ct != "text/html; charset=utf-8" || res.Header.Get("Cache-Control") != "public, no-cache" ||
		res.Header.Get("Vary") != "Cookie" {
		t.Fatalf("headers %v", res.Header)
	}
	_, body = do(t, gate(t, web.AuthNone, ""), request{path: "/privacy", host: "librehold.test"})
	if !strings.Contains(body, `href="http://librehold.test/privacy"`) {
		t.Fatalf("request host: %q", body)
	}
}

func TestUnknownAddressIs404Page(t *testing.T) {
	h := gate(t, web.AuthNone, "https://example.com")
	if res, body := get(t, h, "/nope"); res.StatusCode != 404 || !strings.Contains(body, "missing https://example.com") {
		t.Fatalf("/nope: %d %q", res.StatusCode, body)
	}
	if res, body := get(t, h, "/pl/nope"); res.StatusCode != 404 || !strings.Contains(body, "brak") {
		t.Fatalf("/pl/nope: %d %q", res.StatusCode, body)
	}
	for _, p := range []string{"/index.html", "/routes.json", "/account.html", "/assets/none.js", "/head-text.json", "/og/none.png",
		"/secret.txt", "/../secret.txt", "/assets/../../secret.txt", "/game.html"} {
		if res, body := get(t, h, p); res.StatusCode != 404 || strings.Contains(body, "not for anyone") {
			t.Errorf("%s: %d %q", p, res.StatusCode, body)
		}
	}
	res, body := do(t, h, request{method: "HEAD", path: "/nope"})
	if res.StatusCode != 404 || body != "" || !strings.HasPrefix(res.Header.Get("Content-Type"), "text/html") {
		t.Fatalf("HEAD /nope: %d %q %v", res.StatusCode, body, res.Header)
	}
}

func TestBothHomesAreServed(t *testing.T) {
	h := gate(t, web.AuthNone, "")
	for path, want := range map[string]string{"/": `<html lang="en">`, "/pl/": `<html lang="pl">`} {
		if res, body := get(t, h, path); res.StatusCode != 200 || !strings.Contains(body, want) {
			t.Errorf("%s: %d %q", path, res.StatusCode, body)
		}
	}
}

func TestPrerenderedRouteWithoutFileIs404(t *testing.T) {
	h := gate(t, web.AuthNone, "")
	for _, p := range []string{"/units/archer", "/pl/jednostki/spearman", "/units/..", "/units/%2e%2e"} {
		if res, _ := get(t, h, p); res.StatusCode != 404 {
			t.Errorf("%s: %d", p, res.StatusCode)
		}
	}
	if res, body := get(t, h, "/pl/jednostki/pikinier"); res.StatusCode != 200 || !strings.Contains(body, "pikinier") {
		t.Fatalf("existing unit: %d %q", res.StatusCode, body)
	}
}

func TestDynamicPageGetsServerHead(t *testing.T) {
	h := gate(t, web.AuthNone, "https://example.com")
	res, body := get(t, h, "/games/5")
	for _, want := range []string{`<html lang="en">`, `<title>Game #5 — x</title>`, `<link rel="canonical" href="https://example.com/games/5">`,
		`<link rel="alternate" hreflang="pl" href="https://example.com/pl/gry/5">`,
		`<meta property="og:image" content="https://example.com/og/home.png">`, `<meta property="og:image:alt" content="A village">`,
		`<meta property="og:site_name" content="LibreHold">`} {
		if !strings.Contains(body, want) {
			t.Errorf("missing %s", want)
		}
	}
	if res.StatusCode != 200 || strings.Contains(body, "noindex") || strings.Contains(body, "<!--dynamic-head-->") {
		t.Fatalf("/games/5: %d %q", res.StatusCode, body)
	}
	_, body = get(t, h, "/pl/gry/5")
	if !strings.Contains(body, `<html lang="pl">`) || !strings.Contains(body, `<meta property="og:locale" content="pl_PL">`) {
		t.Fatalf("/pl/gry/5: %q", body)
	}
	if res, body := get(t, h, "/games/6"); res.StatusCode != 404 || !strings.Contains(body, "missing") {
		t.Fatalf("not found: %d %q", res.StatusCode, body)
	}
	if res, _ := get(t, h, "/games/7"); res.StatusCode != 500 {
		t.Fatalf("source error: %d", res.StatusCode)
	}
	if _, body := get(t, h, "/games/8"); !strings.Contains(body, `<meta name="robots" content="noindex">`) {
		t.Fatalf("no index: %q", body)
	}
}

func TestAppPagesServeTheShellNoIndex(t *testing.T) {
	res, body := get(t, gate(t, web.AuthNone, ""), "/lobby")
	if res.StatusCode != 200 || !strings.Contains(body, `<meta name="robots" content="noindex">`) || !strings.Contains(body, `lang="en"`) ||
		!strings.Contains(body, "<title>LibreHold</title>") || !strings.Contains(body, `<div id="root"></div>`) {
		t.Fatalf("/lobby: %d %q", res.StatusCode, body)
	}
}

func TestLanguageCookieRedirectsToTwin(t *testing.T) {
	h := gate(t, web.AuthNone, "")
	for path, want := range map[string]string{
		"/privacy": "/pl/prywatnosc", "/units/spearman": "/pl/jednostki/pikinier", "/games/5?a=1": "/pl/gry/5?a=1", "/players/a%20b": "/pl/gracze/a%20b",
	} {
		res, _ := do(t, h, request{path: path, cookie: "lang=pl"})
		if res.StatusCode != 302 || res.Header.Get("Location") != want {
			t.Errorf("%s: %d %s", path, res.StatusCode, res.Header.Get("Location"))
		}
	}
	if res, _ := do(t, h, request{path: "/pl/", cookie: "lang=en"}); res.StatusCode != 302 || res.Header.Get("Location") != "/" {
		t.Errorf("/pl/ with en: %d %s", res.StatusCode, res.Header.Get("Location"))
	}
	if res, _ := do(t, h, request{path: "/privacy", cookie: "lang=xx"}); res.StatusCode != 200 {
		t.Errorf("unknown language: %d", res.StatusCode)
	}
}

func TestLanguageCookieLeavesNonPagesAlone(t *testing.T) {
	h := gate(t, web.AuthRegistered, "")
	for path, status := range map[string]int{"/game/": 200, "/assets/a.js": 200, "/og/home.png": 200, "/nope": 404} {
		res, _ := do(t, h, request{path: path, cookie: "lang=pl"})
		if res.StatusCode != status || res.Header.Get("Location") != "" {
			t.Errorf("%s: %d %s", path, res.StatusCode, res.Header.Get("Location"))
		}
	}
}

func TestRegisteredHomeGoesToLobby(t *testing.T) {
	h := gate(t, web.AuthRegistered, "")
	for path, want := range map[string]string{"/": "/lobby", "/pl/": "/pl/lobby"} {
		if res, _ := get(t, h, path); res.StatusCode != 302 || res.Header.Get("Location") != want {
			t.Errorf("%s: %d %s", path, res.StatusCode, res.Header.Get("Location"))
		}
	}
	if res, _ := get(t, gate(t, web.AuthGuest, ""), "/"); res.StatusCode != 200 {
		t.Errorf("guest: %d", res.StatusCode)
	}
}

func TestRegisteredHomeFollowsLanguageCookieInOneHop(t *testing.T) {
	res, _ := do(t, gate(t, web.AuthRegistered, ""), request{path: "/", cookie: "lang=pl"})
	if res.StatusCode != 302 || res.Header.Get("Location") != "/pl/lobby" {
		t.Fatalf("%d %s", res.StatusCode, res.Header.Get("Location"))
	}
}

func TestTrailingSlashRedirects(t *testing.T) {
	h := gate(t, web.AuthNone, "")
	for path, want := range map[string]string{"/privacy/?x=1": "/privacy?x=1", "/pl": "/pl/", "/pl?x=1": "/pl/?x=1"} {
		if res, _ := get(t, h, path); res.StatusCode != 301 || res.Header.Get("Location") != want {
			t.Errorf("%s: %d %s", path, res.StatusCode, res.Header.Get("Location"))
		}
	}
}

func TestHeadMatchesGet(t *testing.T) {
	h := gate(t, web.AuthNone, "")
	_, getBody := get(t, h, "/privacy")
	res, body := do(t, h, request{method: "HEAD", path: "/privacy"})
	if res.StatusCode != 200 || body != "" || res.Header.Get("Content-Length") != strconv.Itoa(len(getBody)) {
		t.Fatalf("HEAD: %d %q %s vs %d", res.StatusCode, body, res.Header.Get("Content-Length"), len(getBody))
	}
}

func TestStaticFilesAreCached(t *testing.T) {
	h := gate(t, web.AuthNone, "")
	if res, body := get(t, h, "/assets/a.js"); res.StatusCode != 200 || body != "console.log('account')" ||
		res.Header.Get("Cache-Control") != "public, max-age=31536000, immutable" {
		t.Fatalf("asset: %d %q %v", res.StatusCode, body, res.Header)
	}
	if res, _ := get(t, h, "/og/home.png"); res.StatusCode != 200 || res.Header.Get("Cache-Control") != "public, max-age=86400" {
		t.Fatalf("og: %d %v", res.StatusCode, res.Header)
	}
}

func TestMissingBuildAnswersPlain404(t *testing.T) {
	h := web.New(t.TempDir(), authAs(web.AuthNone), fakeHeads{}, "")
	if res, _ := get(t, h, "/"); res.StatusCode != 404 {
		t.Fatalf("/: %d", res.StatusCode)
	}
}

func TestGameAssetsNeedSession(t *testing.T) {
	h := gate(t, web.AuthNone, "")
	if res, body := do(t, h, request{path: "/game/assets/game.js", accept: "*/*"}); res.StatusCode != 401 || strings.Contains(body, "gra") {
		t.Fatalf("asset: %d %q", res.StatusCode, body)
	}
	if res, _ := do(t, h, request{path: "/game/graphic/main.png", accept: "image/png"}); res.StatusCode != 401 {
		t.Fatalf("graphic: %d", res.StatusCode)
	}
	res, _ := do(t, h, request{path: "/game/", accept: "text/html,application/xhtml+xml"})
	if res.StatusCode != 302 || res.Header.Get("Location") != "/" {
		t.Fatalf("page: %d %s", res.StatusCode, res.Header.Get("Location"))
	}
	res, body := do(t, gate(t, web.AuthGuest, ""), request{path: "/game/assets/game.js", accept: "*/*"})
	if res.StatusCode != 200 || body != "console.log('gra')" || !strings.Contains(res.Header.Get("Cache-Control"), "private") {
		t.Fatalf("authed asset: %d %q %s", res.StatusCode, body, res.Header.Get("Cache-Control"))
	}
}

func TestGameSPAFallback(t *testing.T) {
	h := gate(t, web.AuthRegistered, "")
	for _, path := range []string{"/game", "/game/", "/game/world/3/village", "/game/game.html"} {
		if res, body := get(t, h, path); res.StatusCode != 200 || body != "<html>gra</html>" {
			t.Errorf("%s: %d %q", path, res.StatusCode, body)
		}
	}
	if res, _ := do(t, h, request{path: "/game/assets/missing.js", accept: "*/*"}); res.StatusCode != 404 {
		t.Errorf("missing asset: %d", res.StatusCode)
	}
	if res, body := get(t, h, "/game/../secret.txt"); strings.Contains(body, "not for anyone") {
		t.Errorf("traversal: %d", res.StatusCode)
	}
}
