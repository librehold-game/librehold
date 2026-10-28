// Package web serves the two web builds: the public account site to anyone, the game only to a session.
package web

import (
	"bytes"
	"html"
	"io"
	"io/fs"
	"log/slog"
	"mime"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
)

type gate struct {
	site      *site // nil when the account build is missing or broken
	game      fs.FS
	auth      func(*http.Request) AuthKind
	heads     HeadSource
	publicURL string
}

// New serves the account build in dir/account at its real addresses and the game in dir/game at /game/ to a
// session. Prerendered pages come from their files, dynamic pages get their head from heads, app pages the shell;
// anything else is the 404 page with status 404.
func New(dir string, auth func(*http.Request) AuthKind, heads HeadSource, publicURL string) http.Handler {
	g := &gate{game: os.DirFS(filepath.Join(dir, "game")), auth: auth, heads: heads, publicURL: publicURL}
	s, err := loadSite(filepath.Join(dir, "account"))
	if err != nil {
		slog.Error("account site: build files missing, every page answers 404", "dir", dir, "err", err)
	} else {
		g.site = s
	}
	return g
}

func (g *gate) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	p := path.Clean("/" + r.URL.Path)
	if p == "/game" || strings.HasPrefix(p, "/game/") {
		g.serveGame(w, r, p)
		return
	}
	if g.site == nil {
		http.NotFound(w, r)
		return
	}
	g.serveAccount(w, r, p)
}

func (g *gate) serveGame(w http.ResponseWriter, r *http.Request, p string) {
	if g.auth(r) == AuthNone {
		if strings.Contains(r.Header.Get("Accept"), "text/html") {
			http.Redirect(w, r, "/", http.StatusFound)
			return
		}
		http.Error(w, "unauthenticated", http.StatusUnauthorized)
		return
	}
	name := strings.TrimPrefix(strings.TrimPrefix(p, "/game"), "/")
	if strings.HasPrefix(name, "assets/") {
		if !sendAsset(w, r, g.game, name, "private, max-age=31536000, immutable") {
			http.NotFound(w, r)
		}
		return
	}
	f, err := openFile(g.game, name)
	if err != nil {
		if path.Ext(name) != "" && name != "game.html" {
			http.NotFound(w, r)
			return
		}
		name = "game.html"
		if f, err = openFile(g.game, name); err != nil {
			http.NotFound(w, r)
			return
		}
	}
	defer f.Close()
	sendFile(w, r, f, name, "private, no-cache")
}

func (g *gate) serveAccount(w http.ResponseWriter, r *http.Request, p string) {
	raw := r.URL.EscapedPath()
	query := ""
	if r.URL.RawQuery != "" {
		query = "?" + r.URL.RawQuery
	}
	switch {
	case raw == "/pl":
		http.Redirect(w, r, "/pl/"+query, http.StatusMovedPermanently)
		return
	case len(raw) > 1 && strings.HasSuffix(raw, "/") && raw != "/pl/":
		if trimmed := strings.TrimRight(raw, "/"); trimmed != "" && !strings.HasPrefix(trimmed, "//") {
			http.Redirect(w, r, trimmed+query, http.StatusMovedPermanently)
			return
		}
	}
	if strings.HasPrefix(p, "/assets/") || strings.HasPrefix(p, "/og/") {
		g.serveStatic(w, r, p)
		return
	}
	route, locale, params, ok := g.site.match(raw)
	if !ok {
		g.notFound(w, r, p)
		return
	}
	effective := locale
	if c, err := r.Cookie("lang"); err == nil && (c.Value == "en" || c.Value == "pl") {
		effective = c.Value
	}
	if route.ID == "home" && g.auth(r) == AuthRegistered {
		http.Redirect(w, r, g.site.pathOf("lobby", effective, nil)+query, http.StatusFound)
		return
	}
	if effective != locale {
		http.Redirect(w, r, g.site.twin(raw, route, locale, params)+query, http.StatusFound)
		return
	}
	switch route.Build {
	case "prerender":
		own := g.site.pathOf(route.ID, locale, params)
		file := strings.TrimPrefix(strings.TrimSuffix(own, "/")+"/index.html", "/")
		body, err := g.readPage(file, r)
		if err != nil {
			g.notFound(w, r, p)
			return
		}
		sendHTML(w, r, http.StatusOK, body)
	case "dynamic":
		h, found, err := g.heads.Head(r.Context(), route.ID, locale, params)
		if err != nil {
			slog.Error("page head", "path", raw, "err", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		if !found {
			g.notFound(w, r, p)
			return
		}
		g.sendShell(w, r, locale, g.site.headTags(h, raw, route, locale, params, g.origin(r)))
	default:
		g.sendShell(w, r, locale, g.site.appHead(locale))
	}
}

// serveStatic sends the build's content-hashed assets (cached a year) and preview images (cached a day).
func (g *gate) serveStatic(w http.ResponseWriter, r *http.Request, p string) {
	name := strings.TrimPrefix(p, "/")
	if strings.HasPrefix(name, "assets/") {
		if !sendAsset(w, r, g.site.files, name, "public, max-age=31536000, immutable") {
			g.notFound(w, r, p)
		}
		return
	}
	f, err := openFile(g.site.files, name)
	if err != nil {
		g.notFound(w, r, p)
		return
	}
	defer f.Close()
	sendFile(w, r, f, name, "public, max-age=86400")
}

// sendAsset sends an asset, as the build's brotli or gzip copy when the client takes one; the copies are never
// served under their own names. False, with nothing written, when there is no such asset.
func sendAsset(w http.ResponseWriter, r *http.Request, root fs.FS, name, cache string) bool {
	if strings.HasSuffix(name, ".br") || strings.HasSuffix(name, ".gz") {
		return false
	}
	f, err := openFile(root, name)
	if err != nil {
		return false
	}
	defer f.Close()
	w.Header().Set("Vary", "Accept-Encoding")
	for _, enc := range []struct{ coding, ext string }{{"br", ".br"}, {"gzip", ".gz"}} {
		if !acceptsEncoding(r, enc.coding) {
			continue
		}
		packed, err := openFile(root, name+enc.ext)
		if err != nil {
			continue
		}
		defer packed.Close()
		if ct := mime.TypeByExtension(path.Ext(name)); ct != "" {
			w.Header().Set("Content-Type", ct)
		}
		w.Header().Set("Content-Encoding", enc.coding)
		sendFile(w, r, packed, name, cache)
		return true
	}
	sendFile(w, r, f, name, cache)
	return true
}

// acceptsEncoding reports whether Accept-Encoding names the coding with a q above zero.
func acceptsEncoding(r *http.Request, coding string) bool {
	for _, part := range strings.Split(r.Header.Get("Accept-Encoding"), ",") {
		name, params, _ := strings.Cut(strings.TrimSpace(part), ";")
		if !strings.EqualFold(strings.TrimSpace(name), coding) {
			continue
		}
		q, found := strings.CutPrefix(strings.TrimSpace(params), "q=")
		if !found {
			return true
		}
		v, err := strconv.ParseFloat(q, 64)
		return err == nil && v > 0
	}
	return false
}

func (g *gate) origin(r *http.Request) string {
	return html.EscapeString(origin(r, g.publicURL))
}

// readPage reads an HTML file of the account build with the site's address filled in.
func (g *gate) readPage(name string, r *http.Request) ([]byte, error) {
	f, err := openFile(g.site.files, name)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	b, err := io.ReadAll(f)
	if err != nil {
		return nil, err
	}
	return bytes.ReplaceAll(b, []byte("__SITE_URL__"), []byte(g.origin(r))), nil
}

func (g *gate) notFound(w http.ResponseWriter, r *http.Request, p string) {
	name := "404.html"
	if p == "/pl" || strings.HasPrefix(p, "/pl/") {
		name = "pl/404.html"
	}
	body, err := g.readPage(name, r)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	sendHTML(w, r, http.StatusNotFound, body)
}

// sendShell sends account.html with the page's language and head.
func (g *gate) sendShell(w http.ResponseWriter, r *http.Request, locale, head string) {
	body, err := g.readPage("account.html", r)
	if err != nil {
		g.notFound(w, r, r.URL.Path)
		return
	}
	body = bytes.Replace(body, []byte("__LANG__"), []byte(locale), 1)
	body = bytes.Replace(body, []byte("<!--dynamic-head-->"), []byte(head), 1)
	sendHTML(w, r, http.StatusOK, body)
}

func sendHTML(w http.ResponseWriter, r *http.Request, status int, body []byte) {
	h := w.Header()
	h.Set("Content-Type", "text/html; charset=utf-8")
	h.Set("Cache-Control", "public, no-cache")
	h.Set("Vary", "Cookie")
	h.Set("Content-Length", strconv.Itoa(len(body)))
	w.WriteHeader(status)
	if r.Method != http.MethodHead {
		_, _ = w.Write(body)
	}
}

// openFile opens a regular file of a build; directories and invalid names do not exist.
func openFile(root fs.FS, name string) (fs.File, error) {
	if name == "" {
		return nil, fs.ErrNotExist
	}
	f, err := root.Open(name)
	if err != nil {
		return nil, err
	}
	if st, err := f.Stat(); err != nil || st.IsDir() {
		f.Close()
		return nil, fs.ErrNotExist
	}
	return f, nil
}

func sendFile(w http.ResponseWriter, r *http.Request, f fs.File, name, cache string) {
	st, err := f.Stat()
	rs, ok := f.(io.ReadSeeker)
	if err != nil || !ok {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Cache-Control", cache)
	http.ServeContent(w, r, name, st.ModTime(), rs)
}
