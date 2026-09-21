---
title: Contributing
description: How to build, test, and propose changes to cca.
sidebar:
  order: 1
---

Contributions are welcome. The full guidelines live in
[CONTRIBUTING.md](https://github.com/12fahed/cca/blob/main/CONTRIBUTING.md);
this page covers enough to get productive.

## Build and test

```sh
git clone https://github.com/12fahed/cca.git
cd cca

make build            # ./cca
go run test/run.go    # fmt, vet, build, tests, race detector
```

| Command | What it does |
| --- | --- |
| `make build` | Build the binary |
| `make test` | `go test ./...` |
| `make test-race` | Tests under the race detector |
| `make test-all` | Everything CI runs |
| `make build-all` | All five release targets into `dist/` |
| `go run test/run.go -cross` | Everything, plus a cross-compile check |

## Project layout

```
cmd/cca/              flag parsing, dispatch, exit codes — no business logic
internal/transcript/  discovery, streaming parse, deduplication
internal/pricing/     rate table, model resolution, cost math
internal/report/      aggregation and time windows
internal/render/      tables, JSON, CSV, colour
internal/config/      path discovery and precedence
internal/water/       the estimate and its equivalences
test/                 the runner and cross-package tests
docs/                 this site
```

## Conventions worth knowing

**Tests live beside their package.** Go requires it for anything touching
unexported identifiers, and `cca`'s tests deliberately do — the dedup key, the
scanner buffer limit, the prefix matcher. `test/unit/` holds only what reaches
across packages.

**Zero runtime dependencies.** The shipped binary is static, built with
`CGO_ENABLED=0`. Adding a dependency needs a stated reason.

**Never write to `~/.claude`.** Enforced by tests, both behavioural and
structural. See [Privacy](/cca/internals/privacy/).

**Never commit real transcripts.** Fixtures are synthetic and built at run
time. Real transcripts contain source code and conversations.

**Rates are data, not code.** A price change is an edit to
`internal/pricing/pricing.json`, never a rebuild of logic.

## Commit messages

Conventional prefixes — `feat:`, `fix:`, `docs:`, `test:`, `chore:`,
`refactor:` — and a body explaining *why*, not what the diff already shows.

## Before opening a pull request

```sh
go run test/run.go
```

CI runs the suite on macOS, Linux, and Windows, plus the race detector and a
cross-compile check.
