---
title: What is cca?
description: What cca measures, what it does not, and who it is for.
sidebar:
  order: 1
---

`cca` (Claude Code Analyzer) reads the transcripts Claude Code writes to your
own machine and tells you three things:

1. **How many tokens** you have used, split by how each kind is billed.
2. **What that would have cost** at Anthropic's published API prices.
3. **Roughly how much water** that implies — a deliberately soft, playful figure.

It is a tool for curiosity and awareness. It is not an accounting system, and
it is careful to say so wherever it prints a number.

## The one thing to understand first

:::caution[The dollar figure is not a bill]
Claude Code on a Pro, Max, or Team plan does not charge per token. It draws
from a plan allowance. What `cca` reports is what the same usage *would* have
cost at API list prices.

That is a useful gauge of how heavily you lean on the tool. It is not an
invoice, and it will not match one.
:::

If you use Claude Code through API billing rather than a subscription, the
figure is much closer to what you actually pay — but still an estimate, for the
reasons set out in [Accuracy and limits](/cca/internals/accuracy/).

## What it reads

Claude Code stores one JSON object per line under:

```
~/.claude/projects/<slugified-working-directory>/<session-id>.jsonl
```

Sub-agent sessions nest two levels deeper:

```
~/.claude/projects/<slug>/<session-id>/subagents/agent-<id>.jsonl
```

`cca` streams those files, keeps the assistant records that carry token
accounting, deduplicates them, and prices the result. Nothing else on your
machine is touched. See [Privacy](/cca/internals/privacy/).

## What it is good at

- Seeing which **projects** and **sessions** consume the most.
- Watching usage **per day**, to spot the week a refactor got expensive.
- Comparing **models** — how much of your spend is Opus versus Haiku.
- Feeding a spreadsheet or a script through `--json` and `--csv`.

## What it cannot tell you

- What you were actually charged, if you are on a subscription.
- Usage from other machines — it only sees the transcripts on this one.
- Anything about turns Claude Code did not record. See
  [Accuracy and limits](/cca/internals/accuracy/).
