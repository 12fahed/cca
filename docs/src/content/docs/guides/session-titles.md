---
title: Session titles
description: Where session names come from, how they are chosen, and why machine output omits them.
sidebar:
  order: 7
---

A session id tells you nothing about what the session was. `cca sessions` shows
the session's name alongside it.

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

The shortened id is still there, and is still what you paste into
`claude --resume`.

## Where a title comes from

Claude Code records two kinds of name in a session's transcript, and `cca`
prefers them in this order:

| Source | What it is | How it gets set |
| --- | --- | --- |
| `custom` | A name you chose | `/rename`, `--name`, `-n`, `Ctrl+R` in the resume picker, the VS Code rename |
| `ai` | A short summary of your first prompt | Generated automatically in the background |
| `first-prompt` | Your first message, truncated | Only when neither of the above exists |

A name you chose always beats a generated one, whichever was written last. If a
session has no usable source, `cca` shows a dash rather than inventing
something.

:::note[Renaming works as you would expect]
These records are appended rather than replaced, so a session renamed three
times carries three of them. `cca` takes the most recent. On the corpus this
feature was built against, 28 of 41 titled sessions had been renamed at least
once.
:::

## Seeing which source won

```sh
cca sessions --verbose
```

```
  Claude Code usage · by session · all time

  Title                           Session   from       started  tokens   cost
  refactor: split the billing s…  7c1a9f20  custom  2026-09-14    7.2M  $9.30
  feat: dashboard charts and fi…  9c1a77b0  custom  2026-09-14    6.0M  $8.20
  Invoice rounding bug investig…  3ef0d219  ai      2026-09-14    5.2M  $7.01
  —                               51f4fc3e  —       2026-09-15    2.8M  $3.42
```

Useful when a title looks wrong: `custom` is a name someone typed, `ai` is a
guess from the first prompt.

## Turning them off

```sh
cca sessions --no-titles
```

```
  Claude Code usage · by session · all time

  Session   started     project        requests  tokens   cost
  7c1a9f20  2026-09-14  payments-api         40    7.2M  $9.30
  9c1a77b0  2026-09-14  web-dashboard        32    6.0M  $8.20
  3ef0d219  2026-09-14  payments-api         26    5.2M  $7.01
  51f4fc3e  2026-09-15  web-dashboard        12    2.8M  $3.42

  ─ Cost is what this usage would cost at API rates, not what you were billed;
    Claude Code on a subscription draws from your plan allowance instead.
```

Without titles the view returns to showing the project and request count.

## Titles in JSON and CSV

**Machine output omits titles unless you ask for them.**

```sh
cca sessions --json --titles
```

```json
{"key":"7c1a9f20-5dba-4b67-b952-5cd4f8d2dc34","title":"refactor: split the billing service","title_source":"custom"}
```

```sh
cca sessions --csv --titles
```

:::caution[Why opt-in]
A title describes what you were working on, and the `first-prompt` fallback is
literally your prompt text. `--json` and `--csv` output gets committed to
repositories, pasted into issues, and piped into other tools, so titles are
left out unless explicitly requested.

`--no-titles` suppresses them everywhere and overrides `--titles`.
:::

Machine output carries the **full** title and the **full** session id; the
table truncates the title and shortens the id for display only.

## Limits worth knowing

**`cca` will sometimes show a title the VS Code sidebar does not.** The
extension reads only the last 64KB of a transcript, so on a long session a
rename can fall outside that window and the sidebar loses it. `cca` streams the
whole file. This is a better result, not a discrepancy to report.

**A title can be wrong rather than merely missing.** There is an upstream bug
where deleting a transcript lets Claude Code reuse the session id for a new
conversation, and the new session inherits an unrelated title. `cca` cannot
detect this — which is one reason titles never influence anything but display.
No cost, token, deduplication, sorting, or filtering logic reads one.

**Desktop and claude.ai/code sessions are out of scope.** Those surfaces keep
their own history, and their titles do not reliably reach the local transcript.
`cca` covers CLI sessions.

**Duplicate titles are legal.** Two sessions can carry the same name, and `cca`
never groups or deduplicates by title.

## Display handling

Titles are user-controlled text, so they are sanitized before reaching your
terminal: escape sequences and control characters are stripped, and newlines
and tabs collapse to single spaces. Truncation is by **display width**, so a
title in Japanese or one containing emoji lands in the same column as an ASCII
one rather than pushing the table out of alignment.
