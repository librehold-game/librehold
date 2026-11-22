package app_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"librehold/internal/app"
	"librehold/internal/game"
)

func TestSetMarksOrderFollowsThePlayer(t *testing.T) {
	d := newDuel(t)
	ctx := context.Background()
	body := `{"marks":[{"name":"Grimgar","color":"#e6194b"},{"name":"Wenard","color":"#3cb44b"}]}`
	st, err := d.svc.Order(ctx, ala, d.world, "set-marks", json.RawMessage(body))
	if err != nil {
		t.Fatal(err)
	}
	if len(st.Human.Marks) != 2 || st.Human.Marks[0].Name != "Grimgar" {
		t.Fatalf("the order answered %+v", st.Human.Marks)
	}
	// The marks live in the player's record, so a fresh state (another device, another day) still has them.
	again := d.state(t, ala)
	if len(again.Human.Marks) != 2 || again.Human.Marks[1].Color != "#3cb44b" {
		t.Fatalf("the marks did not stay: %+v", again.Human.Marks)
	}
	// Another player's marks are their own.
	if theirs := d.state(t, bartek); len(theirs.Human.Marks) != 0 {
		t.Fatalf("the marks leaked to another player: %+v", theirs.Human.Marks)
	}
}

func TestSetMarksOrderRefusesTooMany(t *testing.T) {
	d := newDuel(t)
	marks := make([]game.PlayerMark, game.MaxMarks+1)
	for i := range marks {
		marks[i] = game.PlayerMark{Name: "p", Color: "#e6194b"}
	}
	body, _ := json.Marshal(map[string]any{"marks": marks})
	_, err := d.svc.Order(context.Background(), ala, d.world, "set-marks", body)
	var ve *app.ErrValidation
	if !errors.As(err, &ve) {
		t.Fatalf("too many marks: %v", err)
	}
}
