---
title: Privacy
description: What cca reads, what it never does, and how that is enforced.
sidebar:
  order: 5
---

Your Claude Code transcripts contain your source code and your conversations.
`cca` is built on the assumption that this is the most sensitive data on your
machine.

## It reads one directory

`~/.claude`, or wherever `--claude-dir` points. Nothing else.

## It makes no network calls

Not at startup, not during a run, not for rates. The rate table is **compiled
into the binary**, which is why replacing it is a local file edit. `cca` works
with no network at all.

There is **no telemetry**, no analytics, no crash reporting, and no update
check.

## It never writes to your transcripts

`cca` never creates, modifies, moves, or deletes anything under the Claude
directory. This is enforced by tests rather than by intention:

- **Behaviourally** — the whole directory tree is hashed before and after a
  full run and must come back byte-identical, with nothing added and nothing
  removed. A read-only directory of read-only files must still read cleanly,
  which would catch any attempt to drop a lock or scratch file alongside the
  transcripts.
- **Structurally** — the package that walks the directory is parsed and the
  test fails if any file-writing standard library call appears in it, or if a
  file is opened with anything other than a read-only open.

The first proves the current code is safe. The second refuses the next edit
that would not be.

## What it does write

Only what you ask it to:

- `stdout` — the report.
- `~/.config/cca/config.json` — **only if you create it**. `cca` never writes
  this file itself.

## What ends up in output

Project names are slugified working directory paths, so `cca projects` prints
directory names from your machine. Session identifiers are UUIDs. Neither
contains file contents or conversation text.

:::caution[Before sharing output]
`cca projects` and `cca sessions` reveal your directory structure. If you are
posting output publicly — in an issue, say — check what the project column
says first.
:::

`--json` includes the same fields, plus full session identifiers.

**Session titles are withheld from machine output by default.** A title is a
name you chose or a model's summary of your first prompt, and where neither
exists the fallback is your prompt text verbatim. Table views show it; `--json`
and `--csv` require `--titles`, because that output is what gets committed and
pasted. `--no-titles` suppresses it everywhere.

## Verifying it yourself

```sh
# Watch for filesystem writes (Linux)
strace -f -e trace=openat,write,unlink,rename ./cca 2>&1 | grep -v O_RDONLY

# Watch for network activity
ss -tunap | grep cca
```

The source is small enough to read: transcript discovery and parsing live in
[`internal/transcript`](https://github.com/12fahed/cca/tree/main/internal/transcript).
