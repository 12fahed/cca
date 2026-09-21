---
title: Where the numbers come from
description: The transcript format, discovery, and deduplication.
sidebar:
  order: 1
---

## The files

Claude Code writes one JSON object per line:

```
~/.claude/projects/<slugified-working-directory>/<session-id>.jsonl
```

Sub-agent sessions nest two levels deeper:

```
~/.claude/projects/<slug>/<session-id>/subagents/agent-<id>.jsonl
```

:::note[The nested layout is easy to miss]
A discovery routine that only walks one level finds the top-level sessions and
silently drops **every sub-agent** — which also means every sidechain record
and most Haiku usage, while still producing a confident-looking total. `cca`
walks recursively for exactly this reason.
:::

## What is kept

Only records of type `assistant` that carry `message.usage`. Everything else —
user turns, attachments, file snapshots, titles — is skipped and counted by
reason, visible under `--verbose`.

A record contributes:

| From | Used for |
| --- | --- |
| `message.model` | Which rate row applies |
| `message.usage` | The five token classes |
| `sessionId`, `cwd`, project directory | Attribution |
| `timestamp` | The local calendar day |
| `isSidechain` | Sub-agent tagging |
| `usage.speed`, `service_tier`, `inference_geo` | Rate modifiers |

## Deduplication

This is the single most important step.

The same assistant message is replayed into several files by **resumed
sessions** and by **compaction**. On the corpus `cca` was developed against,
**54% of usage-bearing records were duplicates**. Without deduplication every
figure would be roughly double.

`cca` keys on `message.id` + `requestId`, falling back to the record `uuid`,
**globally across all files** rather than per file. Files are walked in sorted
order so that first-occurrence-wins attribution is reproducible.

Records with neither key are kept and counted, because dropping them would lose
real spend.

## Tolerance

The transcript format is internal to Claude Code and undocumented. It changes
without notice — one development corpus contained files written by three
different versions.

So parsing is deliberately forgiving. A malformed line, a truncated line, or a
line larger than the read buffer is skipped and counted, never fatal. Reading
uses a buffered reader rather than a scanner specifically because a scanner
abandons the whole file when one line exceeds its buffer, which would lose
every record after it.

## Placeholders

`<synthetic>` appears where a model id belongs, on cancelled or errored turns,
always with zero tokens. It is filtered **before** unknown-model detection, so
it never produces a spurious warning. The list comes from the rate table's
`non_models`, not from a hardcoded string.
