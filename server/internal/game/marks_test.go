package game_test

import (
	"errors"
	"testing"

	"librehold/internal/game"
	"librehold/internal/game/gametest"
)

func marksWorld(t *testing.T) *gametest.MemWorld {
	t.Helper()
	w, _ := gametest.NewTestWorld(7)
	return w
}

func TestSetMarksKeepsThePlayersChoice(t *testing.T) {
	w := marksWorld(t)
	if err := game.SetMarks(w, "h1", []game.PlayerMark{
		{Name: "Grimgar", Color: "#e6194b"},
		{Name: "Wenard", Color: "#3cb44b"},
	}); err != nil {
		t.Fatal(err)
	}
	got := w.Human("h1").Marks
	if len(got) != 2 || got[0].Name != "Grimgar" || got[1].Color != "#3cb44b" {
		t.Fatalf("marks %+v", got)
	}
	// The marks replace the old set rather than adding to it, so a removal reaches the server too.
	if err := game.SetMarks(w, "h1", []game.PlayerMark{{Name: "Wenard", Color: "#4363d8"}}); err != nil {
		t.Fatal(err)
	}
	if got := w.Human("h1").Marks; len(got) != 1 || got[0].Color != "#4363d8" {
		t.Fatalf("marks after replace %+v", got)
	}
	if err := game.SetMarks(w, "h1", nil); err != nil {
		t.Fatal(err)
	}
	if got := w.Human("h1").Marks; len(got) != 0 {
		t.Fatalf("marks after clearing %+v", got)
	}
}

func TestSetMarksTrimsAndFoldsNames(t *testing.T) {
	w := marksWorld(t)
	if err := game.SetMarks(w, "h1", []game.PlayerMark{
		{Name: "  Grimgar ", Color: "#e6194b"},
		{Name: "grimgar", Color: "#3cb44b"},
	}); err != nil {
		t.Fatal(err)
	}
	got := w.Human("h1").Marks
	// One name, kept once, in the place it first appeared; the repeat is dropped rather than recolouring the mark,
	// since the colour on the chip is what a player recolours with.
	if len(got) != 1 || got[0].Name != "Grimgar" || got[0].Color != "#e6194b" {
		t.Fatalf("marks %+v", got)
	}
}

func TestSetMarksDropsWhatCannotBeDrawn(t *testing.T) {
	w := marksWorld(t)
	long := ""
	for range 25 {
		long += "a"
	}
	if err := game.SetMarks(w, "h1", []game.PlayerMark{
		{Name: "Keeper", Color: "#ABCDEF"},
		{Name: "Red", Color: "red"},
		{Name: "Short", Color: "#abc"},
		{Name: "Injected", Color: "#12345g"},
		{Name: long, Color: "#000000"},
		{Name: "", Color: "#000000"},
	}); err != nil {
		t.Fatal(err)
	}
	got := w.Human("h1").Marks
	if len(got) != 1 || got[0].Name != "Keeper" || got[0].Color != "#ABCDEF" {
		t.Fatalf("marks %+v", got)
	}
}

func TestSetMarksCapsTheList(t *testing.T) {
	w := marksWorld(t)
	marks := make([]game.PlayerMark, game.MaxMarks+5)
	for i := range marks {
		marks[i] = game.PlayerMark{Name: "p" + string(rune('a'+i%26)) + string(rune('a'+i/26)), Color: "#e6194b"}
	}
	if err := game.SetMarks(w, "h1", marks); err != nil {
		t.Fatal(err)
	}
	if got := len(w.Human("h1").Marks); got != game.MaxMarks {
		t.Fatalf("kept %d marks", got)
	}
}

func TestSetMarksRefusesABot(t *testing.T) {
	w := marksWorld(t)
	bot := gametest.MakeBotVillage(w, w.Village(w.Player("h1").VillageIDs[0]))
	err := game.SetMarks(w, game.PlayerID(bot.OwnerID), []game.PlayerMark{{Name: "Grimgar", Color: "#e6194b"}})
	var be *game.BlockedError
	if !errors.As(err, &be) || !be.HasKind("notOwner") {
		t.Fatalf("a bot marking players: %v", err)
	}
}
