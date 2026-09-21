---
title: Exit codes
description: What cca returns, for scripting.
sidebar:
  order: 5
---

| Code | Meaning |
| --- | --- |
| `0` | Success. Output was produced, even if it reported no usage |
| `1` | Runtime error — missing Claude directory, unreadable rate table, invalid config |
| `2` | Usage error — unknown command, unknown flag, contradictory options |

An empty result is **not** an error. A directory that exists with no
usage-bearing records exits `0` and prints `No usage found.`, because nothing
went wrong.

```sh
if cca --json > usage.json; then
  jq '.totals.cost_usd.total' usage.json
else
  echo "cca failed with $?" >&2
fi
```

Warnings — unknown models, unusual service tiers — do **not** change the exit
code. They are printed as footnotes and exposed in `--json` under `warnings`,
because the run succeeded and produced usable numbers.
