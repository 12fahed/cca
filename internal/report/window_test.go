package report

import (
	"strings"
	"testing"
	"time"
)

func mustLoc(t *testing.T, name string) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation(name)
	if err != nil {
		t.Skipf("timezone database unavailable: %v", err)
	}
	return loc
}

func at(t *testing.T, loc *time.Location, s string) time.Time {
	t.Helper()
	ts, err := time.ParseInLocation("2006-01-02T15:04:05", s, loc)
	if err != nil {
		t.Fatal(err)
	}
	return ts
}

func TestParseWindowRelativeDays(t *testing.T) {
	loc := mustLoc(t, "UTC")
	now := at(t, loc, "2026-09-20T14:30:00")
	tests := []struct {
		spec      string
		wantStart string
	}{
		// Spans count calendar days inclusive of today, so a 7d window and a
		// seven-row daily table cover the same ground.
		{"1d", "2026-09-20T00:00:00"},
		{"2d", "2026-09-19T00:00:00"},
		{"7d", "2026-09-14T00:00:00"},
		{"30d", "2026-08-22T00:00:00"},
		{"1w", "2026-09-14T00:00:00"},
		{"2w", "2026-09-07T00:00:00"},
		{"1m", "2026-08-20T00:00:00"},
		{"3m", "2026-06-20T00:00:00"},
	}
	for _, tt := range tests {
		t.Run(tt.spec, func(t *testing.T) {
			w, err := ParseWindow(tt.spec, "", now)
			if err != nil {
				t.Fatal(err)
			}
			if got := w.Start; !got.Equal(at(t, loc, tt.wantStart)) {
				t.Errorf("start = %s, want %s", got, tt.wantStart)
			}
			if !w.End.IsZero() {
				t.Errorf("relative since should leave the end unbounded, got %s", w.End)
			}
		})
	}
}

func TestParseWindowAbsolute(t *testing.T) {
	loc := mustLoc(t, "UTC")
	now := at(t, loc, "2026-09-20T14:30:00")
	w, err := ParseWindow("2026-09-01", "2026-09-14", now)
	if err != nil {
		t.Fatal(err)
	}
	if !w.Start.Equal(at(t, loc, "2026-09-01T00:00:00")) {
		t.Errorf("start = %s", w.Start)
	}
	// --until names a day that is included in full, so the exclusive bound is
	// the start of the next day.
	if !w.End.Equal(at(t, loc, "2026-09-15T00:00:00")) {
		t.Errorf("end = %s, want 2026-09-15T00:00:00", w.End)
	}
	if !w.Contains(at(t, loc, "2026-09-14T23:59:59")) {
		t.Error("the last moment of the until day must be inside the window")
	}
	if w.Contains(at(t, loc, "2026-09-15T00:00:00")) {
		t.Error("the day after until must be outside the window")
	}
}

func TestParseWindowErrors(t *testing.T) {
	now := time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC)
	tests := []struct{ name, since, until, want string }{
		{"bad since", "yesterday", "", "--since"},
		{"bad unit", "7y", "", "--since"},
		{"zero span", "0d", "", "at least 1"},
		{"bad until", "", "not-a-date", "--until"},
		{"relative until", "", "7d", "ambiguous"},
		{"inverted", "2026-09-14", "2026-09-01", "not before"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseWindow(tt.since, tt.until, now)
			if err == nil {
				t.Fatal("want an error")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("error %q does not mention %q", err, tt.want)
			}
		})
	}
}

func TestWindowUnboundedAdmitsEverything(t *testing.T) {
	var w Window
	if !w.Unbounded() {
		t.Fatal("the zero window should be unbounded")
	}
	for _, ts := range []time.Time{
		time.Date(1999, 1, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2099, 1, 1, 0, 0, 0, 0, time.UTC),
		{}, // an unparseable timestamp
	} {
		if !w.Contains(ts) {
			t.Errorf("unbounded window rejected %v", ts)
		}
	}
}

// A record whose timestamp did not parse cannot be placed in time, so a bounded
// window must not claim it.
func TestWindowExcludesUndatedRecordsWhenBounded(t *testing.T) {
	now := time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC)
	w, _ := ParseWindow("7d", "", now)
	if w.Contains(time.Time{}) {
		t.Error("a bounded window must not admit an undated record")
	}
}

// The core timezone case. A UTC instant late on one day belongs to the next
// local day east of Greenwich and the same local day to the west, so the same
// record lands in different buckets and on different sides of a window bound.
func TestTimezoneBoundary(t *testing.T) {
	kolkata := mustLoc(t, "Asia/Kolkata")           // +05:30
	losAngeles := mustLoc(t, "America/Los_Angeles") // -07:00
	utc := mustLoc(t, "UTC")

	// 2026-09-13 23:30 UTC.
	rec := time.Date(2026, 9, 13, 23, 30, 0, 0, time.UTC)

	tests := []struct {
		loc     *time.Location
		wantDay string
	}{
		{utc, "2026-09-13"},
		{kolkata, "2026-09-14"},    // 05:00 the next morning
		{losAngeles, "2026-09-13"}, // 16:30 the same afternoon
	}
	for _, tt := range tests {
		if got := DayKey(rec, tt.loc); got != tt.wantDay {
			t.Errorf("DayKey in %s = %s, want %s", tt.loc, got, tt.wantDay)
		}
	}

	// The same instant sits inside a "from 14 September" window in Kolkata and
	// outside it in Los Angeles.
	nowK := time.Date(2026, 9, 20, 12, 0, 0, 0, kolkata)
	wK, err := ParseWindow("2026-09-14", "", nowK)
	if err != nil {
		t.Fatal(err)
	}
	if !wK.Contains(rec) {
		t.Error("Kolkata: record is on 14 September locally and should be inside")
	}

	nowL := time.Date(2026, 9, 20, 12, 0, 0, 0, losAngeles)
	wL, err := ParseWindow("2026-09-14", "", nowL)
	if err != nil {
		t.Fatal(err)
	}
	if wL.Contains(rec) {
		t.Error("Los Angeles: record is still on 13 September locally and should be outside")
	}
}

// Relative spans anchor to local midnight, which is a different instant in each
// zone even for the same wall-clock now.
func TestRelativeSpansAnchorToLocalMidnight(t *testing.T) {
	kolkata := mustLoc(t, "Asia/Kolkata")
	now := time.Date(2026, 9, 20, 14, 30, 0, 0, kolkata)
	w, err := Relative(SpecToday, now)
	if err != nil {
		t.Fatal(err)
	}
	wantStart := time.Date(2026, 9, 20, 0, 0, 0, 0, kolkata)
	if !w.Start.Equal(wantStart) {
		t.Fatalf("start = %s, want %s", w.Start, wantStart)
	}
	// 19:00 UTC on the 19th is already the 20th in Kolkata.
	if !w.Contains(time.Date(2026, 9, 19, 19, 0, 0, 0, time.UTC)) {
		t.Error("an instant that is already today locally should be inside today")
	}
	// 18:00 UTC on the 19th is still the 19th in Kolkata.
	if w.Contains(time.Date(2026, 9, 19, 18, 0, 0, 0, time.UTC)) {
		t.Error("an instant still on yesterday locally should be outside today")
	}
}

func TestNamedSpecs(t *testing.T) {
	loc := mustLoc(t, "UTC")
	now := at(t, loc, "2026-09-20T09:00:00")
	for spec, wantStart := range map[string]string{
		SpecToday: "2026-09-20T00:00:00",
		SpecWeek:  "2026-09-14T00:00:00",
		SpecMonth: "2026-08-20T00:00:00",
	} {
		w, err := Relative(spec, now)
		if err != nil {
			t.Fatal(err)
		}
		if !w.Start.Equal(at(t, loc, wantStart)) {
			t.Errorf("%s start = %s, want %s", spec, w.Start, wantStart)
		}
	}
}
