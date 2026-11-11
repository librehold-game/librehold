package app_test

import (
	"slices"
	"testing"

	"librehold/internal/app"
	"librehold/internal/game"
)

// Support that dies in another player's village leaves its owner only a report, yet their units changed.
func TestReportRecipientsAlsoGetTheirState(t *testing.T) {
	ups := app.Changes{ReportRecipients: []game.PlayerID{"h3"}}.Updates(1)
	if len(ups) != 1 || ups[0].PlayerID != "h3" || !slices.Equal(ups[0].Topics, []string{app.TopicReports, app.TopicState}) {
		t.Fatalf("updates %+v", ups)
	}
}
