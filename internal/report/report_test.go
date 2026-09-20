package report

import (
	"math"
	"testing"
	"time"

	"cca/internal/pricing"
	"cca/internal/transcript"
)

func calc(t *testing.T) *pricing.Calculator {
	t.Helper()
	tbl, err := pricing.Embedded()
	if err != nil {
		t.Fatal(err)
	}
	return pricing.NewCalculator(tbl)
}

type recOpt func(*transcript.Record)

func withTime(ts time.Time) recOpt        { return func(r *transcript.Record) { r.Timestamp = ts } }
func withProject(p string) recOpt         { return func(r *transcript.Record) { r.Project = p } }
func withSession(s string) recOpt         { return func(r *transcript.Record) { r.SessionID = s } }
func sidechain() recOpt                   { return func(r *transcript.Record) { r.IsSidechain = true } }
func withUsage(u transcript.Usage) recOpt { return func(r *transcript.Record) { r.Usage = u } }

func rec(model string, opts ...recOpt) transcript.Record {
	r := transcript.Record{
		Model:     model,
		Project:   "proj",
		SessionID: "sess",
		Timestamp: time.Date(2026, 9, 14, 12, 0, 0, 0, time.UTC),
		Usage:     transcript.Usage{Input: 1e6},
	}
	for _, o := range opts {
		o(&r)
	}
	return r
}

func closeTo(t *testing.T, label string, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 1e-9 {
		t.Errorf("%s = %.10f, want %.10f", label, got, want)
	}
}

func build(t *testing.T, recs []transcript.Record, opts Options) *Report {
	t.Helper()
	if opts.Location == nil {
		opts.Location = time.UTC
	}
	return Build(recs, calc(t), opts)
}

func TestBuildOverallTotals(t *testing.T) {
	rep := build(t, []transcript.Record{
		rec("claude-opus-5"),   // 1M input  -> $5
		rec("claude-sonnet-5"), // 1M input  -> $2
	}, Options{})

	if rep.Overall.Requests != 2 {
		t.Errorf("requests = %d, want 2", rep.Overall.Requests)
	}
	if got := rep.Overall.Tokens.Total(); got != 2e6 {
		t.Errorf("tokens = %d, want 2000000", got)
	}
	closeTo(t, "cost", rep.Overall.Cost.Total, 7)
	if rep.Overall.Sessions != 1 || rep.Overall.Projects != 1 {
		t.Errorf("sessions/projects = %d/%d, want 1/1", rep.Overall.Sessions, rep.Overall.Projects)
	}
}

func TestBuildByModelRankedByCost(t *testing.T) {
	rep := build(t, []transcript.Record{
		rec("claude-haiku-4-5"), // $1
		rec("claude-opus-5"),    // $5
		rec("claude-sonnet-5"),  // $2
		rec("claude-opus-5"),    // $5 -> $10 total
	}, Options{})

	want := []struct {
		key      string
		cost     float64
		requests int64
	}{
		{"claude-opus-5", 10, 2},
		{"claude-sonnet-5", 2, 1},
		{"claude-haiku-4-5", 1, 1},
	}
	if len(rep.ByModel) != len(want) {
		t.Fatalf("got %d model groups, want %d", len(rep.ByModel), len(want))
	}
	for i, w := range want {
		g := rep.ByModel[i]
		if g.Key != w.key {
			t.Errorf("position %d = %s, want %s", i, g.Key, w.key)
		}
		closeTo(t, w.key+" cost", g.Cost.Total, w.cost)
		if g.Requests != w.requests {
			t.Errorf("%s requests = %d, want %d", w.key, g.Requests, w.requests)
		}
	}
}

func TestBuildByProject(t *testing.T) {
	rep := build(t, []transcript.Record{
		rec("claude-opus-5", withProject("a"), withSession("s1")),
		rec("claude-opus-5", withProject("a"), withSession("s2")),
		rec("claude-sonnet-5", withProject("b"), withSession("s3")),
	}, Options{})

	if len(rep.ByProject) != 2 {
		t.Fatalf("got %d projects, want 2", len(rep.ByProject))
	}
	if rep.ByProject[0].Key != "a" {
		t.Errorf("costliest project = %s, want a", rep.ByProject[0].Key)
	}
	if rep.ByProject[0].Sessions != 2 {
		t.Errorf("project a sessions = %d, want 2", rep.ByProject[0].Sessions)
	}
	if rep.Overall.Projects != 2 {
		t.Errorf("overall projects = %d, want 2", rep.Overall.Projects)
	}
}

func TestBuildByDayIsChronological(t *testing.T) {
	day := func(d int) time.Time { return time.Date(2026, 9, d, 12, 0, 0, 0, time.UTC) }
	rep := build(t, []transcript.Record{
		rec("claude-opus-5", withTime(day(16))),
		rec("claude-opus-5", withTime(day(14))),
		rec("claude-opus-5", withTime(day(15))),
		rec("claude-opus-5", withTime(day(14))),
	}, Options{})

	want := []string{"2026-09-14", "2026-09-15", "2026-09-16"}
	if len(rep.ByDay) != len(want) {
		t.Fatalf("got %d days, want %d", len(rep.ByDay), len(want))
	}
	for i, w := range want {
		if rep.ByDay[i].Key != w {
			t.Errorf("position %d = %s, want %s (newest must be last)", i, rep.ByDay[i].Key, w)
		}
	}
	if rep.ByDay[0].Requests != 2 {
		t.Errorf("14 Sep requests = %d, want 2", rep.ByDay[0].Requests)
	}
}

func TestBuildBySessionRankedAndCapped(t *testing.T) {
	big := transcript.Usage{Input: 1e6}
	small := transcript.Usage{Input: 1e5}
	recs := []transcript.Record{
		rec("claude-opus-5", withSession("cheap"), withUsage(small)),
		rec("claude-opus-5", withSession("dear"), withUsage(big)),
		rec("claude-opus-5", withSession("middle"), withUsage(transcript.Usage{Input: 5e5})),
	}

	all := build(t, recs, Options{})
	if len(all.BySession) != 3 {
		t.Fatalf("got %d sessions, want 3", len(all.BySession))
	}
	if all.BySession[0].Key != "dear" || all.BySession[2].Key != "cheap" {
		t.Errorf("sessions not ranked by cost: %v", keys(all.BySession))
	}
	if all.BySession[0].Project != "proj" {
		t.Errorf("session group should carry its project, got %q", all.BySession[0].Project)
	}

	top := build(t, recs, Options{TopSessions: 2})
	if len(top.BySession) != 2 {
		t.Fatalf("TopSessions = 2 gave %d rows", len(top.BySession))
	}
	if top.BySession[0].Key != "dear" || top.BySession[1].Key != "middle" {
		t.Errorf("wrong sessions kept: %v", keys(top.BySession))
	}
	// Capping the session table must not change the totals.
	closeTo(t, "overall cost unchanged", top.Overall.Cost.Total, all.Overall.Cost.Total)
}

func TestBuildSidechainSplit(t *testing.T) {
	rep := build(t, []transcript.Record{
		rec("claude-opus-5"),
		rec("claude-opus-5", sidechain()),
		rec("claude-opus-5", sidechain()),
	}, Options{})

	if rep.Main.Requests != 1 || rep.Sidechain.Requests != 2 {
		t.Errorf("split = %d main / %d sidechain, want 1/2",
			rep.Main.Requests, rep.Sidechain.Requests)
	}
	// Sub-agent usage is real spend and stays in the overall totals.
	closeTo(t, "overall", rep.Overall.Cost.Total, 15)
	closeTo(t, "main+sidechain", rep.Main.Cost.Total+rep.Sidechain.Cost.Total,
		rep.Overall.Cost.Total)
}

func TestBuildWindowFiltering(t *testing.T) {
	day := func(d int) time.Time { return time.Date(2026, 9, d, 12, 0, 0, 0, time.UTC) }
	recs := []transcript.Record{
		rec("claude-opus-5", withTime(day(10))),
		rec("claude-opus-5", withTime(day(14))),
		rec("claude-opus-5", withTime(day(20))),
	}
	now := time.Date(2026, 9, 20, 23, 0, 0, 0, time.UTC)
	w, err := ParseWindow("2026-09-14", "2026-09-20", now)
	if err != nil {
		t.Fatal(err)
	}
	rep := build(t, recs, Options{Window: w})

	if rep.Overall.Requests != 2 {
		t.Errorf("admitted %d records, want 2", rep.Overall.Requests)
	}
	if rep.Filtered != 1 {
		t.Errorf("filtered = %d, want 1", rep.Filtered)
	}
	closeTo(t, "cost", rep.Overall.Cost.Total, 10)
}

// The same UTC instant lands in different local days, so the daily table and a
// date window both shift with the location.
func TestBuildDayBucketsFollowLocation(t *testing.T) {
	kolkata, err := time.LoadLocation("Asia/Kolkata")
	if err != nil {
		t.Skipf("timezone database unavailable: %v", err)
	}
	// 23:30 UTC on the 13th is 05:00 on the 14th in Kolkata.
	recs := []transcript.Record{
		rec("claude-opus-5", withTime(time.Date(2026, 9, 13, 23, 30, 0, 0, time.UTC))),
	}

	utcRep := build(t, recs, Options{Location: time.UTC})
	if utcRep.ByDay[0].Key != "2026-09-13" {
		t.Errorf("UTC bucket = %s, want 2026-09-13", utcRep.ByDay[0].Key)
	}
	kolRep := build(t, recs, Options{Location: kolkata})
	if kolRep.ByDay[0].Key != "2026-09-14" {
		t.Errorf("Kolkata bucket = %s, want 2026-09-14", kolRep.ByDay[0].Key)
	}
	if kolRep.Location != "Asia/Kolkata" {
		t.Errorf("Location = %q", kolRep.Location)
	}
}

// Undated records are real spend, so they count everywhere except the calendar.
func TestBuildUndatedRecords(t *testing.T) {
	rep := build(t, []transcript.Record{
		rec("claude-opus-5"),
		rec("claude-opus-5", withTime(time.Time{})),
	}, Options{})

	if rep.Overall.Requests != 2 {
		t.Errorf("requests = %d, want 2", rep.Overall.Requests)
	}
	closeTo(t, "cost includes undated", rep.Overall.Cost.Total, 10)
	if rep.Undated != 1 {
		t.Errorf("Undated = %d, want 1", rep.Undated)
	}
	if len(rep.ByDay) != 1 {
		t.Errorf("undated record must not create a day bucket, got %d days", len(rep.ByDay))
	}
	if rep.ByDay[0].Requests != 1 {
		t.Errorf("day bucket requests = %d, want 1", rep.ByDay[0].Requests)
	}
}

func TestBuildUnknownModelKeepsTokensDropsDollars(t *testing.T) {
	rep := build(t, []transcript.Record{
		rec("claude-opus-5"),
		rec("claude-imaginary-9"),
	}, Options{})

	if got := rep.Overall.Tokens.Total(); got != 2e6 {
		t.Errorf("tokens = %d, want 2000000 (unknown model tokens still count)", got)
	}
	closeTo(t, "cost excludes the unknown model", rep.Overall.Cost.Total, 5)
	if rep.Overall.UnpricedTokens != 1e6 {
		t.Errorf("UnpricedTokens = %d, want 1000000", rep.Overall.UnpricedTokens)
	}
	if len(rep.UnknownModels) != 1 || rep.UnknownModels[0] != "claude-imaginary-9" {
		t.Errorf("UnknownModels = %v", rep.UnknownModels)
	}
	if len(rep.Warnings) == 0 {
		t.Error("an unknown model must surface a warning")
	}
}

func TestBuildThinkingNotDoubleCounted(t *testing.T) {
	rep := build(t, []transcript.Record{
		rec("claude-opus-5", withUsage(transcript.Usage{Output: 1e6, Thinking: 4e5})),
	}, Options{})

	if got := rep.Overall.Tokens.Total(); got != 1e6 {
		t.Errorf("total = %d, want 1000000; thinking is already inside output", got)
	}
	if rep.Overall.Tokens.Thinking != 4e5 {
		t.Errorf("thinking should still be reported, got %d", rep.Overall.Tokens.Thinking)
	}
	closeTo(t, "cost", rep.Overall.Cost.Total, 25)
}

func TestBuildTimeBounds(t *testing.T) {
	first := time.Date(2026, 9, 10, 8, 0, 0, 0, time.UTC)
	last := time.Date(2026, 9, 20, 17, 0, 0, 0, time.UTC)
	rep := build(t, []transcript.Record{
		rec("claude-opus-5", withTime(last)),
		rec("claude-opus-5", withTime(first)),
		rec("claude-opus-5", withTime(time.Time{})),
	}, Options{})

	if !rep.Overall.First.Equal(first) || !rep.Overall.Last.Equal(last) {
		t.Errorf("bounds = %s..%s, want %s..%s",
			rep.Overall.First, rep.Overall.Last, first, last)
	}
}

func TestBuildEmpty(t *testing.T) {
	rep := build(t, nil, Options{})
	if rep.Overall.Requests != 0 || rep.Overall.Cost.Total != 0 {
		t.Errorf("empty input produced %+v", rep.Overall)
	}
	for name, groups := range map[string][]Group{
		"models": rep.ByModel, "projects": rep.ByProject,
		"days": rep.ByDay, "sessions": rep.BySession,
	} {
		if len(groups) != 0 {
			t.Errorf("%s = %v, want empty", name, keys(groups))
		}
	}
	if len(rep.UnknownModels) != 0 || len(rep.Warnings) != 0 {
		t.Error("empty input should produce no warnings")
	}
}

func TestBuildEverythingFiltered(t *testing.T) {
	now := time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC)
	w, _ := ParseWindow("2027-01-01", "", now)
	rep := build(t, []transcript.Record{rec("claude-opus-5")}, Options{Window: w})

	if rep.Overall.Requests != 0 {
		t.Errorf("requests = %d, want 0", rep.Overall.Requests)
	}
	if rep.Filtered != 1 {
		t.Errorf("Filtered = %d, want 1", rep.Filtered)
	}
	if len(rep.ByDay) != 0 {
		t.Errorf("want no day buckets, got %v", keys(rep.ByDay))
	}
}

func TestBuildIsDeterministic(t *testing.T) {
	// Equal-cost groups must still order stably, or table output flickers
	// between runs.
	recs := []transcript.Record{
		rec("claude-opus-5", withProject("zebra"), withSession("z")),
		rec("claude-opus-5", withProject("alpha"), withSession("a")),
		rec("claude-opus-5", withProject("mango"), withSession("m")),
	}
	first := keys(build(t, recs, Options{}).ByProject)
	for i := 0; i < 20; i++ {
		got := keys(build(t, recs, Options{}).ByProject)
		for j := range first {
			if got[j] != first[j] {
				t.Fatalf("run %d diverged: %v vs %v", i, got, first)
			}
		}
	}
	if first[0] != "alpha" || first[2] != "zebra" {
		t.Errorf("equal-cost groups should fall back to key order, got %v", first)
	}
}

func TestBuildCostBreakdownSumsToTotal(t *testing.T) {
	rep := build(t, []transcript.Record{
		rec("claude-opus-5", withUsage(transcript.Usage{
			Input: 1e6, Output: 1e6, CacheWrite5m: 1e6,
			CacheWrite1h: 1e6, CacheRead: 1e6, WebSearches: 10,
		})),
	}, Options{})

	c := rep.Overall.Cost
	sum := c.Input + c.Output + c.CacheWrite5m + c.CacheWrite1h + c.CacheRead + c.WebSearch
	closeTo(t, "breakdown sums to total", sum, c.Total)
	closeTo(t, "total", c.Total, 46.75+0.10)
}

func keys(gs []Group) []string {
	out := make([]string, len(gs))
	for i, g := range gs {
		out[i] = g.Key
	}
	return out
}

// Every grouping partitions the same records, so each must sum back to the
// overall totals. This is what catches a record being dropped from, or
// double-counted in, one view but not another.
func TestBuildGroupingsPartitionTheSameRecords(t *testing.T) {
	day := func(d int) time.Time { return time.Date(2026, 9, d, 12, 0, 0, 0, time.UTC) }
	recs := []transcript.Record{
		rec("claude-opus-5", withProject("a"), withSession("s1"), withTime(day(14))),
		rec("claude-sonnet-5", withProject("a"), withSession("s1"), withTime(day(15))),
		rec("claude-haiku-4-5", withProject("b"), withSession("s2"), withTime(day(15)), sidechain()),
		rec("claude-opus-5", withProject("b"), withSession("s3"), withTime(day(16))),
		rec("claude-imaginary-9", withProject("b"), withSession("s3"), withTime(day(16))),
	}
	rep := build(t, recs, Options{})

	for name, groups := range map[string][]Group{
		"model": rep.ByModel, "project": rep.ByProject,
		"day": rep.ByDay, "session": rep.BySession,
	} {
		var cost float64
		var tokens, requests int64
		for _, g := range groups {
			cost += g.Cost.Total
			tokens += g.Tokens.Total()
			requests += g.Requests
		}
		closeTo(t, name+" cost", cost, rep.Overall.Cost.Total)
		if tokens != rep.Overall.Tokens.Total() {
			t.Errorf("%s tokens = %d, want %d", name, tokens, rep.Overall.Tokens.Total())
		}
		if requests != rep.Overall.Requests {
			t.Errorf("%s requests = %d, want %d", name, requests, rep.Overall.Requests)
		}
	}

	// The sidechain split is a partition too.
	closeTo(t, "main+sidechain", rep.Main.Cost.Total+rep.Sidechain.Cost.Total, rep.Overall.Cost.Total)
	if rep.Main.Requests+rep.Sidechain.Requests != rep.Overall.Requests {
		t.Errorf("split requests = %d+%d, want %d",
			rep.Main.Requests, rep.Sidechain.Requests, rep.Overall.Requests)
	}
}

// A window's totals must equal the day rows it covers, or `--since 7d` and a
// seven-row daily table would disagree about the same week.
func TestBuildWindowTotalsMatchTheDayRowsItCovers(t *testing.T) {
	day := func(d int) time.Time { return time.Date(2026, 9, d, 12, 0, 0, 0, time.UTC) }
	var recs []transcript.Record
	for d := 10; d <= 20; d++ {
		recs = append(recs, rec("claude-opus-5", withTime(day(d))))
	}

	now := time.Date(2026, 9, 20, 23, 0, 0, 0, time.UTC)
	w, err := Relative(SpecWeek, now)
	if err != nil {
		t.Fatal(err)
	}
	windowed := build(t, recs, Options{Window: w, Location: time.UTC})
	if len(windowed.ByDay) != 7 {
		t.Fatalf("a 7d window covered %d day rows, want 7", len(windowed.ByDay))
	}

	full := build(t, recs, Options{Location: time.UTC})
	var sum float64
	var requests int64
	for _, g := range full.ByDay {
		if g.Key >= "2026-09-14" {
			sum += g.Cost.Total
			requests += g.Requests
		}
	}
	closeTo(t, "window cost equals its day rows", windowed.Overall.Cost.Total, sum)
	if windowed.Overall.Requests != requests {
		t.Errorf("window requests = %d, want %d", windowed.Overall.Requests, requests)
	}
}
