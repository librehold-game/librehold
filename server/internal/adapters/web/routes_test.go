package web

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMatchAndPathOf(t *testing.T) {
	dir := t.TempDir()
	routes := `[{"id":"home","en":"/","pl":"/pl/","index":"yes","build":"prerender"},
		{"id":"player","en":"/players/:name","pl":"/pl/gracze/:name","index":"registered","build":"dynamic"}]`
	if err := os.WriteFile(filepath.Join(dir, "routes.json"), []byte(routes), 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := loadSite(dir)
	if err != nil {
		t.Fatal(err)
	}
	r, locale, params, ok := s.match("/pl/gracze/a%20b")
	if !ok || r.ID != "player" || locale != "pl" || params["name"] != "a b" {
		t.Fatalf("match %+v %s %v %v", r, locale, params, ok)
	}
	if p := s.pathOf("player", "en", params); p != "/players/a%20b" {
		t.Fatalf("pathOf %q", p)
	}
	if p := s.twin("/pl/gracze/a%20b", r, "pl", params); p != "/players/a%20b" {
		t.Fatalf("twin %q", p)
	}
	for _, p := range []string{"/pl", "/players", "/players/", "/players/a/b", "/players/%zz", "/nope"} {
		if _, _, _, ok := s.match(p); ok {
			t.Errorf("%s matched", p)
		}
	}
	if r, locale, _, ok := s.match("/pl/"); !ok || r.ID != "home" || locale != "pl" {
		t.Fatalf("home pl %+v %s", r, locale)
	}
}
