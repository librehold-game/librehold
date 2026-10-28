package web_test

import (
	"context"
	"encoding/xml"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"librehold/internal/adapters/web"
)

func TestRobots(t *testing.T) {
	res, body := get(t, web.Robots("https://example.com"), "/robots.txt")
	want := "User-agent: *\nAllow: /\nDisallow: /game/\nDisallow: /api/\nDisallow: /lobby\nDisallow: /pl/lobby\nDisallow: /admin\n" +
		"Disallow: /pl/admin\n\nSitemap: https://example.com/sitemap.xml\n"
	if res.StatusCode != 200 || body != want || res.Header.Get("Content-Type") != "text/plain; charset=utf-8" ||
		res.Header.Get("Cache-Control") != "public, max-age=3600" {
		t.Fatalf("%d %q %v", res.StatusCode, body, res.Header)
	}
}

type fakeSitemap struct {
	calls int
	err   error
}

func (f *fakeSitemap) SitemapEntries(context.Context) ([]web.SitemapEntry, error) {
	f.calls++
	return []web.SitemapEntry{
		{Route: "world", Params: map[string]string{"id": "3"}},
		{Route: "game", Params: map[string]string{"id": "5"}, LastMod: time.Date(2026, 10, 21, 23, 30, 0, 0, time.UTC)},
		{Route: "unknown", Params: map[string]string{"id": "9"}},
	}, f.err
}

type sitemapXML struct {
	URLs []struct {
		Loc     string `xml:"loc"`
		LastMod string `xml:"lastmod"`
		Links   []struct {
			Rel      string `xml:"rel,attr"`
			Hreflang string `xml:"hreflang,attr"`
			Href     string `xml:"href,attr"`
		} `xml:"link"`
	} `xml:"url"`
}

func TestSitemapListsIndexedPagesWithTwins(t *testing.T) {
	h := web.NewSitemap(site(t), &fakeSitemap{}, "https://example.com")
	res, body := get(t, h, "/sitemap.xml")
	if res.StatusCode != 200 || !strings.HasPrefix(res.Header.Get("Content-Type"), "application/xml") ||
		!strings.Contains(body, `xmlns="http://www.sitemaps.org/schemas/sitemap/0.9"`) || !strings.Contains(body, `xmlns:xhtml="http://www.w3.org/1999/xhtml"`) {
		t.Fatalf("%d %v %q", res.StatusCode, res.Header, body)
	}
	var doc sitemapXML
	if err := xml.Unmarshal([]byte(body), &doc); err != nil {
		t.Fatal(err)
	}
	urls := map[string]int{}
	for i, u := range doc.URLs {
		urls[strings.TrimPrefix(u.Loc, "https://example.com")] = i
	}
	for _, p := range []string{"/", "/pl/", "/privacy", "/pl/prywatnosc", "/units/spearman", "/pl/jednostki/pikinier", "/worlds/3", "/pl/swiaty/3",
		"/games/5", "/pl/gry/5"} {
		i, ok := urls[p]
		if !ok {
			t.Errorf("missing %s", p)
			continue
		}
		langs := map[string]string{}
		for _, l := range doc.URLs[i].Links {
			if l.Rel == "alternate" {
				langs[l.Hreflang] = l.Href
			}
		}
		if len(langs) != 3 || langs["x-default"] != langs["en"] {
			t.Errorf("%s alternates %v", p, langs)
		}
	}
	if u := doc.URLs[urls["/"]]; len(u.Links) == 0 {
		t.Errorf("home %+v", u)
	}
	if got := doc.URLs[urls["/pl/gry/5"]].LastMod; got != "2026-10-21" {
		t.Errorf("game lastmod %q", got)
	}
	for _, p := range []string{"/lobby", "/pl/lobby", "/games/9", "/unknown/9"} {
		if _, ok := urls[p]; ok {
			t.Errorf("%s listed", p)
		}
	}
}

func TestSitemapIsCached(t *testing.T) {
	src := &fakeSitemap{}
	h := web.NewSitemap(site(t), src, "https://example.com")
	get(t, h, "/sitemap.xml")
	get(t, h, "/sitemap.xml")
	if src.calls != 1 {
		t.Fatalf("%d source calls", src.calls)
	}
}

func TestSitemapSourceErrorIs500(t *testing.T) {
	src := &fakeSitemap{err: errors.New("db down")}
	h := web.NewSitemap(site(t), src, "https://example.com")
	if res, _ := get(t, h, "/sitemap.xml"); res.StatusCode != 500 {
		t.Fatalf("%d", res.StatusCode)
	}
	src.err = nil
	if res, _ := get(t, h, "/sitemap.xml"); res.StatusCode != 200 || src.calls != 2 {
		t.Fatalf("after recovery: %d, %d calls", res.StatusCode, src.calls)
	}
}

func TestPrecompressedAssets(t *testing.T) {
	dir := site(t)
	for name, body := range map[string]string{"account/assets/a.js.br": "BROTLI", "account/assets/a.js.gz": "GZIP",
		"game/assets/game.js.br": "GAME BROTLI"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	h := web.New(dir, authAs(web.AuthGuest), fakeHeads{}, "")
	asset := func(path, enc string) (*http.Response, string) {
		return do(t, h, request{path: path, encoding: enc})
	}
	res, body := asset("/assets/a.js", "gzip, deflate, br")
	if body != "BROTLI" || res.Header.Get("Content-Encoding") != "br" || !strings.Contains(res.Header.Get("Content-Type"), "javascript") ||
		res.Header.Get("Vary") != "Accept-Encoding" || res.Header.Get("Cache-Control") != "public, max-age=31536000, immutable" {
		t.Fatalf("br: %q %v", body, res.Header)
	}
	if res, body := asset("/assets/a.js", "br;q=0, gzip"); body != "GZIP" || res.Header.Get("Content-Encoding") != "gzip" {
		t.Fatalf("gzip: %q %v", body, res.Header)
	}
	if res, body := asset("/assets/a.js", ""); body != "console.log('account')" || res.Header.Get("Content-Encoding") != "" ||
		res.Header.Get("Vary") != "Accept-Encoding" {
		t.Fatalf("plain: %q %v", body, res.Header)
	}
	for _, p := range []string{"/assets/a.js.br", "/assets/a.js.gz", "/game/assets/game.js.br"} {
		if res, _ := asset(p, "br"); res.StatusCode != 404 {
			t.Errorf("%s: %d", p, res.StatusCode)
		}
	}
	if res, body := asset("/game/assets/game.js", "br"); body != "GAME BROTLI" || res.Header.Get("Content-Encoding") != "br" {
		t.Fatalf("game br: %q %v", body, res.Header)
	}
}
