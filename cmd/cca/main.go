// Command cca reports Claude Code token usage, cost at API list prices, and a
// rough water estimate, computed from local transcripts under ~/.claude.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"runtime"
	"strconv"
	"text/tabwriter"
	"time"

	"cca/internal/config"
	"cca/internal/pricing"
	"cca/internal/render"
	"cca/internal/report"
	"cca/internal/transcript"
	"cca/internal/water"
)

// Injected at build time via -ldflags -X; see the Makefile.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

const (
	exitOK    = 0
	exitError = 1
	exitUsage = 2
)

type options struct {
	since        string
	until        string
	claudeDir    string
	pricingPath  string
	waterMLPer1k optionalFloat
	top          int
	json         bool
	csv          bool
	noColor      bool
	noSidechains bool
	verbose      bool
	ascii        bool

	// set records which flags the user actually passed. A bool flag left alone
	// is indistinguishable from one passed as false, so without this the config
	// file could never be overridden back to a default.
	set map[string]bool
}

// markSet records the flags seen so far. It is called after each parse pass,
// since flags may sit on either side of the subcommand.
func (o *options) markSet(fs *flag.FlagSet) {
	if o.set == nil {
		o.set = make(map[string]bool)
	}
	fs.Visit(func(f *flag.Flag) { o.set[f.Name] = true })
}

// overrides translates the flags that were passed into config overrides,
// leaving the rest nil so the config file and defaults can supply them.
func (o *options) overrides() config.Overrides {
	var ov config.Overrides
	if o.waterMLPer1k.set {
		ov.WaterMLPer1k = &o.waterMLPer1k.value
	}
	if o.set["claude-dir"] {
		ov.ClaudeDir = &o.claudeDir
	}
	if o.set["pricing"] {
		ov.Pricing = &o.pricingPath
	}
	if o.set["no-sidechains"] {
		ov.NoSidechains = &o.noSidechains
	}
	if o.set["ascii"] {
		ov.ASCII = &o.ascii
	}
	if o.set["no-color"] {
		ov.NoColor = &o.noColor
	}
	return ov
}

// resolve loads the config file and applies flag over file over default.
func (o *options) resolve() (config.Resolved, error) {
	path, err := config.ConfigPath()
	if err != nil {
		return config.Resolved{}, err
	}
	file, found, err := config.LoadFile(path)
	if err != nil {
		return config.Resolved{}, err
	}
	return config.Resolve(o.overrides(), file, found, path)
}

func (o *options) register(fs *flag.FlagSet) {
	fs.StringVar(&o.since, "since", "", "start of window: 7d, 2w, 3m, or 2026-09-01")
	fs.StringVar(&o.until, "until", "", "end of window: 2026-09-14")
	fs.StringVar(&o.claudeDir, "claude-dir", "", "override the ~/.claude location")
	fs.StringVar(&o.pricingPath, "pricing", "", "path to an alternate pricing.json")
	fs.Var(&o.waterMLPer1k, "water-ml-per-1k",
		"water constant in mL per 1k tokens (default 0.30 — a rough placeholder, not a measured value)")
	fs.IntVar(&o.top, "top", 10, "number of rows for the sessions view")
	fs.BoolVar(&o.json, "json", false, "machine-readable output")
	fs.BoolVar(&o.csv, "csv", false, "CSV output for the current view")
	fs.BoolVar(&o.noColor, "no-color", false, "disable styling (NO_COLOR is also honored)")
	fs.BoolVar(&o.noSidechains, "no-sidechains", false, "exclude sub-agent usage")
	fs.BoolVar(&o.verbose, "verbose", false, "show files scanned, skipped lines, dedup stats")
	fs.BoolVar(&o.ascii, "ascii", false, "ASCII-only output for terminals that mangle Unicode")
}

// optionalFloat separates "flag absent" from "flag set", so that config-file
// resolution can apply its own default rather than being overridden by the flag
// package's zero value. An unset value renders as "" so that PrintDefaults omits
// a misleading sentinel default.
type optionalFloat struct {
	value float64
	set   bool
}

func (f *optionalFloat) String() string {
	if f == nil || !f.set {
		return ""
	}
	return strconv.FormatFloat(f.value, 'g', -1, 64)
}

func (f *optionalFloat) Set(s string) error {
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return err
	}
	f.value, f.set = v, true
	return nil
}

type command struct {
	name    string
	summary string
	run     func(*options, io.Writer) error
}

var commands = []command{
	{"summary", "all-time usage summary (default)", view("", render.ViewSummary, render.Summary)},
	{"today", "usage since local midnight", view(report.SpecToday, render.ViewSummary, render.Summary)},
	{"week", "usage over the last 7 days", view(report.SpecWeek, render.ViewSummary, render.Summary)},
	{"month", "usage over the last month", view(report.SpecMonth, render.ViewSummary, render.Summary)},
	{"models", "breakdown by model", view("", render.ViewModels, render.Models)},
	{"projects", "breakdown by project directory", view("", render.ViewProjects, render.Projects)},
	{"daily", "per-day table, newest last", view("", render.ViewDaily, render.Daily)},
	{"sessions", "most expensive sessions", runSessions},
	{"config", "show resolved config and file paths", runConfig},
	{"version", "version, commit, and build date", runVersion},
	{"help", "show this help", nil},
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	opts := &options{}
	fs := flag.NewFlagSet("cca", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() { usage(stderr, fs) }
	opts.register(fs)

	// Two passes so that flags may sit on either side of the subcommand:
	// flag.Parse stops at the first non-flag argument, so the first pass
	// consumes "cca --json models" and the second "cca models --json".
	if err := fs.Parse(args); err != nil {
		return usageExit(err)
	}
	opts.markSet(fs)
	name := "summary"
	if rest := fs.Args(); len(rest) > 0 {
		name = rest[0]
		if err := fs.Parse(rest[1:]); err != nil {
			return usageExit(err)
		}
		opts.markSet(fs)
		if extra := fs.Args(); len(extra) > 0 {
			fmt.Fprintf(stderr, "cca: unexpected argument %q\n", extra[0])
			return exitUsage
		}
	}

	if name == "help" {
		usage(stdout, fs)
		return exitOK
	}
	cmd := lookup(name)
	if cmd == nil {
		fmt.Fprintf(stderr, "cca: unknown command %q\nRun 'cca help' for usage.\n", name)
		return exitUsage
	}
	if opts.json && opts.csv {
		fmt.Fprintln(stderr, "cca: --json and --csv are mutually exclusive")
		return exitUsage
	}

	if err := cmd.run(opts, stdout); err != nil {
		fmt.Fprintf(stderr, "cca: %v\n", err)
		return exitError
	}
	return exitOK
}

func lookup(name string) *command {
	for i := range commands {
		if commands[i].name == name && commands[i].run != nil {
			return &commands[i]
		}
	}
	return nil
}

func usageExit(err error) int {
	if errors.Is(err, flag.ErrHelp) {
		return exitOK
	}
	return exitUsage
}

// viewFunc renders a built report.
type viewFunc func(io.Writer, *report.Report, render.Options) error

// runView is the shared pipeline behind every usage command: resolve settings,
// discover transcripts, price them, aggregate, render. The logic lives in the
// internal packages; this only connects them.
//
// spec names a preset window for the bare today/week/month commands and is
// empty when the window comes from --since and --until.
func runView(o *options, out io.Writer, spec, name string, view viewFunc, topSessions int) error {
	cfg, err := o.resolve()
	if err != nil {
		return err
	}

	window, err := o.window(spec)
	if err != nil {
		return err
	}

	table, err := pricing.LoadWithFallback(cfg.PricingPath)
	if err != nil {
		return err
	}

	records, stats, err := transcript.Load(transcript.Options{
		ClaudeDir:         cfg.ClaudeDir,
		NonModels:         table.NonModelSet(),
		ExcludeSidechains: cfg.NoSidechains,
	})
	if err != nil {
		return explainLoad(err, cfg.ClaudeDir)
	}

	rep := report.Build(records, pricing.NewCalculator(table), report.Options{
		Window:      window,
		TopSessions: topSessions,
		Stats:       stats,
	})

	ropts := render.Options{
		ASCII: cfg.ASCII,
		Water: water.For(rep.Overall.Tokens.Total(), cfg.WaterMLPer1k),
		View:  name,
		Color: render.NewPalette(render.ColorOptions{Out: out, Disabled: cfg.NoColor}),
	}

	switch {
	case o.json:
		// Machine-readable output is never styled or abbreviated.
		return render.JSON(out, rep, ropts)
	case o.csv:
		return render.CSV(out, rep, ropts)
	}

	if err := view(out, rep, ropts); err != nil {
		return err
	}
	if o.verbose {
		return render.Diagnostics(out, rep, ropts)
	}
	return nil
}

// window resolves the time range, refusing a combination that cannot be
// honoured rather than silently picking one side.
func (o *options) window(spec string) (report.Window, error) {
	now := time.Now()
	if spec == "" {
		return report.ParseWindow(o.since, o.until, now)
	}
	if o.since != "" || o.until != "" {
		return report.Window{}, fmt.Errorf(
			"this command already sets its own time range; drop --since/--until " +
				"or use the default view with them instead")
	}
	return report.Relative(spec, now)
}

// explainLoad turns a discovery failure into something actionable. A missing
// directory is a normal situation for a new user, not a malfunction.
func explainLoad(err error, dir string) error {
	if !errors.Is(err, transcript.ErrNoClaudeDir) {
		return err
	}
	if info, statErr := os.Stat(dir); statErr == nil && info.IsDir() {
		return fmt.Errorf("%s exists but holds no transcripts yet.\n"+
			"Run Claude Code at least once, or point cca elsewhere with --claude-dir", dir)
	}
	return fmt.Errorf("%w\nIf Claude Code stores its data elsewhere, "+
		"point cca at it with --claude-dir", err)
}

// view builds a command handler for one of the report views.
func view(spec, name string, f viewFunc) func(*options, io.Writer) error {
	return func(o *options, out io.Writer) error { return runView(o, out, spec, name, f, 0) }
}

func runSessions(o *options, out io.Writer) error {
	return runView(o, out, "", render.ViewSessions, render.Sessions, o.top)
}

// runConfig reports the resolved settings without touching any transcripts, so
// it still works when the Claude directory is missing.
func runConfig(o *options, out io.Writer) error {
	cfg, err := o.resolve()
	if err != nil {
		return err
	}
	origin := "embedded default"
	if table, err := pricing.LoadWithFallback(cfg.PricingPath); err == nil && cfg.PricingPath != "" {
		origin = "override (" + table.VerifiedOn + ")"
	}
	return render.Config(out, cfg, origin, render.Options{
		ASCII: cfg.ASCII,
		View:  render.ViewConfig,
		Color: render.NewPalette(render.ColorOptions{Out: out, Disabled: cfg.NoColor}),
	})
}

func runVersion(_ *options, out io.Writer) error {
	w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "cca\t%s\n", version)
	fmt.Fprintf(w, "commit\t%s\n", commit)
	fmt.Fprintf(w, "built\t%s\n", date)
	fmt.Fprintf(w, "go\t%s\n", runtime.Version())
	fmt.Fprintf(w, "platform\t%s/%s\n", runtime.GOOS, runtime.GOARCH)
	return w.Flush()
}

func usage(out io.Writer, fs *flag.FlagSet) {
	fmt.Fprint(out, "cca — analyze local Claude Code usage\n\nUsage:\n  cca [command] [flags]\n\nCommands:\n")
	w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	for _, c := range commands {
		fmt.Fprintf(w, "  %s\t%s\n", c.name, c.summary)
	}
	w.Flush()
	fmt.Fprint(out, "\nFlags:\n")
	prev := fs.Output()
	fs.SetOutput(out)
	fs.PrintDefaults()
	fs.SetOutput(prev)
}
