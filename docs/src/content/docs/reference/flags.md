---
title: Flags
description: Every flag, what it does, and where it can also be set.
sidebar:
  order: 2
---

Flags may appear **on either side of the command**:

```sh
cca --json models
cca models --json    # identical
```

## Output format

| Flag | Effect |
| --- | --- |
| `--json` | Machine-readable document with exact figures. Never styled |
| `--csv` | CSV rows for the current view. Never styled |

Mutually exclusive.

## Time range

| Flag | Effect |
| --- | --- |
| `--since SPEC` | Start of the window: `Nd`, `Nw`, `Nm`, or `YYYY-MM-DD` |
| `--until DATE` | End of the window, inclusive of that whole day. Absolute dates only |

Rejected when combined with `today`, `week`, or `month`. See
[Time ranges](/cca/guides/time-ranges/).

## Scope

| Flag | Default | Effect |
| --- | --- | --- |
| `--top N` | `10` | Rows in the `sessions` view |
| `--no-sidechains` | off | Exclude sub-agent usage. It is real spend, so it is included by default |
| `--claude-dir PATH` | `~/.claude` | Read transcripts from elsewhere |

## Pricing and water

| Flag | Default | Effect |
| --- | --- | --- |
| `--pricing PATH` | embedded | Replace the rate table entirely |
| `--water-ml-per-1k N` | `0.30` | The water constant. A placeholder, not a measurement |

An explicitly named `--pricing` file must exist; a discovered one in the config
directory may be absent, in which case the embedded table is used.

## Presentation

| Flag | Effect |
| --- | --- |
| `--no-color` | Never style output |
| `--ascii` | Substitute `-` and `~` for `·`, `≈`, `─` |
| `--verbose` | Append the diagnostics block |

## Environment variables

| Variable | Effect |
| --- | --- |
| `NO_COLOR` | Disables colour whatever its value. Its presence is the signal |
| `CLICOLOR_FORCE` | Keeps colour when output is not a terminal, for paging |
| `XDG_CONFIG_HOME` | Relocates the config directory on Unix |
| `APPDATA` | Relocates the config directory on Windows |

Colour precedence, highest first: `--no-color`, `NO_COLOR`, `CLICOLOR_FORCE`,
whether output is a terminal, whether that terminal supports escapes.

## Which flags can be set in the config file

`claude_dir`, `pricing`, `water_ml_per_1k_tokens`, `no_sidechains`, `ascii`,
and `no_color`. A flag always wins over the file. See
[Configuration](/cca/guides/configuration/).

## Full help output

```
cca — analyze local Claude Code usage

Usage:
  cca [command] [flags]

Commands:
  summary   all-time usage summary (default)
  today     usage since local midnight
  week      usage over the last 7 days
  month     usage over the last month
  models    breakdown by model
  projects  breakdown by project directory
  daily     per-day table, newest last
  sessions  most expensive sessions
  config    show resolved config and file paths
  version   version, commit, and build date
  help      show this help

Flags:
  -ascii
    	ASCII-only output for terminals that mangle Unicode
  -claude-dir string
    	override the ~/.claude location
  -csv
    	CSV output for the current view
  -json
    	machine-readable output
  -no-color
    	disable styling (NO_COLOR is also honored)
  -no-sidechains
    	exclude sub-agent usage
  -pricing string
    	path to an alternate pricing.json
  -since string
    	start of window: 7d, 2w, 3m, or 2026-09-01
  -top int
    	number of rows for the sessions view (default 10)
  -until string
    	end of window: 2026-09-14
  -verbose
    	show files scanned, skipped lines, dedup stats
  -water-ml-per-1k value
    	water constant in mL per 1k tokens (default 0.30 — a rough placeholder, not a measured value)
```
