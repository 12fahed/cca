# test

```
test/
├── run.go          single entry point: runs every check
└── unit/           cross-package tests, exported APIs only
```

## Running everything

```sh
go run test/run.go                      # fmt, vet, build, tests, race
go run test/run.go -short               # skip the race detector
go run test/run.go -cross               # also build all five release targets
go run test/run.go -oracle ~/.claude    # also sweep real cost-state records
```

`make test-all` runs the same thing.

The runner is a Go program rather than a shell script so that it works unchanged
on Windows, which the project supports and where a bash script would not run.

## Why the other tests are not in here

Go requires a test that touches unexported identifiers to live in the same
directory as the code it tests. cca's package-local suites do exactly that, on
purpose — the dedup key, the scanner buffer limit, the equivalence ladder, the
colour palette and the prefix matcher are internals whose edge cases carry the
project's correctness, and exporting them merely to relocate a file would widen
the public API for no benefit. `cmd/cca` is `package main`, which nothing outside
it can import at all.

So the package-local tests stay beside their packages:

| Location | Covers |
|---|---|
| `internal/transcript/` | discovery, streaming parse, dedup, cost-state, read-only safety |
| `internal/pricing/` | rate table, model resolution, fast mode, cost math, the oracle |
| `internal/report/` | windows, timezone boundaries, aggregation |
| `internal/render/` | formatting, tables, golden files, JSON/CSV agreement, colour |
| `internal/config/` | path discovery, file loading, flag-over-file-over-default |
| `internal/water/` | the estimate and its equivalence ladder |
| `cmd/cca/` | flag parsing, subcommand dispatch, exit codes |

`test/unit/` holds what none of those can see: the seam between packages, where
a transcript is parsed, priced, aggregated and rendered as one pipeline. Those
tests use only exported APIs, which is what lets them live outside.

## Fixtures

Synthetic only. Never copy a real transcript into a fixture — they contain the
user's source code and conversations. The pipeline tests build their corpus in a
temporary directory at run time; the renderer's golden files live in
`internal/render/testdata/`.
