---
title: Commands
description: Every command cca accepts.
sidebar:
  order: 1
---

```
cca [command] [flags]
```

With no command, `cca` prints the all-time summary.

```
cca — analyze local Claude Code usage
```

## Usage views

Each of these prints a table, and each accepts every
[global flag](/cca/reference/flags/) including `--json`, `--csv`, and the time
range options.

### `cca` / `cca summary`

The default one-screen view: token classes, models ranked by spend, cost,
water, and the accuracy footnotes. See
[Reading the summary](/cca/guides/summary/).

### `cca today`

The summary, since local midnight. Equivalent to `--since 1d`.

### `cca week`

The summary, over the last seven calendar days including today. Equivalent to
`--since 7d`.

### `cca month`

The summary, back one calendar month from today. Equivalent to `--since 1m`.

### `cca models`

Per-model breakdown with requests, tokens, cost, and each model's share of the
cost total.

### `cca projects`

Per-project breakdown, plus a distinct session count per project. Project names
are slugified working directories, shortened from the left when long.

### `cca daily`

One row per local calendar day, oldest first. Days with no usage are omitted.

### `cca sessions`

Sessions ranked by cost, each shown with its title and a shortened identifier.
Accepts `--top N` (default 10).

```
  Claude Code usage · by session · all time

  Title                                Session    water  tokens   cost
  refactor: split the billing service  7c1a9f20   2.1 L    7.2M  $9.30
  feat: dashboard charts and filters   9c1a77b0   1.8 L    6.0M  $8.20
  Invoice rounding bug investigation   3ef0d219   1.6 L    5.2M  $7.01
  —                                    51f4fc3e  841 mL    2.8M  $3.42

  ─ Cost is what this usage would cost at API rates, not what you were billed;
    Claude Code on a subscription draws from your plan allowance instead.
  ─ Water assumes 0.30 mL / 1k tokens, a rough estimate. See README.
```

`--verbose` adds the title's resolution source, `--no-titles` returns the
project and request columns, and `--no-water` drops the water column and brings
back the one it displaces. Full identifiers and untruncated titles appear in
`--json` and `--csv`. See [Session titles](/cca/guides/session-titles/).

## Informational

### `cca config`

Prints every resolved setting, the layer that supplied it, and the paths `cca`
searches for its config and rate table. Reads **no transcripts**, so it works
even when the Claude directory is missing — which is exactly when you need it.

### `cca version`

```
cca       v0.1.0
commit    1964cd1
built     2026-09-22T09:14:03Z
go        go1.27.1
platform  linux/amd64
license   GPL-3.0, no warranty — https://github.com/12fahed/cca/blob/main/LICENSE
```

Version, commit and build date are injected at build time.

### `cca help`

The command list and every flag with its default.

## Exit codes

See [Exit codes](/cca/reference/exit-codes/).
