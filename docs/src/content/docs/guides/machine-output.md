---
title: JSON and CSV output
description: Exact figures for scripts, spreadsheets, and dashboards.
sidebar:
  order: 4
---

Every view supports `--json` and `--csv`. Both carry **exact** figures: token
counts are never abbreviated, costs are not rounded to cents, and session
identifiers are full.

The two flags are mutually exclusive.

## JSON

```sh
cca models --json
```

```json
{
  "schema_version": 1,
  "view": "models",
  "window": {
    "label": "all time",
    "since": null,
    "until": null
  },
  "location": "Local",
  "totals": {
    "requests": 340,
    "sessions": 13,
    "projects": 3,
    "first": "2026-09-05T09:07:00Z",
    "last": "2026-09-18T19:25:00Z",
    "tokens": {
      "input": 144544,
      "output": 673457,
      "cache_write_5m": 503032,
      "cache_write_1h": 6599068,
      "cache_read": 52044613,
      "thinking": 135109,
      "web_searches": 0,
      "total": 59964714
    },
    "cost_usd": {
      "input": 0.39251900000000023,
      "output": 8.951250000000007,
      "cache_write_5m": 1.5278349999999992,
      "cache_write_1h": 35.93335000000003,
      "cache_read": 14.017779100000006,
      "web_search": 0,
      "total": 60.82273309999998
    }
...
```

The document is the **same shape for every view** — all four groupings are
always present, with a `view` field naming what was asked for. A consumer gains
nothing from `cca` withholding data it has already computed, and one shape
makes the numbers verifiably identical to the tables.

### Useful queries

```sh
# Cost per model
cca --json | jq -r '.models[] | "\(.key)\t\(.cost_usd.total)"'

# Total spend this month
cca month --json | jq '.totals.cost_usd.total'

# The five most expensive sessions, with full ids
cca sessions --top 5 --json | jq -r '.sessions[] | "\(.key) \(.cost_usd.total)"'

# Anything cca could not price
cca --json | jq '.unknown_models'
```

### The caveats travel with the numbers

```sh
cca --json | jq -r '.notes[]'
```

```
Cost is what this usage would cost at API rates, not what you were billed; Claude Code on a subscription draws from your plan allowance instead.
Water assumes 0.30 mL / 1k tokens, a rough estimate.
```

Machine output is not an excuse to drop them. If you surface the cost figure in
a dashboard, carry the note with it.

See the [JSON schema reference](/cca/reference/json/) for every field.

## CSV

```sh
cca models --csv
```

```
model,requests,input_tokens,output_tokens,cache_write_5m_tokens,cache_write_1h_tokens,cache_read_tokens,thinking_tokens,web_searches,total_tokens,input_usd,output_usd,cache_write_5m_usd,cache_write_1h_usd,cache_read_usd,web_search_usd,total_usd,unpriced_tokens
claude-opus-5,101,49531,219438,138450,2243392,17672752,41667,0,20323563,0.24765500000000001,5.48595,0.8653125000000003,22.433919999999997,8.836375999999996,0,37.86921349999999,0
claude-sonnet-5,111,49851,239041,165436,2394039,17442170,50017,0,20290537,0.09970199999999997,2.39041,0.4135900000000002,9.576156,3.4884339999999994,0,15.968292000000005,0
```

The leading columns vary by view; every row then carries the full token and
cost breakdown.

| View | Leading columns |
| --- | --- |
| `summary`, `models` | `model`, `requests` |
| `projects` | `project`, `sessions`, `requests` |
| `daily` | `date`, `requests` |
| `sessions` | `session`, `project`, `started`, `ended`, `requests` |

Costs are written at **full precision** rather than rounded to cents,
specifically so that summing a column reaches the same total `cca` prints.

```sh
cca daily --csv > usage.csv
cca sessions --top 50 --csv | column -t -s,
```

## Neither format is ever styled

Colour and hyperlinks are suppressed for `--json` and `--csv` regardless of
terminal or environment, so piping into `jq` or a spreadsheet never encounters
escape sequences.
