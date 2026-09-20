# cca

Analyze your local Claude Code usage: tokens, what it would have cost at API list
prices, and — for fun — roughly how much water it took.

```
  Claude Code usage · all time · 13 sessions across 3 projects

  Tokens   input  output  cache 5m  cache 1h  cache read
          144.5K  673.5K    503.0K      6.6M       52.0M

  Model                      tokens    cost
  claude-opus-5               20.3M  $37.87
  claude-sonnet-5             20.3M  $15.97
  claude-haiku-4-5-20251001   19.4M   $6.99

  Cost    $60.82     at API list prices
  Water   ≈ 18.0 L   about 2 minutes of shower

  ─ Cost is what this usage would cost at API rates, not what you were billed;
    Claude Code on a subscription draws from your plan allowance instead.
  ─ Water assumes 0.30 mL / 1k tokens, a rough estimate. See README.
```

**The dollar figure is not a bill.** Claude Code on a Pro, Max, or Team plan does not
charge per token — it draws from a plan allowance. What `cca` reports is what the same
usage *would* have cost at published API rates. That is a useful gauge of how heavily
you lean on the tool. It is not an invoice, and it will not match one.

`cca` reads local files only, sends nothing anywhere, works entirely offline, and never
writes to `~/.claude`.

## Install

### Build from source

Requires Go 1.27 or newer.

```sh
git clone https://github.com/12fahed/cca.git
cd cca
make build      # produces ./cca
```

Put it on your `PATH`:

```sh
sudo install -m 0755 cca /usr/local/bin/cca     # macOS, Linux
```

On Windows, copy `cca.exe` somewhere on your `PATH`.

### Download a binary

Prebuilt archives for macOS (arm64, amd64), Linux (amd64, arm64) and Windows (amd64) are
attached to each [release](https://github.com/12fahed/cca/releases), with a
`checksums.txt` alongside them. Each is a single static binary with no runtime
dependencies.

### Not `go install`

The module path is `cca` rather than a domain-qualified path, so
`go install cca/cmd/cca@latest` cannot resolve. Clone and build instead.

## Usage

```sh
cca                          # all-time summary (the default)
cca today                    # since local midnight
cca week                     # the last 7 days
cca month                    # the last month
cca models                   # breakdown by model
cca projects                 # breakdown by project directory
cca daily                    # per-day table, newest last
cca sessions --top 10        # the most expensive sessions
cca config                   # resolved settings and where they came from
cca version
```

Time ranges apply to any usage view:

```sh
cca --since 7d                              # relative: Nd, Nw, Nm
cca --since 2026-09-01 --until 2026-09-14   # inclusive of both days
```

`--since 7d` means the last seven calendar days **including today**, so it lines up with
a seven-row `cca daily` table. `--until` includes the whole day you name.

### Flags

| Flag | Effect |
|---|---|
| `--json` | machine-readable output; exact token counts, full session ids |
| `--csv` | CSV rows for the current view |
| `--since`, `--until` | limit the time window |
| `--top N` | rows in the `sessions` view (default 10) |
| `--no-sidechains` | exclude sub-agent usage |
| `--claude-dir` | read transcripts from somewhere other than `~/.claude` |
| `--pricing` | use an alternate `pricing.json` |
| `--water-ml-per-1k` | override the water constant |
| `--verbose` | show files scanned, records kept, duplicates dropped, warnings |
| `--no-color` | disable styling (`NO_COLOR` is honoured too) |
| `--ascii` | plain ASCII for terminals that mangle `·`, `≈` and `─` |

### Examples

```
$ cca projects

  Claude Code usage · by project · all time

  Project                  sessions  requests  tokens    cost  share
  -home-dev-payments-api          4        95   17.6M  $25.51  41.9%
  -home-dev-infra-scripts         4       117   20.2M  $20.88  34.3%
  -home-dev-web-dashboard         5       128   22.1M  $14.43  23.7%

  Total                          13       340   60.0M  $60.82
```

```
$ cca sessions --top 3

  Session   started     project                  requests  tokens    cost
  070d7109  2026-09-15  -home-dev-payments-api         38    7.1M  $11.09
  7a3a8394  2026-09-08  -home-dev-infra-scripts        35    7.6M   $7.46
  b82763ba  2026-09-07  -home-dev-infra-scripts        41    6.7M   $6.05
```

Project names are the slugified working directories Claude Code stores transcripts under;
long ones are shortened from the left, since the tail is what distinguishes them. Session
ids are shortened for the table and appear in full in `--json` and `--csv`.

```
$ cca --verbose | tail -6

  Scan · 18 files, 450 lines
                            count
  usage records kept          340
  duplicates dropped  110 (24.4%)
```

## Configuration

`cca` works with no configuration. To change a default, create `config.json` at:

- `~/.config/cca/config.json` (or `$XDG_CONFIG_HOME/cca/config.json`)
- `%APPDATA%\cca\config.json` on Windows

```json
{
  "water_ml_per_1k_tokens": 0.30,
  "claude_dir": "",
  "pricing": "",
  "no_sidechains": false,
  "ascii": false,
  "no_color": false
}
```

Every key is optional, and a command-line flag always wins over the file. `cca config`
shows the resolved values **and which layer supplied each one**, which is the quickest way
to work out why `cca` is behaving as it is.

## How the cost is calculated

Claude Code writes one JSON object per line into
`~/.claude/projects/<slug>/<session>.jsonl`, and sub-agent sessions into
`<session>/subagents/agent-*.jsonl`. `cca` streams those files, keeps the assistant
records that carry token accounting, and prices them.

Three details do most of the work:

**Deduplication.** The same assistant message is replayed into several files by resumed
sessions and by compaction. On real corpora this is routinely **half the records** — the
corpus this tool was developed against was 54% duplicates. Without deduplication every
total would be roughly double. `cca` keys on the message id plus request id, falling back
to the record uuid, across all files at once.

**Cache write TTLs.** Cached input is billed differently depending on whether it was
written with a 5-minute or a 1-hour lifetime — 1.25x and 2x the base input rate. The two
are reported separately because the split matters: the development corpus was 96%
1-hour writes, so collapsing them would have understated cache-write cost by about a third.

**Absolute rates, not multipliers.** [`internal/pricing/pricing.json`](internal/pricing/pricing.json)
stores USD per million tokens for all five billed classes of every model, mirroring the
published price sheet. That makes each number checkable by eye and removes the arithmetic
that per-model exceptions would otherwise need.

Fast mode, `us`-pinned inference (+10%), and the batch tier are applied where the
transcript records them. Web search is billed per request; web fetch is not.

### Overriding the rates

Rates change. Drop a `pricing.json` beside your `config.json`, or pass `--pricing PATH`,
and it **replaces** the embedded table entirely — a price change is a data edit, never a
rebuild. Start by copying
[`internal/pricing/pricing.json`](internal/pricing/pricing.json).

If a model has no rate, `cca` counts its tokens, leaves it out of the dollar total, and
says so in a footnote naming the model. It never guesses a rate.

## The water estimate

This is the playful part, and the part where being honest matters most.

**There is no reliable public per-token water figure for any model.** Published estimates
vary by more than an order of magnitude depending on datacenter location, cooling design,
power mix, and whether the figure counts only on-site evaporation or also the water used
to generate the electricity. Anyone quoting a precise number is extrapolating.

So `cca` does the least-bad thing: one constant, `0.30 mL per 1,000 tokens`, applied flat
across every model and token class, with the assumption printed next to the result every
single time. It is a **placeholder, not a measurement**. Scaling it per model would be
false precision dressed up as rigour.

If you have a figure you trust more, use it:

```sh
cca --water-ml-per-1k 0.12
```

or set `water_ml_per_1k_tokens` in `config.json`.

## Accuracy and limitations

Worth knowing before you quote a number from this tool.

- **The transcript format is undocumented and internal to Claude Code.** It changes without
  notice — the development corpus alone contained files written by versions 2.1.178,
  2.1.247 and 2.1.270. `cca` parses tolerantly, skips what it cannot read, and counts the
  skips under `--verbose`. A future version could still break it.
- **Totals can go *down* between runs.** Claude Code prunes and rotates transcripts, so
  history disappears over time. `cca` reports what is on disk now; it is not an
  append-only ledger.
- **A transcript is not a guaranteed-complete record of billed turns.** Claude Code's own
  per-session accounting has been observed to include a request whose record was never
  written to the transcript. `cca` can only count what it can read, so a session's figure
  can be an undercount.
- **Web search cost is an upper bound.** Failed searches are not billed, but the transcript
  does not mark them.
- **Raw token counts are not comparable across model generations.** Claude 4.7 and later
  use a tokenizer that produces roughly 30% more tokens for the same text. Costs are
  unaffected, since real tokens are counted — but a token column comparing an Opus 4.8 row
  against a Haiku 4.5 row is not measuring the same thing.
- **Sub-agent usage is included by default**, tagged separately, and excludable with
  `--no-sidechains`. It is real spend.

## Privacy

`cca` reads `~/.claude` and nothing else. It makes **no network calls at any point** —
the rate table is compiled into the binary — and it **never writes to, moves, or deletes
anything under `~/.claude`**. Transcripts contain your source code and your conversations,
so that restriction is enforced by tests: the directory is hashed before and after a run
and must come back byte-identical, and the parser is checked for any file-writing call at
all.

Nothing is sent anywhere. There is no telemetry.

## Development

```sh
go run test/run.go          # fmt, vet, build, tests, race detector
go run test/run.go -cross   # also build all five release targets
make build-all              # write those binaries to dist/
```

See [test/README.md](test/README.md) for how the suite is laid out and why unit tests live
beside their packages.
