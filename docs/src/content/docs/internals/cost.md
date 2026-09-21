---
title: How cost is calculated
description: The pipeline from a token count to a dollar figure.
sidebar:
  order: 2
---

Six steps, each independently testable.

## 1. Normalise the model id

Lowercased, trimmed, and any bracketed context-window tag removed.
`claude-opus-5[1m]` becomes `claude-opus-5`: the 1M window is the model's
standard context at its standard price, so the tag is a label rather than a
rate.

## 2. Resolve it to a rate row

Longest-prefix matching, which is load-bearing because several entries are
prefixes of others **and priced differently**:

| Id | Rate | Prefix of |
| --- | --- | --- |
| `claude-opus-4` | $15 / $75 | `claude-opus-4-8` |
| `claude-opus-4-8` | $5 / $25 | — |
| `claude-fable-5` | cache read $1.00 | `claude-fable-5-1` |
| `claude-fable-5-1` | cache read $0.25 | — |

First-match or shortest-match would misprice all of them. The Fable pair is the
nastier case: the two rows are identical in every other column, so a wrong
match still produces a plausible-looking total.

A prefix only counts when what follows is a **dated snapshot suffix** — a dash
and six or more digits. Without that rule an unreleased `claude-opus-4-9` would
quietly inherit retired `claude-opus-4`'s triple rate. Refusing to match makes
it an unknown model, which warns loudly; guessing would not.

## 3. Pick the rate row for the speed

Fast mode is not a uniform doubling, and **both directions of the mistake cost
money**. The table partitions models three ways:

- **Supported** — the fast row applies, with cache columns already stacked on
  the fast base rather than left at standard.
- **Accepted but standard** — the request runs at normal speed and bills at
  normal rates, so the fast row must *not* be used. Applying it would overcharge
  by 2x.
- **Rejected** — the API refuses fast for this model, so such a record should
  not exist. It bills at standard rates and raises a warning.

## 4. Multiply the five columns

```
cost = tokens ÷ 1,000,000 × rate
```

Applied independently to input, output, cache write 5m, cache write 1h, and
cache read.

:::note[Why the cache tiers stay separate]
A 5-minute cache write bills at 1.25x the base input rate; a 1-hour write at
2x. On the development corpus **96% of cache writes were 1-hour**, so
collapsing them into one figure would have understated cache-write cost by
about a third.

`cca` prefers the nested `cache_creation` split in the transcript. When a
record lacks it, everything is attributed to the 5-minute tier and the run is
footnoted, because that is an approximation and you should know.
:::

## 5. Apply modifiers

| Modifier | Effect |
| --- | --- |
| `inference_geo: "us"` | 1.1x on all five columns |
| Batch tier | 0.5x on all five columns |

An unrecognised region or tier is **not** guessed at. It bills at standard
rates and warns — priority tier, for instance, costs more rather than less, so
assuming parity would be wrong in the expensive direction.

## 6. Add server tools

Web search bills per request, so it sits **outside** the token multipliers. Web
fetch is free beyond the tokens it pulls in.

## Thinking tokens

Billed as output tokens and **already inside** `output_tokens`. `cca` never
adds them.

:::caution[A trap for anyone reimplementing this]
Because thinking is summarised, the visible thinking text is far shorter than
what was billed. A tool that counts output tokens from *content blocks* rather
than from `usage.output_tokens` undercounts badly. `cca` reads the usage field.
:::

## Unknown models

Tokens counted, dollars excluded, model named in a footnote. `cca` never
invents a rate.

## How this is verified

Claude Code writes its own cost accounting into `cost-state` records. `cca`'s
arithmetic reproduces one such record to within `1e-9`, and three further
assertions keep that from being a coincidence: the 5-minute cache attribution
must *not* also match, adding thinking tokens must *break* the match, and
`claude-opus-5[1m]` must resolve to the same rates as the bare id.
