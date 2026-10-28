package web

import (
	"context"
	"encoding/json"
	"html"
	"os"
	"path/filepath"
	"strings"

	"librehold/internal/app"
)

// AuthKind is who a request belongs to, as far as the gate cares.
type AuthKind int

const (
	AuthNone AuthKind = iota
	AuthGuest
	AuthRegistered
)

type PageHead = app.PageHead

// HeadSource words the head of a page the server fills at request time; false means the page does not exist.
type HeadSource interface {
	Head(ctx context.Context, route, locale string, params map[string]string) (PageHead, bool, error)
}

const defaultImage = "/og/home.png"

var ogLocale = map[string]string{"en": "en_US", "pl": "pl_PL"}

// LoadHeadText reads the head copy the account build exported, for the service's server-side titles.
func LoadHeadText(dir string) (app.HeadText, error) {
	b, err := os.ReadFile(filepath.Join(dir, "account", "head-text.json"))
	if err != nil {
		return nil, err
	}
	var t app.HeadText
	return t, json.Unmarshal(b, &t)
}

// headTags is the same tag set the build writes into prerendered pages (src/account/headTags.ts), in the same order.
func (s *site) headTags(h PageHead, p string, r routeDef, locale string, params map[string]string, origin string) string {
	meta := func(attr, key, value string) string {
		return `<meta ` + attr + `="` + key + `" content="` + html.EscapeString(value) + `">`
	}
	link := func(rel, extra, href string) string {
		return `<link rel="` + rel + `"` + extra + ` href="` + html.EscapeString(href) + `">`
	}
	own := s.pathOf(r.ID, locale, params)
	twin := s.twin(own, r, locale, params)
	en, pl := own, twin
	if locale == "pl" {
		en, pl = twin, own
	}
	out := []string{"<title>" + html.EscapeString(h.Title) + "</title>", meta("name", "description", h.Description)}
	if h.NoIndex || r.Index == "no" {
		out = append(out, `<meta name="robots" content="noindex">`)
	}
	out = append(out, link("canonical", "", origin+own),
		link("alternate", ` hreflang="en"`, origin+en), link("alternate", ` hreflang="pl"`, origin+pl),
		link("alternate", ` hreflang="x-default"`, origin+en))
	image := h.Image
	if image == "" {
		image = defaultImage
	}
	out = append(out,
		meta("property", "og:type", "website"),
		meta("property", "og:site_name", s.text.Format(locale, "app.name", nil)),
		meta("property", "og:title", h.Title),
		meta("property", "og:description", h.Description),
		meta("property", "og:url", origin+own),
		meta("property", "og:image", origin+image),
		meta("property", "og:image:width", "1200"),
		meta("property", "og:image:height", "630"),
		meta("property", "og:image:alt", s.text.Format(locale, "head.imageAlt", nil)),
		meta("property", "og:locale", ogLocale[locale]),
		meta("property", "og:locale:alternate", ogLocale[other(locale)]),
		meta("name", "twitter:card", "summary_large_image"),
	)
	return strings.Join(out, "\n")
}

// appHead is the head of a page only the browser fills: the site's name, kept out of search results.
func (s *site) appHead(locale string) string {
	return "<title>" + html.EscapeString(s.text.Format(locale, "app.name", nil)) + `</title><meta name="robots" content="noindex">`
}
