---
title: Reading the summary
description: What every figure in the default view means, and which ones to distrust.
sidebar:
  order: 1
---

Running `cca` with no arguments prints the summary. It is the view most people
use most of the time, so it is worth reading closely once.

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

## The header

```
Claude Code usage · all time · 13 sessions across 3 projects
```

The window comes first (`all time`, `since 2026-09-14`, `2026-09-01 to
2026-09-14`), then the scope. A session is one Claude Code conversation; a
project is one working directory.

## The token row

Five classes, because each is **billed at a different rate**:

| Class | What it is | Typical rate |
| --- | --- | --- |
| `input` | Fresh prompt tokens | base |
| `output` | Everything the model generates | 5x base |
| `cache 5m` | Cache written with a 5-minute lifetime | 1.25x base |
| `cache 1h` | Cache written with a 1-hour lifetime | 2x base |
| `cache read` | Cache hits | 0.1x base |

:::tip[Cache reads dominate, but cost little]
On a real corpus, cache reads are routinely **95% of all tokens** and under
half the cost, while 1-hour cache writes can be 4% of tokens and a third of the
cost. Token share and dollar share tell different stories, which is why the
view shows both.
:::

Thinking tokens are **not** a separate column. They are billed as output tokens
and already counted inside `output`; adding them would double-count. They are
reported separately in [JSON output](/cca/guides/machine-output/).

## The model table

Models ranked by spend, with abbreviated token counts. A model with no rate in
the table shows `unpriced` instead of a figure — its tokens still count, its
dollars cannot. See [Rate table](/cca/reference/pricing/).

## Cost and water

```
Cost    $60.82     at API list prices
Water   ≈ 18.0 L   about 2 minutes of shower
```

**Cost** is the sum of every priced token at published API rates.

**Water** is a deliberately rough estimate, printed with its assumption inline
every single time. It is a placeholder, not a measurement — see
[The water estimate](/cca/internals/water/).

## The footnotes

Two always appear:

- that the cost is **not a bill**, because a subscription draws from an allowance;
- that the water figure rests on **a rough assumed rate**.

Others appear only when they apply — unknown models, missing cache tier data,
fast mode, US-pinned inference, batch tier, web search, undated records. Each
names a condition that changed the arithmetic. If you see one, it is telling
you something specific about *your* data.

## Colour

Output is coloured when writing to a terminal that supports it:

- **orange** for token counts
- **green** for money
- **light blue** for the water figure
- **amber** for anything you are being warned about

Colour is disabled automatically when output is piped or redirected, and can be
turned off with `--no-color` or the `NO_COLOR` environment variable.
