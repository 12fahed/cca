---
title: Rate table
description: The embedded prices, and how to replace them.
sidebar:
  order: 3
---

Rates are compiled into the binary, which is why `cca` needs no network access.
They are stored as **absolute USD per million tokens** for all five billed
classes of every model, mirroring the published price sheet — so each number is
checkable by eye, and per-model exceptions need no special arithmetic.

## Embedded rates

Verified on **2026-09-19** against
[https://platform.claude.com/docs/en/about-claude/pricing](https://platform.claude.com/docs/en/about-claude/pricing).

| Model | Input | Output | Cache 5m | Cache 1h | Cache read | Notes |
| --- | --- | --- | --- | --- | --- | --- |
| `claude-fable-5-1` | 10 | 50 | 12.5 | 20 | 0.25 | — |
| `claude-mythos-5-1` | 10 | 50 | 12.5 | 20 | 0.25 | inferred id |
| `claude-fable-5` | 10 | 50 | 12.5 | 20 | 1 | — |
| `claude-mythos-5` | 10 | 50 | 12.5 | 20 | 1 | inferred id |
| `claude-opus-5` | 5 | 25 | 6.25 | 10 | 0.5 | fast mode |
| `claude-opus-4-8` | 5 | 25 | 6.25 | 10 | 0.5 | fast mode |
| `claude-opus-4-7` | 5 | 25 | 6.25 | 10 | 0.5 | — |
| `claude-opus-4-6` | 5 | 25 | 6.25 | 10 | 0.5 | — |
| `claude-opus-4-5` | 5 | 25 | 6.25 | 10 | 0.5 | — |
| `claude-opus-4-1` | 15 | 75 | 18.75 | 30 | 1.5 | retired |
| `claude-opus-4` | 15 | 75 | 18.75 | 30 | 1.5 | retired |
| `claude-sonnet-5` | 2 | 10 | 2.5 | 4 | 0.2 | — |
| `claude-sonnet-4-6` | 3 | 15 | 3.75 | 6 | 0.3 | — |
| `claude-sonnet-4-5` | 3 | 15 | 3.75 | 6 | 0.3 | — |
| `claude-sonnet-4` | 3 | 15 | 3.75 | 6 | 0.3 | retired |
| `claude-haiku-4-5` | 1 | 5 | 1.25 | 2 | 0.1 | — |
| `claude-haiku-3-5` | 0.8 | 4 | 1 | 1.6 | 0.08 | retired |

All figures are USD per million tokens.

:::caution[Two pairs that look alike and are not]
`claude-fable-5` and `claude-fable-5-1` are identical in every column **except
cache reads**, where they differ four-fold. `claude-sonnet-4-6` costs 50% more
than `claude-sonnet-5`. Model resolution uses longest-prefix matching precisely
so these never collapse into one another.
:::

## Modifiers

| Modifier | Multiplier | When |
| --- | --- | --- |
| `inference_geo_us` | 1.1x | The record pins inference to the US |
| `batch` | 0.5x | The record ran on the batch tier |

Fast mode does not multiply — it **swaps** the base rate pair, with cache
columns recomputed from the fast base.

| Fast mode | Models |
| --- | --- |
| Supported | `claude-opus-5`, `claude-opus-4-8` |
| Accepted, runs and bills at standard speed | `claude-opus-4-6` |
| Rejected by the API | `claude-opus-4-7` |

## Server tools

| Tool | Price |
| --- | --- |
| Web search | $0.01 per request |
| Web fetch | Free beyond the tokens it pulls in |

Failed web searches are not billed, but the transcript does not mark them, so
that charge is an **upper bound**.

## Replacing the table

Rates change. Put a `pricing.json` in your config directory, or pass
`--pricing PATH`:

```sh
cca --pricing ./my-rates.json
```

The override **replaces** the embedded table entirely rather than merging into
it, so a price change is a whole-file edit and never a partial one. Start by
copying
[`internal/pricing/pricing.json`](https://github.com/12fahed/cca/blob/main/internal/pricing/pricing.json).

### Minimal example

```json
{
  "schema_version": 1,
  "currency": "USD",
  "unit": "per_million_tokens",
  "models": [
    {
      "id": "claude-opus-5",
      "name": "Claude Opus 5",
      "rates": {
        "input": 5.00,
        "output": 25.00,
        "cache_write_5m": 6.25,
        "cache_write_1h": 10.00,
        "cache_read": 0.50
      }
    }
  ],
  "non_models": ["<synthetic>"]
}
```

`non_models` lists placeholder strings that appear where a model id belongs.
They are dropped before unknown-model detection, so they never produce a
spurious warning.

### Validation

A table is rejected on load if the schema version is unsupported, no models are
listed, a model has no id, two models share an id, or any rate is negative.

## Unknown models

`cca` never guesses a rate. A model with no entry has its **tokens counted and
its dollars excluded**, and is named in a footnote telling you how to price it.
