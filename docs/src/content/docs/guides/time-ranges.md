---
title: Time ranges
description: Narrowing any view to a window, and exactly what each spec means.
sidebar:
  order: 3
---

Every usage view accepts a time range. There are two ways to give one.

## Preset commands

```sh
cca today     # since local midnight
cca week      # the last 7 days
cca month     # the last calendar month
```

These print the same layout as the default summary, narrowed.

## Explicit ranges

```sh
cca --since 7d                              # relative
cca --since 2026-09-01                      # absolute, open-ended
cca --since 2026-09-01 --until 2026-09-14   # both ends
cca projects --since 30d                    # works on any view
```

## What the relative specs mean

| Spec | Meaning |
| --- | --- |
| `Nd` | The last **N calendar days including today** |
| `Nw` | The last `7 × N` days, same rule |
| `Nm` | Back **N calendar months** from today |

:::tip[Why `7d` is seven rows]
`--since 7d` covers today plus the six days before it, so it lines up exactly
with a seven-row `cca daily` table. A definition of "168 hours ago" would have
produced a ragged partial day at the start.
:::

## What `--until` means

`--until` includes **the whole day you name**. `--until 2026-09-14` covers
everything up to `2026-09-14 23:59:59` local time.

Relative specs are rejected for `--until`, because "until 7d" reads ambiguously
— it could plausibly mean either end of that span.

## Time zones

Records carry UTC timestamps. Every boundary is computed in **your local zone**,
and relative specs anchor to local midnight.

This means the same instant can fall on different sides of the same window
depending on where you are, which is correct: `cca today` should mean *your*
today.

## Combining is refused

```sh
$ cca today --since 7d
cca: this command already sets its own time range; drop --since/--until or use
the default view with them instead
```

A preset window and an explicit one cannot both be honoured. Rather than
silently picking one, `cca` stops and says so.

## Records with no usable timestamp

A record whose timestamp cannot be parsed is counted in the totals but cannot
be placed on a calendar. A **bounded** window therefore excludes it — claiming
it falls inside a range would be a guess. `--verbose` reports how many there
were.
