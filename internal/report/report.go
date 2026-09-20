package report

import (
	"sort"
	"time"

	"cca/internal/pricing"
	"cca/internal/transcript"
)

// Tokens is a summable token count. Thinking is carried for display only; it is
// already inside Output and is never part of Total.
type Tokens struct {
	Input        int64
	Output       int64
	CacheWrite5m int64
	CacheWrite1h int64
	CacheRead    int64
	Thinking     int64
	WebSearches  int64
}

func (t Tokens) Total() int64 {
	return t.Input + t.Output + t.CacheWrite5m + t.CacheWrite1h + t.CacheRead
}

func (t *Tokens) add(u transcript.Usage) {
	t.Input += u.Input
	t.Output += u.Output
	t.CacheWrite5m += u.CacheWrite5m
	t.CacheWrite1h += u.CacheWrite1h
	t.CacheRead += u.CacheRead
	t.Thinking += u.Thinking
	t.WebSearches += u.WebSearches
}

// Cost is a summable money breakdown. It drops the per-request fields of
// pricing.Cost, which describe one request and do not aggregate.
type Cost struct {
	Input        float64
	Output       float64
	CacheWrite5m float64
	CacheWrite1h float64
	CacheRead    float64
	WebSearch    float64
	Total        float64
}

func (c *Cost) add(p pricing.Cost) {
	c.Input += p.Input
	c.Output += p.Output
	c.CacheWrite5m += p.CacheWrite5m
	c.CacheWrite1h += p.CacheWrite1h
	c.CacheRead += p.CacheRead
	c.WebSearch += p.WebSearch
	c.Total += p.Total
}

// Group is one aggregated bucket: a model, a project, a day, a session, or the
// run as a whole.
type Group struct {
	Key      string
	Tokens   Tokens
	Cost     Cost
	Requests int64

	// Sessions and Projects count distinct ids within the group.
	Sessions int
	Projects int

	// First and Last bound the group in time, ignoring undated records.
	First time.Time
	Last  time.Time

	// Project labels a session group with the directory it ran in.
	Project string

	// UnpricedTokens counts tokens whose model had no rate row. They are real
	// usage, so they stay in Tokens; only their dollars are missing from Cost.
	UnpricedTokens int64

	sessionIDs map[string]struct{}
	projectIDs map[string]struct{}
}

func (g *Group) observe(r transcript.Record, c pricing.Cost) {
	g.Tokens.add(r.Usage)
	g.Cost.add(c)
	g.Requests++
	if !c.Priced {
		g.UnpricedTokens += r.Usage.Total()
	}
	if !r.Timestamp.IsZero() {
		if g.First.IsZero() || r.Timestamp.Before(g.First) {
			g.First = r.Timestamp
		}
		if g.Last.IsZero() || r.Timestamp.After(g.Last) {
			g.Last = r.Timestamp
		}
	}
	if r.SessionID != "" {
		if g.sessionIDs == nil {
			g.sessionIDs = make(map[string]struct{})
		}
		g.sessionIDs[r.SessionID] = struct{}{}
	}
	if r.Project != "" {
		if g.projectIDs == nil {
			g.projectIDs = make(map[string]struct{})
		}
		g.projectIDs[r.Project] = struct{}{}
	}
}

func (g *Group) seal() {
	g.Sessions = len(g.sessionIDs)
	g.Projects = len(g.projectIDs)
	g.sessionIDs, g.projectIDs = nil, nil
}

// Options configures a build. Location decides which calendar day a record
// falls in and defaults to the system zone.
type Options struct {
	Window   Window
	Location *time.Location
	// TopSessions caps BySession. Zero keeps every session.
	TopSessions int
}

// Report is the aggregated view of a run, ready for rendering.
type Report struct {
	Window   Window
	Location string

	Overall Group

	ByModel   []Group
	ByProject []Group
	ByDay     []Group
	BySession []Group

	// Main and Sidechain split the overall totals by origin. Sub-agent usage is
	// real spend and is included by default, but it is worth showing apart.
	Main      Group
	Sidechain Group

	// Undated counts admitted records whose timestamp did not parse. They
	// contribute to totals but cannot appear in the per-day table.
	Undated int64
	// Filtered counts records excluded by the window.
	Filtered int64

	UnknownModels []string
	Warnings      []pricing.Warning
	Stats         transcript.Stats
}

// Build aggregates records into a report. It is pure: no I/O, and the same
// inputs always produce the same output, including slice ordering.
func Build(recs []transcript.Record, calc *pricing.Calculator, opts Options) *Report {
	loc := opts.Location
	if loc == nil {
		loc = time.Local
	}
	rep := &Report{Window: opts.Window, Location: loc.String()}

	byModel := map[string]*Group{}
	byProject := map[string]*Group{}
	byDay := map[string]*Group{}
	bySession := map[string]*Group{}

	for _, r := range recs {
		if !opts.Window.Contains(r.Timestamp) {
			rep.Filtered++
			continue
		}
		cost := calc.Cost(pricing.Request{
			Model:        r.Model,
			Speed:        r.Speed,
			ServiceTier:  r.ServiceTier,
			InferenceGeo: r.InferenceGeo,
			Tokens: pricing.Tokens{
				Input:        r.Usage.Input,
				Output:       r.Usage.Output,
				CacheWrite5m: r.Usage.CacheWrite5m,
				CacheWrite1h: r.Usage.CacheWrite1h,
				CacheRead:    r.Usage.CacheRead,
				WebSearches:  r.Usage.WebSearches,
			},
		})

		rep.Overall.observe(r, cost)
		if r.IsSidechain {
			rep.Sidechain.observe(r, cost)
		} else {
			rep.Main.observe(r, cost)
		}
		bucket(byModel, r.Model).observe(r, cost)
		bucket(byProject, r.Project).observe(r, cost)

		session := bucket(bySession, r.SessionID)
		session.Project = r.Project
		session.observe(r, cost)

		// Undated records join every other grouping but cannot be placed on a
		// calendar, so they are counted rather than bucketed into a wrong day.
		if r.Timestamp.IsZero() {
			rep.Undated++
		} else {
			bucket(byDay, DayKey(r.Timestamp, loc)).observe(r, cost)
		}
	}

	rep.Overall.seal()
	rep.Main.seal()
	rep.Sidechain.seal()

	rep.ByModel = sealed(byModel, byCostDesc)
	rep.ByProject = sealed(byProject, byCostDesc)
	rep.ByDay = sealed(byDay, byKeyAsc) // chronological, newest last
	rep.BySession = sealed(bySession, byCostDesc)
	if opts.TopSessions > 0 && len(rep.BySession) > opts.TopSessions {
		rep.BySession = rep.BySession[:opts.TopSessions]
	}

	rep.UnknownModels = calc.UnknownModels()
	rep.Warnings = calc.Warnings()
	return rep
}

func bucket(m map[string]*Group, key string) *Group {
	g, ok := m[key]
	if !ok {
		g = &Group{Key: key}
		m[key] = g
	}
	return g
}

func sealed(m map[string]*Group, less func(a, b Group) bool) []Group {
	out := make([]Group, 0, len(m))
	for _, g := range m {
		g.seal()
		out = append(out, *g)
	}
	sort.Slice(out, func(i, j int) bool { return less(out[i], out[j]) })
	return out
}

// byCostDesc ranks by spend, falling back to the key so that ties and
// all-zero-cost runs still order deterministically.
func byCostDesc(a, b Group) bool {
	if a.Cost.Total != b.Cost.Total {
		return a.Cost.Total > b.Cost.Total
	}
	if a.Tokens.Total() != b.Tokens.Total() {
		return a.Tokens.Total() > b.Tokens.Total()
	}
	return a.Key < b.Key
}

func byKeyAsc(a, b Group) bool { return a.Key < b.Key }
