---
title: Quick start
description: Your first few commands, and how to read what they print.
sidebar:
  order: 3
---

Once `cca` is on your `PATH`, there is nothing to set up. Run it.

## Everything, all time

```sh
cca
```

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

That is the default view, and it is designed to fit one screen. Reading it top
to bottom:

- **The header** names the window, the session count and the project count.
- **Tokens** splits usage across the five classes that are billed differently.
  Cache reads dominate almost every real corpus.
- **Model** ranks the models by spend.
- **Cost** and **Water** are the two headline figures, each printed with the
  caveat that qualifies it.

The footnotes are not decoration. They say what the number means and where it
is soft. See [Reading the summary](/cca/guides/summary/).

## Narrow it to this week

```sh
cca week
```

Or pick the window yourself:

```sh
cca --since 7d
cca --since 2026-09-01 --until 2026-09-14
```

See [Time ranges](/cca/guides/time-ranges/).

## Find where the money went

```sh
cca projects
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
cca sessions --top 5
```

```
  Claude Code usage · by session · all time

  Session   started     project                  requests  tokens    cost
  070d7109  2026-09-15  -home-dev-payments-api         38    7.1M  $11.09
  7a3a8394  2026-09-08  -home-dev-infra-scripts        35    7.6M   $7.46
  b82763ba  2026-09-07  -home-dev-infra-scripts        41    6.7M   $6.05
  bee80626  2026-09-18  -home-dev-payments-api         26    4.9M   $6.03
  a3a16d92  2026-09-16  -home-dev-web-dashboard        34    6.7M   $4.64

  ─ Cost is what this usage would cost at API rates, not what you were billed;
    Claude Code on a subscription draws from your plan allowance instead.
```

## Feed it to something else

```sh
cca models --json | jq '.models[] | {model: .key, usd: .cost_usd.total}'
cca daily --csv > usage.csv
```

See [JSON and CSV output](/cca/guides/machine-output/).

## Check what cca thinks it is doing

```sh
cca config
```

Shows every resolved setting **and which layer supplied it** — flag, config
file, or default. It is the fastest way to explain surprising output.

```sh
cca --verbose
```

Appends a diagnostics block: files scanned, records kept, duplicates dropped,
and any warnings. See [Troubleshooting](/cca/guides/troubleshooting/).

## Where next

- [Reading the summary](/cca/guides/summary/) — what each figure means.
- [Commands](/cca/reference/commands/) — the full list.
- [How cost is calculated](/cca/internals/cost/) — deduplication and cache tiers.
