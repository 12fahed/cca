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

Sessions ranked by cost. Accepts `--top N` (default 10). Session identifiers
are shortened for display; full ones appear in `--json` and `--csv`.

## Informational

### `cca config`

Prints every resolved setting, the layer that supplied it, and the paths `cca`
searches for its config and rate table. Reads **no transcripts**, so it works
even when the Claude directory is missing — which is exactly when you need it.

### `cca version`

```
cca       2aaa79b
commit    2aaa79b
built     2026-09-21T20:00:33Z
go        go1.27.1
platform  linux/amd64
```

Version, commit and build date are injected at build time.

### `cca help`

The command list and every flag with its default.

## Exit codes

See [Exit codes](/cca/reference/exit-codes/).
