package web

import (
	"encoding/json"
	"errors"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"librehold/internal/app"
)

// routeDef is one entry of routes.json, the account site's route table written by the build.
type routeDef struct {
	ID    string `json:"id"`
	En    string `json:"en"`
	Pl    string `json:"pl"`
	Index string `json:"index"` // yes | no | registered
	Build string `json:"build"` // prerender | dynamic | app
}

type compiledRoute struct {
	def    routeDef
	locale string
	segs   []string
}

// site is the account build as the gate reads it.
type site struct {
	files  fs.FS
	routes []compiledRoute
	twins  map[string]string // prerendered page → its other-language page (pages.json)
	text   app.HeadText
}

func segments(p string) []string {
	trimmed := strings.Trim(p, "/")
	if trimmed == "" {
		return nil
	}
	return strings.Split(trimmed, "/")
}

func readJSON(dir, name string, into any) error {
	b, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		return err
	}
	return json.Unmarshal(b, into)
}

// loadSite reads the route table (required), the language pairs and the head copy (optional) of an account build.
func loadSite(accountDir string) (*site, error) {
	var defs []routeDef
	if err := readJSON(accountDir, "routes.json", &defs); err != nil {
		return nil, err
	}
	s := &site{files: os.DirFS(accountDir), twins: map[string]string{}, text: app.HeadText{}}
	for _, d := range defs {
		s.routes = append(s.routes, compiledRoute{def: d, locale: "en", segs: segments(d.En)},
			compiledRoute{def: d, locale: "pl", segs: segments(d.Pl)})
	}
	var pairs []struct{ Route, En, Pl string }
	if err := readJSON(accountDir, "pages.json", &pairs); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}
	for _, p := range pairs {
		s.twins[p.En], s.twins[p.Pl] = p.Pl, p.En
	}
	if err := readJSON(accountDir, "head-text.json", &s.text); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}
	return s, nil
}

// match finds the route of an escaped path; params come back decoded. "/pl" never matches: it redirects to "/pl/".
func (s *site) match(p string) (routeDef, string, map[string]string, bool) {
	if p == "/pl" {
		return routeDef{}, "", nil, false
	}
	parts := segments(p)
	for _, r := range s.routes {
		if len(r.segs) != len(parts) {
			continue
		}
		params := map[string]string{}
		ok := true
		for i, seg := range r.segs {
			if name, isParam := strings.CutPrefix(seg, ":"); isParam {
				v, err := url.PathUnescape(parts[i])
				if err != nil || v == "" {
					ok = false
					break
				}
				params[name] = v
			} else if seg != parts[i] {
				ok = false
				break
			}
		}
		if ok {
			return r.def, r.locale, params, true
		}
	}
	return routeDef{}, "", nil, false
}

// pathOf is a route's address in a language, params escaped.
func (s *site) pathOf(id, locale string, params map[string]string) string {
	for _, r := range s.routes {
		if r.def.ID != id || r.locale != locale {
			continue
		}
		if len(r.segs) == 0 {
			return "/"
		}
		parts := make([]string, len(r.segs))
		for i, seg := range r.segs {
			if name, isParam := strings.CutPrefix(seg, ":"); isParam {
				parts[i] = url.PathEscape(params[name])
			} else {
				parts[i] = seg
			}
		}
		out := "/" + strings.Join(parts, "/")
		if out == "/pl" {
			out = "/pl/"
		}
		return out
	}
	return "/"
}

// twin is the same page in the other language: the build's pair for a prerendered page (slugs differ by
// language), else the same params at the other language's address.
func (s *site) twin(p string, r routeDef, locale string, params map[string]string) string {
	if t, ok := s.twins[p]; ok {
		return t
	}
	return s.pathOf(r.ID, other(locale), params)
}

func other(locale string) string {
	if locale == "pl" {
		return "en"
	}
	return "pl"
}

// origin is the site's scheme and host: the configured public address, else the request's own.
func origin(r *http.Request, publicURL string) string {
	if publicURL != "" {
		return publicURL
	}
	scheme := "http"
	if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	return scheme + "://" + r.Host
}
