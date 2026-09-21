---
title: Accuracy and limits
description: Everything worth knowing before quoting a number from cca.
sidebar:
  order: 4
---

`cca` tries to be accurate and, where it cannot be, to say so. This page
collects every known limit in one place.

## The cost is not a bill

Claude Code on a Pro, Max, or Team plan draws from a plan allowance rather than
charging per token. The figure is what the same usage **would** have cost at
API list prices.

## The transcript format is undocumented

It is internal to Claude Code and changes without notice — one development
corpus alone contained files written by **three different versions**. `cca`
parses tolerantly, skips what it cannot read, and counts every skip under
`--verbose`. A future version could still break it.

## Totals can go **down** between runs

Claude Code prunes and rotates transcripts, so history disappears over time.
`cca` reports what is on disk now. **It is not an append-only ledger**, and a
figure you recorded last month may not reproduce today.

## A transcript is not a complete record of billed turns

This one was found by cross-checking against Claude Code's own accounting.

On the development corpus, a session's `cost-state` snapshot accounted for a
request whose assistant record appears **nowhere in the files** — `cca` reported
8% less for that session. Deduplication, cross-session attribution, and
missing-usage records were each ruled out first; the turn simply was not
written.

So a per-session figure can be an undercount, and `cca` cannot detect when.

## Web search cost is an upper bound

Failed searches are not billed, but the transcript does not mark them, so every
recorded search is charged.

## Token counts are not comparable across model generations

Claude 4.7 and later use a tokenizer producing roughly **30% more tokens** for
the same text. Costs are unaffected — real tokens are counted — but a token
column comparing an Opus 4.8 row against a Haiku 4.5 row is not measuring the
same thing.

## Long-context and fast-mode variants

`[1m]` is a label, not a rate: the 1M window is standard context at standard
price. Fast mode **is** a real premium, and `cca` applies it where the
transcript records `speed: "fast"` — but it can only do so where the record
says.

## Sub-agent usage

Included by default, tagged separately, excludable with `--no-sidechains`. It
is real spend. On the development corpus it was a small fraction of the total,
because sub-agents mostly run Haiku.

## Records with no usable timestamp

Counted in totals, excluded from the per-day table, and excluded from any
bounded window — placing them would be a guess. `--verbose` reports the count.

## What is verified, and how

| Property | How it is checked |
| --- | --- |
| Cost arithmetic | Reproduces Claude Code's own `cost-state` total to `1e-9` |
| Deduplication | Exact counts against fixtures, plus a ratio band on real data |
| `--json` matches the tables | Tables are rendered, parsed back, and compared cell by cell |
| Groupings reconcile | Every grouping must sum to the overall totals |
| Nothing is written | The directory is hashed before and after a run and must match |
| Time zone handling | Asserted across a UTC/local day boundary in both directions |
