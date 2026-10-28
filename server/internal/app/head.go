package app

import (
	"context"
	"encoding/json"
	"errors"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"librehold/internal/game"
)

// PageHead is what the gate writes into a dynamic page's head before the browser runs any script.
type PageHead struct {
	Title, Description, Image string
	NoIndex                   bool
}

// SitemapEntry is one page the sitemap lists, in both languages.
type SitemapEntry struct {
	Route   string
	Params  map[string]string
	LastMod time.Time // zero when unknown
}

// HeadMessage is a catalogue message: plain text, or plural forms by CLDR category with # for the count.
type HeadMessage struct {
	Text   string
	Plural map[string]string
}

func (m *HeadMessage) UnmarshalJSON(b []byte) error {
	if len(b) > 0 && b[0] == '"' {
		return json.Unmarshal(b, &m.Text)
	}
	return json.Unmarshal(b, &m.Plural)
}

// HeadText is the client's head copy exported at build time (head-text.json): locale → key → message.
type HeadText map[string]map[string]HeadMessage

// PluralCategory is the CLDR plural category of a whole number in English or Polish.
func PluralCategory(locale string, n int) string {
	if n == 1 {
		return "one"
	}
	if locale != "pl" {
		return "other"
	}
	if d, h := n%10, n%100; d >= 2 && d <= 4 && (h < 12 || h > 14) {
		return "few"
	}
	return "many"
}

var placeholder = regexp.MustCompile(`\{(\w+)\}`)

// Format words a message in the locale, falling back to English and then to the key; unknown placeholders stay.
func (h HeadText) Format(locale, key string, params map[string]string) string {
	m, ok := h[locale][key]
	if !ok {
		if m, ok = h["en"][key]; !ok {
			return key
		}
	}
	text := m.Text
	if m.Plural != nil {
		n, _ := strconv.Atoi(params["count"])
		if text, ok = m.Plural[PluralCategory(locale, n)]; !ok {
			text = m.Plural["other"]
		}
		text = strings.ReplaceAll(text, "#", strconv.Itoa(n))
	}
	return placeholder.ReplaceAllStringFunc(text, func(p string) string {
		if v, ok := params[p[1:len(p)-1]]; ok {
			return v
		}
		return p
	})
}

func (h HeadText) Count(locale, key string, n int) string {
	return h.Format(locale, key, map[string]string{"count": strconv.Itoa(n)})
}

// positiveID reads a page's numeric id: digits only, above zero.
func positiveID(s string) (int64, bool) {
	if s == "" || strings.TrimLeft(s, "0123456789") != "" {
		return 0, false
	}
	id, err := strconv.ParseInt(s, 10, 64)
	return id, err == nil && id > 0
}

// Head is the head of a page the server fills at request time; false when the page does not exist.
func (s *Service) Head(ctx context.Context, route, locale string, params map[string]string) (PageHead, bool, error) {
	var h PageHead
	var err error
	switch route {
	case "game":
		h, err = s.gameHead(ctx, locale, params["id"])
	case "player":
		h, err = s.playerHead(ctx, locale, params["name"])
	case "join":
		h, err = s.joinHead(ctx, locale, params["code"])
	case "world":
		h, err = s.worldHead(ctx, locale, params["id"])
	case "worlds":
		t := s.cfg.HeadText
		return PageHead{Title: t.Format(locale, "head.worlds.title", nil), Description: t.Format(locale, "head.worlds.description", nil)},
			true, nil
	default:
		return PageHead{}, false, nil
	}
	if errors.Is(err, ErrNotFound) {
		return PageHead{}, false, nil
	}
	return h, err == nil, err
}

// gameHead is the head of a game page; a player world's game, reached by its code, is kept out of search.
func (s *Service) gameHead(ctx context.Context, locale, ref string) (PageHead, error) {
	id, err := s.ResolveGameRef(ctx, ref)
	if err != nil {
		return PageHead{}, err
	}
	_, byID := positiveID(ref)
	h, err := s.gameHeadOf(ctx, locale, id)
	h.NoIndex = !byID
	return h, err
}

func (s *Service) gameHeadOf(ctx context.Context, locale string, id int64) (PageHead, error) {
	g, err := s.PublicGame(ctx, id)
	if err != nil {
		return PageHead{}, err
	}
	t := s.cfg.HeadText
	p := map[string]string{
		"name":      g.Name,
		"players":   t.Count(locale, "head.count.players", g.Summary.Humans),
		"bots":      t.Count(locale, "head.count.bots", g.Summary.Bots),
		"conquests": t.Count(locale, "head.count.conquests", g.Summary.ConquestsHumans+g.Summary.ConquestsBots),
		"winner":    g.WinnerName,
	}
	if g.GameNumber > 0 {
		p["name"] = t.Format(locale, "world.gameName", map[string]string{"n": strconv.Itoa(g.GameNumber)})
	}
	if p["winner"] == "" {
		p["winner"] = t.Format(locale, "head.formerPlayer", nil)
	}
	if g.Running {
		return PageHead{Title: t.Format(locale, "head.game.running.title", p),
			Description: t.Format(locale, "head.game.running.description", p)}, nil
	}
	title := "head.game.ended.title"
	if g.WinnerIsBot {
		title = "head.game.ended.titleBot"
	}
	return PageHead{Title: t.Format(locale, title, p), Description: t.Format(locale, "head.game.ended.description", p)}, nil
}

func (s *Service) playerHead(ctx context.Context, locale, name string) (PageHead, error) {
	if name == "" {
		return PageHead{}, ErrNotFound
	}
	prof, err := s.PublicProfile(ctx, name, nil)
	if err != nil {
		return PageHead{}, err
	}
	t := s.cfg.HeadText
	p := map[string]string{
		"name":      prof.Username,
		"games":     t.Count(locale, "head.count.games", prof.Tally.Games),
		"wins":      t.Count(locale, "head.count.wins", prof.Tally.Wins),
		"conquests": t.Count(locale, "head.count.conquests", prof.Tally.Conquered),
	}
	return PageHead{Title: t.Format(locale, "head.player.title", p), Description: t.Format(locale, "head.player.description", p),
		NoIndex: prof.IsGuest}, nil
}

// joinHead is the head of a player world's invite page, never indexed.
func (s *Service) joinHead(ctx context.Context, locale, code string) (PageHead, error) {
	row, err := s.worldByInvite(ctx, code)
	if err != nil {
		return PageHead{}, err
	}
	t := s.cfg.HeadText
	p := map[string]string{"name": row.Name, "host": row.Host.HostName}
	return PageHead{Title: t.Format(locale, "head.join.title", p), Description: t.Format(locale, "head.join.description", p), NoIndex: true}, nil
}

// worldHead covers long worlds only: instant games have their page at /games/:id, player worlds none.
func (s *Service) worldHead(ctx context.Context, locale, rawID string) (PageHead, error) {
	id, ok := positiveID(rawID)
	if !ok {
		return PageHead{}, ErrNotFound
	}
	list, err := s.worlds.List(ctx)
	if err != nil {
		return PageHead{}, err
	}
	for _, w := range list {
		if w.ID != id || w.Config.Kind != game.KindLong {
			continue
		}
		t := s.cfg.HeadText
		p := map[string]string{
			"name":    w.Name,
			"day":     strconv.FormatInt(WorldNow(&w.WorldRow, s.clock.NowMs())/game.MsPerDay+1, 10),
			"players": t.Count(locale, "head.count.players", w.Humans),
			"bots":    t.Count(locale, "head.count.bots", w.Bots),
		}
		return PageHead{Title: t.Format(locale, "head.world.title", p), Description: t.Format(locale, "head.world.description", p)}, nil
	}
	return PageHead{}, ErrNotFound
}

// SitemapEntries lists the dynamic pages worth indexing: long worlds, finished games and the registered players in them.
func (s *Service) SitemapEntries(ctx context.Context) ([]SitemapEntry, error) {
	var out []SitemapEntry
	list, err := s.worlds.List(ctx)
	if err != nil {
		return nil, err
	}
	for _, w := range list {
		if w.Config.Kind == game.KindLong {
			out = append(out, SitemapEntry{Route: "world", Params: map[string]string{"id": strconv.FormatInt(w.ID, 10)}})
		}
	}
	games, err := s.worlds.ArchivedGameList(ctx, 0)
	if err != nil {
		return nil, err
	}
	for _, g := range games {
		out = append(out, SitemapEntry{Route: "game", Params: map[string]string{"id": strconv.FormatInt(g.WorldID, 10)},
			LastMod: time.UnixMilli(g.EndedAtMs)})
	}
	latest, err := s.worlds.AccountsWithGames(ctx)
	if err != nil {
		return nil, err
	}
	ids := make([]int64, 0, len(latest))
	for id := range latest {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	for _, id := range ids {
		acct, err := s.accounts.ByID(ctx, id)
		if errors.Is(err, ErrNotFound) {
			continue
		}
		if err != nil {
			return nil, err
		}
		if !acct.IsGuest {
			out = append(out, SitemapEntry{Route: "player", Params: map[string]string{"name": acct.Username},
				LastMod: time.UnixMilli(latest[id])})
		}
	}
	return out, nil
}
