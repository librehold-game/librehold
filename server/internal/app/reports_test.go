package app_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"librehold/internal/app"
)

var reportAdmin = &app.Account{ID: 99, Username: "admin", IsAdmin: true}

func storedReports(t *testing.T, f *instantFixture) []app.ProblemReport {
	t.Helper()
	list, err := f.svc.ProblemReports(context.Background(), reportAdmin)
	if err != nil {
		t.Fatal(err)
	}
	return list
}

func validReport() app.ReportInput {
	return app.ReportInput{Kind: "bug", Text: "The map stays black", Client: "10.1.1.1"}
}

func TestSubmitReportAsVisitorAndAccount(t *testing.T) {
	ctx := context.Background()
	f := newInstantFixture(t, app.Config{})
	if _, err := f.svc.SubmitReport(ctx, nil, validReport()); err != nil {
		t.Fatal(err)
	}
	guest := f.guest(t)
	wid := f.play(t, guest).WorldID
	row, _ := f.worlds.Get(ctx, wid)
	in := validReport()
	in.WorldID, in.Screen, in.Build, in.UserAgent, in.Email = wid, "map", "abc123", "Firefox 140", "ala@example.org"
	id, err := f.svc.SubmitReport(ctx, guest, in)
	if err != nil || id == 0 {
		t.Fatalf("report %d %v", id, err)
	}
	list := storedReports(t, f)
	if len(list) != 2 {
		t.Fatalf("list %+v", list)
	}
	g, v := list[0], list[1]
	if v.AccountID != 0 || v.WorldID != 0 || v.Kind != "bug" || v.Status != "new" {
		t.Fatalf("visitor's %+v", v)
	}
	if g.ID != id || g.AccountID != guest.ID || g.WorldID != wid || g.WorldName != row.Name || g.Screen != "map" ||
		g.Build != "abc123" || g.UserAgent != "Firefox 140" || g.Email != "ala@example.org" || !g.CreatedAt.Equal(time.UnixMilli(f.clock.NowMs())) {
		t.Fatalf("guest's %+v", g)
	}
}

// Review focus 2: lengths are counted in characters, not bytes, and only the text, kind and email can be refused.
func TestSubmitReportValidates(t *testing.T) {
	ctx := context.Background()
	for _, c := range []struct {
		name  string
		edit  func(in *app.ReportInput)
		field string
	}{
		{"unknown kind", func(in *app.ReportInput) { in.Kind = "spam" }, "reportKind"},
		{"blank text", func(in *app.ReportInput) { in.Text = "   " }, "reportText"},
		{"nine characters", func(in *app.ReportInput) { in.Text = "123456789" }, "reportText"},
		{"ten Polish letters", func(in *app.ReportInput) { in.Text = "ąęółżźćńśą" }, ""},
		{"2000 emoji", func(in *app.ReportInput) { in.Text = strings.Repeat("🏰", 2000) }, ""},
		{"2001 characters", func(in *app.ReportInput) { in.Text = strings.Repeat("a", 2001) }, "reportText"},
		{"email without at", func(in *app.ReportInput) { in.Email = "x" }, "reportEmail"},
		{"email too long", func(in *app.ReportInput) { in.Email = strings.Repeat("a", 250) + "@b.pl" }, "reportEmail"},
		{"no email", func(in *app.ReportInput) { in.Email = "" }, ""},
		{"a long user agent", func(in *app.ReportInput) { in.UserAgent = strings.Repeat("ż", 1000) }, ""},
	} {
		t.Run(c.name, func(t *testing.T) {
			f := newInstantFixture(t, app.Config{})
			in := validReport()
			c.edit(&in)
			_, err := f.svc.SubmitReport(ctx, nil, in)
			if c.field == "" {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			var invalid *app.ErrValidation
			if !errors.As(err, &invalid) || invalid.Code != c.field {
				t.Fatalf("err %v, want %s", err, c.field)
			}
		})
	}
	f := newInstantFixture(t, app.Config{})
	in := validReport()
	in.UserAgent, in.Screen, in.Build = strings.Repeat("ż", 1000), strings.Repeat("s", 100), strings.Repeat("b", 100)
	if _, err := f.svc.SubmitReport(ctx, nil, in); err != nil {
		t.Fatal(err)
	}
	r := storedReports(t, f)[0]
	if utf8.RuneCountInString(r.UserAgent) != 400 || len(r.Screen) != 40 || len(r.Build) != 64 {
		t.Fatalf("cut to %d, %d, %d", utf8.RuneCountInString(r.UserAgent), len(r.Screen), len(r.Build))
	}
}

func TestSubmitReportLimitedFivePerHourPerAddress(t *testing.T) {
	ctx := context.Background()
	f := newInstantFixture(t, app.Config{})
	bad := validReport()
	bad.Kind = "spam"
	for range 3 {
		_, _ = f.svc.SubmitReport(ctx, nil, bad)
	}
	for i := range 5 {
		if _, err := f.svc.SubmitReport(ctx, nil, validReport()); err != nil {
			t.Fatalf("report %d: %v", i+1, err)
		}
	}
	if _, err := f.svc.SubmitReport(ctx, nil, validReport()); !errors.Is(err, app.ErrTooManyReports) {
		t.Fatalf("sixth: %v", err)
	}
	other := validReport()
	other.Client = "10.2.2.2"
	if _, err := f.svc.SubmitReport(ctx, nil, other); err != nil {
		t.Fatalf("another address: %v", err)
	}
	f.clock.Add(time.Hour.Milliseconds())
	if _, err := f.svc.SubmitReport(ctx, nil, validReport()); err != nil {
		t.Fatalf("an hour later: %v", err)
	}
}

func TestProblemReportsAdminOnly(t *testing.T) {
	ctx := context.Background()
	f := newInstantFixture(t, app.Config{})
	player := &app.Account{ID: 5, Username: "ala"}
	if _, err := f.svc.ProblemReports(ctx, player); !errors.Is(err, app.ErrForbidden) {
		t.Fatalf("list: %v", err)
	}
	id, _ := f.svc.SubmitReport(ctx, nil, validReport())
	if err := f.svc.SetProblemReportStatus(ctx, player, id, "done"); !errors.Is(err, app.ErrForbidden) {
		t.Fatalf("status: %v", err)
	}
	var invalid *app.ErrValidation
	if err := f.svc.SetProblemReportStatus(ctx, reportAdmin, id, "gone"); !errors.As(err, &invalid) || invalid.Code != "statusInvalid" {
		t.Fatalf("bad status: %v", err)
	}
}

func TestClosedReportsPurgedAfterAYear(t *testing.T) {
	ctx := context.Background()
	f := newInstantFixture(t, app.Config{})
	done, _ := f.svc.SubmitReport(ctx, nil, validReport())
	open, _ := f.svc.SubmitReport(ctx, nil, validReport())
	if err := f.svc.SetProblemReportStatus(ctx, reportAdmin, done, "done"); err != nil {
		t.Fatal(err)
	}
	f.clock.Add(364 * 24 * time.Hour.Milliseconds())
	if n, err := f.svc.PurgeClosedReports(ctx); err != nil || n != 0 || len(storedReports(t, f)) != 2 {
		t.Fatalf("after 364 days: %d %v", n, err)
	}
	f.clock.Add(2 * 24 * time.Hour.Milliseconds())
	if n, err := f.svc.PurgeClosedReports(ctx); err != nil || n != 1 {
		t.Fatalf("after 366 days: %d %v", n, err)
	}
	if list := storedReports(t, f); len(list) != 1 || list[0].ID != open {
		t.Fatalf("left %+v", list)
	}
}
