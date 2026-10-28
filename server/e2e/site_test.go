//go:build e2e

package e2e

import (
	"encoding/xml"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

type page struct {
	status   int
	body     string
	location string
	header   http.Header
}

// fetch requests a page as a browser would, without following redirects; token becomes the session cookie.
func (s *server) fetch(t *testing.T, method, path, token string, headers ...string) page {
	t.Helper()
	req, err := http.NewRequest(method, s.url+path, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Accept", "text/html")
	if token != "" {
		req.AddCookie(&http.Cookie{Name: "lh_session", Value: token})
	}
	for i := 0; i+1 < len(headers); i += 2 {
		if headers[i] == "Cookie" {
			req.Header.Add("Cookie", headers[i+1])
		} else {
			req.Header.Set(headers[i], headers[i+1])
		}
	}
	tr := &http.Transport{DisableCompression: true}
	res, err := (&http.Client{Transport: tr, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}).Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	return page{status: res.StatusCode, body: string(b), location: res.Header.Get("Location"), header: res.Header}
}

func TestPublicSite(t *testing.T) {
	s := newServer(t)
	home := s.fetch(t, "GET", "/", "")
	for _, want := range []string{`<html lang="en"`, `<link rel="canonical" href="` + s.url + `/">`,
		`<meta property="og:image" content="` + s.url + `/og/home.png">`, `<link rel="alternate" hreflang="pl" href="` + s.url + `/pl/">`} {
		if !strings.Contains(home.body, want) {
			t.Errorf("/ misses %s", want)
		}
	}
	if home.status != 200 {
		t.Fatalf("/: %d", home.status)
	}
	if img := s.fetch(t, "GET", "/og/home.png", ""); img.status != 200 || img.header.Get("Content-Type") != "image/png" {
		t.Fatalf("og image: %d %s", img.status, img.header.Get("Content-Type"))
	}
	if pl := s.fetch(t, "GET", "/pl/", ""); pl.status != 200 || !strings.Contains(pl.body, `<html lang="pl"`) {
		t.Fatalf("/pl/: %d", pl.status)
	}
	if p := s.fetch(t, "GET", "/nope", ""); p.status != 404 || !strings.Contains(p.body, "<html") {
		t.Fatalf("/nope: %d %.100s", p.status, p.body)
	}
	if p := s.fetch(t, "HEAD", "/nope", ""); p.status != 404 || p.body != "" {
		t.Fatalf("HEAD /nope: %d %q", p.status, p.body)
	}

	robots := s.fetch(t, "GET", "/robots.txt", "")
	for _, want := range []string{"Disallow: /game/", "Disallow: /api/", "Sitemap: " + s.url + "/sitemap.xml"} {
		if !strings.Contains(robots.body, want) {
			t.Errorf("robots.txt misses %q", want)
		}
	}
	var sm struct {
		URLs []struct {
			Loc string `xml:"loc"`
		} `xml:"url"`
	}
	if err := xml.Unmarshal([]byte(s.fetch(t, "GET", "/sitemap.xml", "").body), &sm); err != nil {
		t.Fatal(err)
	}
	listed := map[string]bool{}
	for _, u := range sm.URLs {
		listed[strings.TrimPrefix(u.Loc, s.url)] = true
	}
	if !listed["/"] || !listed["/pl/"] || !listed["/privacy"] || listed["/lobby"] {
		t.Fatalf("sitemap %v", listed)
	}

	g := s.guest(t)
	wid, _ := g.play(t)
	id := strconv.FormatInt(wid, 10)
	if p := s.fetch(t, "GET", "/games/"+id, ""); p.status != 200 || !strings.Contains(p.body, "<title>Game #") || !strings.Contains(p.body, "live now") {
		t.Fatalf("running game: %d %.300s", p.status, p.body)
	}
	if p := s.fetch(t, "GET", "/pl/gry/"+id, ""); p.status != 200 || !strings.Contains(p.body, "<title>Gra #") {
		t.Fatalf("running game in Polish: %d", p.status)
	}
	if p := s.fetch(t, "GET", "/games/999999", ""); p.status != 404 {
		t.Fatalf("missing game: %d", p.status)
	}

	ala := s.register(t, "ala")
	if p := s.fetch(t, "GET", "/players/ala", ""); p.status != 200 || !strings.Contains(p.body, "<title>ala — ") {
		t.Fatalf("/players/ala: %d %.300s", p.status, p.body)
	}
	if p := s.fetch(t, "GET", "/players/nobody", ""); p.status != 404 {
		t.Fatalf("/players/nobody: %d", p.status)
	}
	if p := s.fetch(t, "GET", "/", ala.token); p.status != 302 || p.location != "/lobby" {
		t.Fatalf("registered home: %d %s", p.status, p.location)
	}
	if p := s.fetch(t, "GET", "/", ala.token, "Cookie", "lang=pl"); p.status != 302 || p.location != "/pl/lobby" {
		t.Fatalf("registered home in Polish: %d %s", p.status, p.location)
	}
	if p := s.fetch(t, "GET", "/privacy", "", "Cookie", "lang=pl"); p.status != 302 || p.location != "/pl/prywatnosc" {
		t.Fatalf("language cookie: %d %s", p.status, p.location)
	}

	scripts, _ := filepath.Glob(filepath.Join(webDir(), "account", "assets", "account-*.js"))
	if len(scripts) == 0 {
		t.Fatal("no account script in dist/account/assets")
	}
	p := s.fetch(t, "GET", "/assets/"+filepath.Base(scripts[0]), "", "Accept-Encoding", "br")
	if p.status != 200 || p.header.Get("Content-Encoding") != "br" {
		t.Fatalf("account script: %d %v", p.status, p.header)
	}
}

func TestPublicURLSetting(t *testing.T) {
	s := startServerAt(t, newDB(t), time.Now().UnixMilli(), timeScale, "LH_PUBLIC_URL=https://librehold.example")
	if p := s.fetch(t, "GET", "/", ""); !strings.Contains(p.body, `<link rel="canonical" href="https://librehold.example/">`) {
		t.Fatalf("canonical: %.400s", p.body)
	}
}
