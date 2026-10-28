package web

import (
	"bytes"
	"context"
	"encoding/xml"
	"log/slog"
	"net/http"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"librehold/internal/app"
)

type SitemapEntry = app.SitemapEntry

// SitemapSource lists the dynamic pages worth indexing.
type SitemapSource interface {
	SitemapEntries(ctx context.Context) ([]SitemapEntry, error)
}

const (
	sitemapCacheFor = 5 * time.Minute
	// sitemapOrigins bounds the cache: without a public URL the origin comes from the request's Host header.
	sitemapOrigins = 8
)

// Robots keeps crawlers out of the game, the API and the pages behind a login.
func Robots(publicURL string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := "User-agent: *\nAllow: /\nDisallow: /game/\nDisallow: /api/\nDisallow: /lobby\nDisallow: /pl/lobby\nDisallow: /admin\n" +
			"Disallow: /pl/admin\n\nSitemap: " + origin(r, publicURL) + "/sitemap.xml\n"
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("Cache-Control", "public, max-age=3600")
		_, _ = w.Write([]byte(body))
	})
}

type sitemap struct {
	site      *site
	src       SitemapSource
	publicURL string

	mu    sync.Mutex
	cache map[string]cachedSitemap // by origin
}

type cachedSitemap struct {
	at   time.Time
	body []byte
}

// NewSitemap lists every indexed page in both languages, each with its language twins.
func NewSitemap(dir string, src SitemapSource, publicURL string) http.Handler {
	s, err := loadSite(filepath.Join(dir, "account"))
	if err != nil {
		slog.Error("sitemap: account build files missing", "err", err)
	}
	return &sitemap{site: s, src: src, publicURL: publicURL, cache: map[string]cachedSitemap{}}
}

func (m *sitemap) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if m.site == nil {
		http.NotFound(w, r)
		return
	}
	o := origin(r, m.publicURL)
	m.mu.Lock()
	c, ok := m.cache[o]
	m.mu.Unlock()
	if !ok || time.Since(c.at) >= sitemapCacheFor {
		body, err := m.render(r.Context(), o)
		if err != nil {
			slog.Error("sitemap", "err", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		c = cachedSitemap{at: time.Now(), body: body}
		m.mu.Lock()
		if len(m.cache) >= sitemapOrigins {
			clear(m.cache)
		}
		m.cache[o] = c
		m.mu.Unlock()
	}
	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=300")
	_, _ = w.Write(c.body)
}

type xmlLink struct {
	Rel      string `xml:"rel,attr"`
	Hreflang string `xml:"hreflang,attr"`
	Href     string `xml:"href,attr"`
}

type xmlURL struct {
	Loc     string    `xml:"loc"`
	Links   []xmlLink `xml:"xhtml:link"`
	LastMod string    `xml:"lastmod,omitempty"`
}

func (m *sitemap) render(ctx context.Context, o string) ([]byte, error) {
	entries, err := m.src.SitemapEntries(ctx)
	if err != nil {
		return nil, err
	}
	var urls []xmlURL
	add := func(en, pl string, lastMod time.Time) {
		links := []xmlLink{{"alternate", "en", o + en}, {"alternate", "pl", o + pl}, {"alternate", "x-default", o + en}}
		mod := ""
		if !lastMod.IsZero() {
			mod = lastMod.UTC().Format("2006-01-02")
		}
		urls = append(urls, xmlURL{Loc: o + en, Links: links, LastMod: mod}, xmlURL{Loc: o + pl, Links: links, LastMod: mod})
	}
	index := map[string]string{}
	for _, r := range m.site.routes {
		index[r.def.ID] = r.def.Index
	}
	var pages []string
	for en := range m.site.twins {
		if r, locale, _, ok := m.site.match(en); ok && locale == "en" && r.Index == "yes" {
			pages = append(pages, en)
		}
	}
	sort.Strings(pages)
	for _, en := range pages {
		add(en, m.site.twins[en], time.Time{})
	}
	for _, e := range entries {
		if _, known := index[e.Route]; !known {
			continue
		}
		add(m.site.pathOf(e.Route, "en", e.Params), m.site.pathOf(e.Route, "pl", e.Params), e.LastMod)
	}
	var buf bytes.Buffer
	buf.WriteString(xml.Header)
	buf.WriteString(`<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9" xmlns:xhtml="http://www.w3.org/1999/xhtml">` + "\n")
	enc := xml.NewEncoder(&buf)
	enc.Indent("", " ")
	for _, u := range urls {
		if err := enc.EncodeElement(u, xml.StartElement{Name: xml.Name{Local: "url"}}); err != nil {
			return nil, err
		}
	}
	if err := enc.Flush(); err != nil {
		return nil, err
	}
	buf.WriteString("\n</urlset>\n")
	return buf.Bytes(), nil
}
