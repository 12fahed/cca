// Package report aggregates parsed usage records by model, project, day, and
// session. It is pure: no I/O, no formatting.
package report

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Window is a half-open time range, [Start, End). A zero Start or End means
// that side is unbounded.
//
// Records carry UTC timestamps but users think in local days, so every boundary
// here is computed in the caller's location. Getting that wrong shifts whole
// days of usage into the neighbouring bucket.
type Window struct {
	Start time.Time
	End   time.Time
}

// Unbounded reports whether the window admits everything.
func (w Window) Unbounded() bool { return w.Start.IsZero() && w.End.IsZero() }

// Contains reports whether t falls inside the window.
//
// A zero timestamp means the record's own timestamp did not parse. Such a
// record cannot be placed in time, so it is admitted only when no bound was
// requested; claiming it falls inside a window would be a guess.
func (w Window) Contains(t time.Time) bool {
	if t.IsZero() {
		return w.Unbounded()
	}
	if !w.Start.IsZero() && t.Before(w.Start) {
		return false
	}
	if !w.End.IsZero() && !t.Before(w.End) {
		return false
	}
	return true
}

// ParseWindow builds a window from the --since and --until flag values. An
// empty string leaves that side unbounded. now supplies both the reference
// point for relative specs and the location every boundary is computed in.
func ParseWindow(since, until string, now time.Time) (Window, error) {
	var w Window
	if since != "" {
		start, err := parseSince(since, now)
		if err != nil {
			return Window{}, err
		}
		w.Start = start
	}
	if until != "" {
		end, err := parseUntil(until, now)
		if err != nil {
			return Window{}, err
		}
		w.End = end
	}
	if !w.Start.IsZero() && !w.End.IsZero() && !w.Start.Before(w.End) {
		return Window{}, fmt.Errorf("--since %s is not before --until %s", since, until)
	}
	return w, nil
}

// Relative builds a window from a relative spec alone, for the today, week, and
// month subcommands.
func Relative(spec string, now time.Time) (Window, error) {
	start, err := parseSince(spec, now)
	if err != nil {
		return Window{}, err
	}
	return Window{Start: start}, nil
}

// Named window specs behind the bare subcommands.
const (
	SpecToday = "1d"
	SpecWeek  = "7d"
	SpecMonth = "1m"
)

func parseSince(s string, now time.Time) (time.Time, error) {
	if n, unit, ok := splitRelative(s); ok {
		return relativeStart(n, unit, now)
	}
	d, err := parseDate(s, now.Location())
	if err != nil {
		return time.Time{}, fmt.Errorf("--since %q: want a date like 2026-09-01 "+
			"or a relative span like 7d, 2w, 3m", s)
	}
	return d, nil
}

func parseUntil(s string, now time.Time) (time.Time, error) {
	if _, _, ok := splitRelative(s); ok {
		return time.Time{}, fmt.Errorf("--until %q: relative spans are ambiguous "+
			"as an end bound; give a date like 2026-09-14", s)
	}
	d, err := parseDate(s, now.Location())
	if err != nil {
		return time.Time{}, fmt.Errorf("--until %q: want a date like 2026-09-14", s)
	}
	// The named day is included in full, so the exclusive bound is the start of
	// the following day.
	return d.AddDate(0, 0, 1), nil
}

// relativeStart resolves a span such as 7d to the start of the earliest local
// day it covers.
//
// Spans count calendar days inclusive of today: 1d is today alone and 7d is
// today plus the six days before it, so `--since 7d` and a seven-row daily
// table agree. Months step back by calendar month instead, since a month has
// no fixed length.
func relativeStart(n int, unit byte, now time.Time) (time.Time, error) {
	if n < 1 {
		return time.Time{}, fmt.Errorf("relative span must be at least 1, got %d", n)
	}
	today := startOfDay(now)
	switch unit {
	case 'd':
		return today.AddDate(0, 0, -(n - 1)), nil
	case 'w':
		return today.AddDate(0, 0, -(n*7 - 1)), nil
	case 'm':
		// AddDate normalises overflow, so one month before 31 March lands in
		// early March rather than February. That is acceptable for a usage
		// window and avoids inventing a month length.
		return today.AddDate(0, -n, 0), nil
	}
	return time.Time{}, fmt.Errorf("unknown span unit %q", string(unit))
}

func splitRelative(s string) (int, byte, bool) {
	if len(s) < 2 {
		return 0, 0, false
	}
	unit := s[len(s)-1] | 0x20 // lowercase
	if unit != 'd' && unit != 'w' && unit != 'm' {
		return 0, 0, false
	}
	n, err := strconv.Atoi(s[:len(s)-1])
	if err != nil {
		return 0, 0, false
	}
	return n, unit, true
}

func parseDate(s string, loc *time.Location) (time.Time, error) {
	return time.ParseInLocation("2006-01-02", strings.TrimSpace(s), loc)
}

func startOfDay(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}

// DayKey is the local calendar day a record belongs to, formatted for grouping
// and display. The conversion to local time is what makes buckets line up with
// the user's idea of a day.
func DayKey(t time.Time, loc *time.Location) string {
	return t.In(loc).Format("2006-01-02")
}
