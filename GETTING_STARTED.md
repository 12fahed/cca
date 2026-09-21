# Getting started with the cca codebase

A practical tour for someone who has just cloned the repository and wants to
change something. For *using* the tool, see the
[documentation](https://12fahed.github.io/cca) instead.

[CONTRIBUTING.md](CONTRIBUTING.md) covers the rules; this covers the mechanics.

## 1. Build it

You need **Go 1.27 or newer** and nothing else.

```sh
git clone https://github.com/12fahed/cca.git
cd cca
make build
./cca version
```

If `make` is unavailable:

```sh
go build -o cca ./cmd/cca
```

The `make` target additionally injects the version, commit and build date, so
`./cca version` shows `dev` without it. That is fine for development.

## 2. Run it against your own data

```sh
./cca
./cca --verbose
```

If you have never run Claude Code, there will be nothing to read. Point it at a
fixture instead — see step 5.

## 3. Run the tests

```sh
go run test/run.go
```

That is formatting, `go vet`, a build, the full test suite, and the race
detector. It is what CI runs, so if it passes locally it should pass there.

```sh
go run test/run.go -short   # skip the race detector, roughly twice as fast
go run test/run.go -cross   # also build all five release targets
go test ./internal/pricing/ -run TestOracle -v   # one package, one test
```

## 4. Find your way around

```
cmd/cca/              flag parsing, dispatch, exit codes — no business logic
internal/
  transcript/         discovery, streaming parse, deduplication
  pricing/            rate table, model resolution, cost math
  report/             aggregation, time windows
  render/             tables, JSON, CSV, colour
  config/             path discovery, flag-over-file-over-default
  water/              the estimate and its equivalences
test/
  run.go              the single entry point above
  unit/               cross-package pipeline tests
docs/                 the Starlight documentation site
```

Data flows one way, and each stage is independently testable:

```
~/.claude/**/*.jsonl
      │
      ▼
 transcript.Load()          parse, deduplicate, attribute
      │  []transcript.Record
      ▼
 pricing.Calculator.Cost()  resolve model → rate row → dollars
      │  pricing.Cost
      ▼
 report.Build()             group by model, project, day, session
      │  *report.Report
      ▼
 render.Summary() / JSON() / CSV()
```

`cmd/cca/main.go` wires those four calls together and does nothing else.

### Where to look first

| If you want to change… | Start in |
| --- | --- |
| How a file is found or parsed | `internal/transcript/discover.go`, `parse.go` |
| Deduplication | `internal/transcript/parse.go` — `dedupKey` |
| A price, or a new model | `internal/pricing/pricing.json` |
| How a model id maps to a rate | `internal/pricing/resolve.go` |
| The cost arithmetic | `internal/pricing/cost.go` |
| What `--since 7d` means | `internal/report/window.go` |
| Table layout or colour | `internal/render/table.go`, `color.go` |
| A new command or flag | `cmd/cca/main.go` |

## 5. Work without touching your real data

No test may read your real `~/.claude`, and neither should you while
developing. Build a fixture:

```sh
mkdir -p /tmp/fake-claude/projects/demo
cat > /tmp/fake-claude/projects/demo/session.jsonl <<'EOF'
{"type":"assistant","uuid":"u1","sessionId":"s1","requestId":"r1","timestamp":"2026-09-14T10:00:00.000Z","cwd":"/demo","isSidechain":false,"message":{"id":"m1","model":"claude-opus-5","usage":{"input_tokens":1000,"output_tokens":2000,"cache_creation_input_tokens":5000,"cache_read_input_tokens":90000,"cache_creation":{"ephemeral_5m_input_tokens":1000,"ephemeral_1h_input_tokens":4000}}}}
EOF

./cca --claude-dir /tmp/fake-claude
./cca --claude-dir /tmp/fake-claude --json | jq .totals
```

Duplicate that line to watch deduplication work — the record count stays at one
while `--verbose` reports a duplicate.

## 6. Make a change

Three worked examples, easiest first.

### Add a model rate

Data only, no logic.

1. Add an entry to `internal/pricing/pricing.json`:

   ```json
   {
     "id": "claude-example-6",
     "name": "Claude Example 6",
     "rates": {
       "input": 4.00,
       "output": 20.00,
       "cache_write_5m": 5.00,
       "cache_write_1h": 8.00,
       "cache_read": 0.40
     }
   }
   ```

2. Pin it in `internal/pricing/table_test.go`, in
   `TestEmbeddedRatesArePinned`. That test exists because a silent edit to a
   rate changes every figure the tool reports.

3. If the id is a prefix of another, or vice versa, add a case to
   `TestResolve`.

4. `go test ./internal/pricing/`

### Add a column to a table

1. `internal/render/views.go` — add the header and the cell.
2. Colour it with the palette method that matches its **meaning**
   (`p.Tokens`, `p.Cost`, `p.Muted`), not a colour name.
3. Regenerate goldens and **read the diff**:

   ```sh
   go test ./internal/render -update
   git diff internal/render/testdata/
   ```

4. Check the width test still passes — the default view is meant to fit 80
   columns.

### Add a command

1. Add a view function in `internal/render/views.go`.
2. Register it in the `commands` table in `cmd/cca/main.go`.
3. Add a CSV case in `internal/render/csv.go` if it needs different columns.
4. `TestHelpListsOnlyWorkingCommands` will now exercise it automatically — it
   runs every command in the help listing and fails if any errors.

## 7. Traps worth knowing before you hit them

These cost real time to discover. They are commented in the code, but knowing
they exist helps.

**Deduplication is half the data.** The same message is replayed into several
files by resumed sessions and compaction. If a number looks doubled, check
`--verbose` first.

**Sub-agent transcripts nest deeper.** `projects/<slug>/<session>/subagents/`.
A one-level walk silently drops every sidechain record and most Haiku usage
while still producing a confident total.

**`tabwriter` cannot align coloured cells.** Its escape mechanism counts a
bracketed segment as width *one*, not zero. That is why `internal/render`
measures visible width itself. Do not reintroduce `tabwriter` for styled output.

**Thinking tokens are already inside output tokens.** Never add them.

**Prefix matching needs the dated-suffix rule.** `claude-opus-4` is
`$15/$75`; `claude-opus-4-8` is `$5/$25`. A bare longest-prefix match would let
a future `claude-opus-4-9` inherit the retired model's triple rate.

**Timestamps are UTC, days are local.** Bucketing without converting puts a
23:30 UTC turn on the wrong day for most of the world.

## 8. Documentation

```sh
cd docs
npm install
npm run dev      # http://localhost:4321/cca
npm run build    # verify before pushing
```

The rate table page is generated from `pricing.json`, so adding a model updates
the docs automatically.

## 9. Before you open a pull request

```sh
go run test/run.go
```

Then read [CONTRIBUTING.md](CONTRIBUTING.md) — particularly the constraints
that are not negotiable, since they are the most common reason a change is
declined.

## Stuck?

Open a [discussion](https://github.com/12fahed/cca/discussions). If something
here was unclear, that is a documentation bug worth reporting on its own.
