<div align="center">

<img src="docs/public/cca-logo.svg" alt="cca logo" width="128" height="128">

# cca

**See what Claude Code actually costs you — tokens, dollars, and a glass of water.**

[![ci](https://github.com/12fahed/cca/actions/workflows/ci.yml/badge.svg)](https://github.com/12fahed/cca/actions/workflows/ci.yml)
[![docs](https://img.shields.io/badge/docs-12fahed.github.io%2Fcca-c2613c)](https://12fahed.github.io/cca)
[![release](https://img.shields.io/github/v/release/12fahed/cca?display_name=tag&sort=semver)](https://github.com/12fahed/cca/releases)
[![license](https://img.shields.io/badge/license-GPL--3.0-blue)](LICENSE)
[![go](https://img.shields.io/badge/go-1.27%2B-00ADD8?logo=go&logoColor=white)](https://go.dev)
[![platforms](https://img.shields.io/badge/platforms-macOS%20%7C%20Linux%20%7C%20Windows-lightgrey)](https://github.com/12fahed/cca/releases)

<img src="docs/public/banner.png" alt="cca summarising all-time Claude Code usage in a terminal" width="860">

[**Documentation**](https://12fahed.github.io/cca) ·
[Installation](https://12fahed.github.io/cca/start/installation/) ·
[Commands](https://12fahed.github.io/cca/reference/commands/) ·
[Contributing](CONTRIBUTING.md)

</div>

---

`cca` reads the transcripts Claude Code already writes to your machine and tells you how
many tokens you have burned, what that would have cost at API list prices, and — for fun
— roughly how much water it implies.

It runs entirely offline, reads one directory, and never writes to it.

> [!IMPORTANT]
> **The dollar figure is not a bill.** Claude Code on a Pro, Max, or Team plan does not
> charge per token — it draws from a plan allowance. What `cca` reports is what the same
> usage *would* have cost at published API rates. That is a useful gauge of how heavily
> you lean on the tool. It is not an invoice, and it will not match one.

## Install

**Build from source** — requires Go 1.27 or newer:

```sh
git clone https://github.com/12fahed/cca.git
cd cca
make build
sudo install -m 0755 cca /usr/local/bin/cca   # macOS, Linux
```

**Or download a binary** for macOS (arm64/amd64), Linux (amd64/arm64) or Windows (amd64)
from the [releases page](https://github.com/12fahed/cca/releases). Each is a single
static binary with no runtime dependencies.

> [!NOTE]
> `go install` is not supported. The module path is `cca` rather than a domain-qualified
> path, so it cannot be resolved remotely. Clone and build instead.

Full instructions: [Installation](https://12fahed.github.io/cca/start/installation/).

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

Narrow any view to a window:

```sh
cca --since 7d                              # relative: Nd, Nw, Nm
cca --since 2026-09-01 --until 2026-09-14   # inclusive of both days
```

`--since 7d` means the last seven calendar days **including today**, so it lines up with
a seven-row `cca daily` table.

### Where the money went

```sh
$ cca projects
```

```
  Claude Code usage · by project · all time

  Project                  sessions  requests  tokens    cost  share
  -home-dev-payments-api          4        95   17.6M  $25.51  41.9%
  -home-dev-infra-scripts         4       117   20.2M  $20.88  34.3%
  -home-dev-web-dashboard         5       128   22.1M  $14.43  23.7%

  Total                          13       340   60.0M  $60.82

  ─ Cost is what this usage would cost at API rates, not what you were billed;
    Claude Code on a subscription draws from your plan allowance instead.
```

```sh
$ cca sessions
```

```
  Claude Code usage · by session · all time

  Title                                Session      started  tokens   cost
  refactor: split the billing service  7c1a9f20  2026-09-14    7.2M  $9.30
  feat: dashboard charts and filters   9c1a77b0  2026-09-14    6.0M  $8.20
  Invoice rounding bug investigation   3ef0d219  2026-09-14    5.2M  $7.01
  —                                    51f4fc3e  2026-09-15    2.8M  $3.42

  ─ Cost is what this usage would cost at API rates, not what you were billed;
    Claude Code on a subscription draws from your plan allowance instead.
```

### Scripting

```sh
cca --json | jq '.totals.cost_usd.total'
cca models --json | jq -r '.models[] | "\(.key)\t\(.cost_usd.total)"'
cca daily --csv > usage.csv
```

`--json` and `--csv` work on every view, carry exact token counts and full session
identifiers, and are never styled. See
[JSON and CSV output](https://12fahed.github.io/cca/guides/machine-output/).

### Flags

| Flag | Effect |
| --- | --- |
| `--json`, `--csv` | machine-readable output for the current view |
| `--since`, `--until` | limit the time window |
| `--top N` | rows in the `sessions` view (default 10) |
| `--no-sidechains` | exclude sub-agent usage |
| `--titles`, `--no-titles` | include session titles in machine output; or suppress them everywhere |
| `--claude-dir` | read transcripts from somewhere other than `~/.claude` |
| `--pricing` | use an alternate `pricing.json` |
| `--water-ml-per-1k` | override the water constant |
| `--verbose` | files scanned, records kept, duplicates dropped, warnings |
| `--no-color`, `--ascii` | disable styling; plain ASCII output |

Full list: [Flags](https://12fahed.github.io/cca/reference/flags/).

## How the cost is calculated

Three details do most of the work:

**Deduplication.** The same assistant message is replayed into several files by resumed
sessions and by compaction. On real corpora this is routinely **half the records** — the
corpus this tool was built against was 54% duplicates. Without deduplication every total
would be roughly double.

**Cache write tiers.** Cached input bills differently depending on whether it was written
with a 5-minute or a 1-hour lifetime — 1.25x and 2x the base input rate. The development
corpus was 96% 1-hour writes, so collapsing them would have understated cache-write cost
by about a third.

**Absolute rates, not multipliers.**
[`internal/pricing/pricing.json`](internal/pricing/pricing.json) stores USD per million
tokens for all five billed classes of every model, mirroring the published price sheet.
Each number is checkable by eye, and per-model exceptions need no special arithmetic.

Rates change, so drop a `pricing.json` beside your config or pass `--pricing PATH`; it
**replaces** the embedded table entirely. If a model has no rate, `cca` counts its tokens,
leaves it out of the dollar total, and names it in a footnote. It never guesses.

Details: [How cost is calculated](https://12fahed.github.io/cca/internals/cost/).

## The water estimate

This is the playful part, and the part where being honest matters most.

**There is no reliable public per-token water figure for any model.** Published estimates
vary by more than an order of magnitude depending on datacenter location, cooling design,
power mix, and whether the figure counts only on-site evaporation or also the water used
to generate the electricity. Anyone quoting a precise number is extrapolating.

So `cca` does the least-bad thing: one constant, `0.30 mL per 1,000 tokens`, applied flat
across every model and token class, with the assumption printed beside the result every
single time. It is a **placeholder, not a measurement**.

If you have a figure you trust more, use it:

```sh
cca --water-ml-per-1k 0.12
```

More: [The water estimate](https://12fahed.github.io/cca/internals/water/).

## Accuracy and limitations

Worth knowing before you quote a number from this tool.

- **The transcript format is undocumented** and internal to Claude Code. It changes
  without notice — the development corpus alone held files written by three versions.
- **Totals can go *down* between runs.** Claude Code prunes and rotates transcripts.
  `cca` reports what is on disk now; it is not an append-only ledger.
- **A transcript is not a guaranteed-complete record of billed turns.** Claude Code's own
  accounting has been observed to include a request whose record was never written.
- **Web search cost is an upper bound** — failed searches are not billed, but the
  transcript does not mark them.
- **Token counts are not comparable across model generations.** Claude 4.7 and later use
  a tokenizer producing roughly 30% more tokens for the same text. Costs are unaffected;
  a token column comparing Opus 4.8 against Haiku 4.5 is not measuring the same thing.

Full list: [Accuracy and limits](https://12fahed.github.io/cca/internals/accuracy/).

## Privacy

`cca` reads `~/.claude` and nothing else. It makes **no network calls at any point** —
the rate table is compiled into the binary — and it **never writes to, moves, or deletes
anything under `~/.claude`**.

Transcripts contain your source code and your conversations, so that restriction is
enforced by tests: the directory is hashed before and after a run and must come back
byte-identical, and the parser is checked for any file-writing call at all.

**Session titles are withheld from machine output by default.** A title describes what you
were working on, and where no title was recorded the fallback is your first prompt
verbatim. Table views show it; `--json` and `--csv` require `--titles`, since that output
is what gets committed to repositories and pasted into issues. `--no-titles` suppresses it
everywhere.

No telemetry. Nothing is sent anywhere. See
[Privacy](https://12fahed.github.io/cca/internals/privacy/).

## Development

```sh
go run test/run.go          # fmt, vet, build, tests, race detector
go run test/run.go -cross   # also build all five release targets
make build-all              # write those binaries to dist/
```

See [test/README.md](test/README.md) for how the suite is laid out, and
[CONTRIBUTING.md](CONTRIBUTING.md) before opening a pull request.

The documentation site lives in [`docs/`](docs/) and is built with
[Starlight](https://starlight.astro.build):

```sh
cd docs && npm install && npm run dev
```

## License

[GPL-3.0](LICENSE) © Fahed Khan

`cca` is free software: you may use, study, share and modify it. If you distribute it or
a derivative, you must do so under the same license and make the corresponding source
available. Running it — however heavily, however modified — triggers no obligation.
