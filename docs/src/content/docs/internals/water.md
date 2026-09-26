---
title: The water estimate
description: Why the number is soft, and how to replace it with your own.
sidebar:
  order: 3
---

This is the playful part of `cca`, and the part where being honest matters
most.

## There is no reliable figure

**No trustworthy public per-token water figure exists for any model.**
Published estimates vary by more than an order of magnitude depending on:

- where the datacenter is, and what its climate demands of cooling;
- the cooling design — evaporative towers consume far more water than
  closed-loop or air-cooled systems;
- the power mix, since thermoelectric generation consumes water too;
- and crucially, **whether the figure counts only on-site evaporation or also
  the water consumed generating the electricity**.

Anyone quoting a precise number is extrapolating from one datacenter, one
cooling design, and one set of accounting boundaries.

## So `cca` does the least-bad thing

One constant — **0.30 mL per 1,000 tokens** — applied flat across every model
and every token class, with the assumption printed beside the result every
single time:

```
Water   ≈ 18.0 L   about 2 minutes of shower

─ Water assumes 0.30 mL / 1k tokens, a rough estimate. See README.
```

The figure is never shown without the caveat that produced it.

:::caution[It is a placeholder, not a measurement]
Treat the water figure as an order-of-magnitude illustration. It is there to
make an abstract quantity feel concrete, not to support a claim.
:::

## Why it is flat

Scaling per model or per token class would be **false precision dressed up as
rigour**. The underlying figure is uncertain by more than 10x; differentiating
a cache read from an output token to three significant figures would imply a
confidence that does not exist.

## Substitute your own

If you have a figure you trust more — from your provider, or from research you
find credible — use it:

```sh
cca --water-ml-per-1k 0.12
```

Or permanently, in `~/.config/cca/config.json`:

```json
{ "water_ml_per_1k_tokens": 0.12 }
```

The printed assumption updates to match, so the output never claims a rate it
did not use.

## Per-session water

`cca sessions` shows each session's share:

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

:::note[The column is the token column rescaled]
Water is a flat rate per token, so a session's water figure is exactly its token
total in different units. Ranking sessions by water gives the same order as
ranking by tokens, and the ratio between any two rows is identical in both
columns.

That is the point rather than a flaw — "2.1 L" means something to a person in a
way "7.2M tokens" does not. But the column reveals nothing the token column did
not, and nothing in `cca` sorts, filters, or aggregates by it.
:::

Rows carry the volume alone. An equivalence is what makes a single figure
concrete; twenty of them down a column would be noise, so those stay on the
summary.

`--no-water` hides the column:

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

In `--json` and `--csv` the figure is always present, on every grouping, since
those are not displays.

## The equivalences

The litre figure is converted to whatever familiar unit it covers at least
once, which keeps the count small and readable:

| Unit | Volume |
| --- | --- |
| Cup of coffee | 0.25 L |
| Bottle of water | 0.5 L |
| Minute of shower | 9 L |
| Load of laundry | 50 L |
| Bathtub | 150 L |
| Swimming pool | 50,000 L |

Counts are rounded to whole numbers, because a decimal would imply precision
the estimate does not have.

The equivalences are deliberately lighthearted and carry **no judgement**. The
tool reports; it does not lecture. There is a test that guards the wording
against drifting into it.
