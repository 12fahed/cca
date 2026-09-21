---
title: JSON schema
description: Every field in the --json document.
sidebar:
  order: 4
---

`--json` emits one document, the **same shape for every view**. All four
groupings are always present; the `view` field names what was asked for.

## Top level

| Field | Type | Meaning |
| --- | --- | --- |
| `schema_version` | int | Bumped when the shape changes incompatibly. Currently `1` |
| `view` | string | `summary`, `models`, `projects`, `daily`, or `sessions` |
| `window` | object | The time range that was applied |
| `location` | string | The time zone used to bucket days |
| `totals` | group | Everything admitted by the window |
| `main` | group | Totals excluding sub-agent traffic |
| `sidechain` | group | Sub-agent traffic alone |
| `water` | object | The water estimate and its assumed rate |
| `models` | group[] | Per model, ranked by cost |
| `projects` | group[] | Per project directory |
| `daily` | group[] | Per local calendar day, oldest first |
| `sessions` | group[] | Per session, ranked by cost, capped by `--top` |
| `unknown_models` | string[] | Model ids with no rate. Always an array |
| `warnings` | warning[] | Aggregated advisories |
| `diagnostics` | object | What was scanned and dropped |
| `notes` | string[] | The accuracy caveats, in words |

## `window`

| Field | Type | Meaning |
| --- | --- | --- |
| `label` | string | Human form, e.g. `all time`, `since 2026-09-14` |
| `since` | string \| null | RFC 3339 start, inclusive |
| `until` | string \| null | RFC 3339 last included instant |

`until` reports the last instant **inside** the window, not the exclusive
bound, so filtering on it selects the same records `cca` did.

## Group objects

Used by `totals`, `main`, `sidechain`, and every array entry.

| Field | Type | Meaning |
| --- | --- | --- |
| `key` | string | Model id, project, date, or full session id |
| `project` | string | Present on session groups |
| `requests` | int | Deduplicated assistant responses |
| `sessions` | int | Distinct sessions in the group |
| `projects` | int | Distinct projects in the group |
| `first`, `last` | string | RFC 3339 bounds, omitted when undated |
| `tokens` | object | Exact counts |
| `cost_usd` | object | Unrounded dollars |
| `unpriced_tokens` | int | Tokens whose model had no rate |

### `tokens`

`input`, `output`, `cache_write_5m`, `cache_write_1h`, `cache_read`,
`thinking`, `web_searches`, `total`.

:::caution[`thinking` is not an addend]
Thinking tokens are billed as output tokens and are **already inside**
`output`. They are reported so you can see how much reasoning happened, and are
excluded from `total`. Adding them double-counts.
:::

### `cost_usd`

`input`, `output`, `cache_write_5m`, `cache_write_1h`, `cache_read`,
`web_search`, `total`. The components sum to `total`.

## `water`

| Field | Type | Meaning |
| --- | --- | --- |
| `ml_per_1k_tokens` | number | The rate actually used |
| `millilitres`, `litres` | number | The estimate |
| `equivalence` | string | The lighthearted comparison |

## `warnings`

| Field | Type | Meaning |
| --- | --- | --- |
| `kind` | string | e.g. `unknown-model`, `fast-mode-anomaly` |
| `subject` | string | What it concerns |
| `count` | int | How many records |
| `detail` | string | A sentence explaining it |

## `diagnostics`

`files`, `lines`, `records`, `duplicates`, `skipped` (by reason),
`flat_cache_fallback`, `cache_split_mismatch`, `bad_timestamp`,
`no_dedup_key`, `filtered_by_window`, `undated`.

## Stability

`schema_version` is bumped when a change would break a consumer. Adding a field
is not such a change, so read defensively. Empty collections are `[]` rather
than `null`, so you can iterate without a nil check.

## A complete document

Arrays trimmed to one entry each; everything else is verbatim.

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
  },
  "main": {
    "requests": 305,
    "sessions": 13,
    "projects": 3,
    "first": "2026-09-05T09:07:00Z",
    "last": "2026-09-18T19:25:00Z",
    "tokens": {
      "input": 141138,
      "output": 651462,
      "cache_write_5m": 450532,
      "cache_write_1h": 6599068,
      "cache_read": 50894030,
      "thinking": 135109,
      "web_searches": 0,
      "total": 58736230
    },
    "cost_usd": {
      "input": 0.3891130000000003,
      "output": 8.841275000000005,
      "cache_write_5m": 1.462209999999999,
      "cache_write_1h": 35.93335000000003,
      "cache_read": 13.902720800000004,
      "web_search": 0,
      "total": 60.528668799999984
    }
  },
  "sidechain": {
    "requests": 35,
    "sessions": 5,
    "projects": 3,
    "first": "2026-09-05T10:18:00Z",
    "last": "2026-09-18T15:19:00Z",
    "tokens": {
      "input": 3406,
      "output": 21995,
      "cache_write_5m": 52500,
      "cache_write_1h": 0,
      "cache_read": 1150583,
      "thinking": 0,
      "web_searches": 0,
      "total": 1228484
    },
    "cost_usd": {
      "input": 0.003406,
      "output": 0.10997499999999999,
      "cache_write_5m": 0.06562500000000004,
      "cache_write_1h": 0,
      "cache_read": 0.1150583,
      "web_search": 0,
      "total": 0.2940643
    }
  },
  "water": {
    "ml_per_1k_tokens": 0.3,
    "millilitres": 17989.4142,
    "litres": 17.9894142,
    "equivalence": "about 2 minutes of shower"
  },
  "models": [
    {
      "key": "claude-opus-5",
      "requests": 101,
      "sessions": 8,
      "projects": 2,
      "first": "2026-09-06T11:04:00Z",
      "last": "2026-09-18T18:28:00Z",
      "tokens": {
        "input": 49531,
        "output": 219438,
        "cache_write_5m": 138450,
        "cache_write_1h": 2243392,
        "cache_read": 17672752,
        "thinking": 41667,
        "web_searches": 0,
        "total": 20323563
      },
      "cost_usd": {
        "input": 0.24765500000000001,
        "output": 5.48595,
        "cache_write_5m": 0.8653125000000003,
        "cache_write_1h": 22.433919999999997,
        "cache_read": 8.836375999999996,
        "web_search": 0,
        "total": 37.86921349999999
      }
    }
  ],
  "projects": [
    {
      "key": "-home-dev-payments-api",
      "requests": 95,
      "sessions": 4,
      "projects": 1,
      "first": "2026-09-06T11:04:00Z",
      "last": "2026-09-18T18:28:00Z",
      "tokens": {
        "input": 45085,
        "output": 196981,
        "cache_write_5m": 134106,
        "cache_write_1h": 2051770,
        "cache_read": 15188227,
        "thinking": 39217,
        "web_searches": 0,
        "total": 17616169
      },
      "cost_usd": {
        "input": 0.16629599999999994,
        "output": 3.7905300000000013,
        "cache_write_5m": 0.5942050000000001,
        "cache_write_1h": 15.215708000000001,
        "cache_read": 5.7395396000000005,
        "web_search": 0,
        "total": 25.506278600000005
      }
    }
  ],
  "daily": [
    {
      "key": "2026-09-05",
      "requests": 22,
      "sessions": 1,
      "projects": 1,
      "first": "2026-09-05T09:07:00Z",
      "last": "2026-09-05T10:27:00Z",
      "tokens": {
        "input": 5763,
        "output": 33186,
        "cache_write_5m": 35593,
        "cache_write_1h": 304427,
        "cache_read": 2550059,
        "thinking": 5257,
        "web_searches": 0,
        "total": 2929028
      },
      "cost_usd": {
        "input": 0.008548,
        "output": 0.252575,
        "cache_write_5m": 0.060432500000000014,
        "cache_write_1h": 0.9260259999999999,
        "cache_read": 0.4037712,
        "web_search": 0,
        "total": 1.6513527000000001
      }
    }
  ],
  "sessions": [
    {
      "key": "070d7109-26b1-4973-8e7a-ce7677216e9e",
      "project": "-home-dev-payments-api",
      "requests": 38,
      "sessions": 1,
      "projects": 1,
      "first": "2026-09-15T11:06:00Z",
      "last": "2026-09-15T14:38:00Z",
      "tokens": {
        "input": 19242,
        "output": 77500,
        "cache_write_5m": 47183,
        "cache_write_1h": 833766,
        "cache_read": 6156420,
        "thinking": 16438,
        "web_searches": 0,
        "total": 7134111
      },
      "cost_usd": {
        "input": 0.07875800000000001,
        "output": 1.588445,
        "cache_write_5m": 0.2396525,
        "cache_write_1h": 6.785294000000001,
        "cache_read": 2.3950386,
        "web_search": 0,
        "total": 11.087188100000004
      }
    }
  ],
  "unknown_models": [],
  "warnings": [],
  "diagnostics": {
    "files": 18,
    "lines": 450,
    "records": 340,
    "duplicates": 110,
    "skipped": {},
    "flat_cache_fallback": 0,
    "cache_split_mismatch": 0,
    "bad_timestamp": 0,
    "no_dedup_key": 0,
    "filtered_by_window": 0,
    "undated": 0
  },
  "notes": [
    "Cost is what this usage would cost at API rates, not what you were billed; Claude Code on a subscription draws from your plan allowance instead.",
    "Water assumes 0.30 mL / 1k tokens, a rough estimate."
  ]
}
```
