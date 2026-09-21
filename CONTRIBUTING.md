# Contributing to cca

Thanks for considering it. `cca` is a small, deliberately conservative tool, and
this document explains what that means in practice so your time is not wasted on
a change that was never going to land.

New to the codebase? [GETTING_STARTED.md](GETTING_STARTED.md) walks through
building it, running the tests, and making a first change.

By contributing, you agree that your work is distributed under the
[GPL-3.0](LICENSE), and you are expected to follow the
[Code of Conduct](CODE_OF_CONDUCT.md).

## Quick reference

```sh
git clone https://github.com/12fahed/cca.git
cd cca
go run test/run.go     # fmt, vet, build, tests, race detector — run before every PR
```

| Command | What it does |
| --- | --- |
| `make build` | Build `./cca` |
| `make test` | `go test ./...` |
| `make test-race` | Tests under the race detector |
| `make test-all` | Everything CI runs |
| `make build-all` | All five release targets into `dist/` |
| `go run test/run.go -cross` | Everything, plus a cross-compile check |

## Reporting a bug

Open an [issue](https://github.com/12fahed/cca/issues) and include the output of:

```sh
cca version
cca --verbose
```

The `--verbose` block shows what was scanned, what was dropped, and why, which
is usually enough to identify the problem.

> [!CAUTION]
> `cca --verbose` prints your project directory names. Redact them before
> posting if that matters to you. **Never attach a real transcript** — they
> contain your source code and your conversations.

If a figure looks wrong, say what you expected and why. "The total seems high"
is hard to act on; "deduplication reports 0% on a corpus with resumed sessions"
is a bug report.

## Proposing a change

For anything beyond a typo, **open an issue first**. A short discussion is
cheaper than a rejected pull request, especially for the constraints below.

## Constraints that are not up for negotiation

These come from the project's purpose, and a change that breaks one will be
declined however good it otherwise is.

### Never write to `~/.claude`

`cca` analyses files containing your source code and your conversations. It
reads them and nothing else — no lock files, no caches, no scratch files, no
"just this once".

This is enforced two ways, and both must keep passing:

- **Behaviourally** — the directory tree is hashed before and after a run and
  must come back byte-identical, with nothing created and nothing removed.
- **Structurally** — `internal/transcript` is parsed and the test fails if any
  file-writing standard library call appears in it, or if a file is opened with
  anything other than `os.Open`.

### No network calls at runtime

The rate table is compiled into the binary. `cca` must work fully offline, with
no telemetry, no analytics, and no update check.

### Zero runtime dependencies

The shipped artifact is a single static binary built with `CGO_ENABLED=0`.
Adding a dependency needs a stated reason in the pull request, and "it is
convenient" is not one. The standard library has been enough so far.

### Never commit real transcripts

Fixtures are synthetic and built at run time. This is not negotiable for the
same reason as the first constraint.

### Honesty about accuracy

Every figure that is uncertain says so, in the output itself rather than only in
the documentation. The cost caveat and the water assumption are printed on every
run. A change that removes a caveat, or makes a soft number look precise, will
be declined.

If you add a calculation that rests on an assumption, print the assumption.

## Code conventions

**Tests live beside the code they test.** Go requires it for anything touching
unexported identifiers, and `cca`'s tests deliberately do — the dedup key, the
scanner buffer limit, the prefix matcher. `test/unit/` holds only what reaches
across packages. See [test/README.md](test/README.md).

**Comments explain why, not what.** The code says what it does. A comment earns
its place by recording a decision, a constraint, or a trap — why `tabwriter` is
not used for styled tables, why prefix matching requires a dated suffix.

**`cmd/cca` holds no business logic.** Flag parsing, dispatch, exit codes, and
wiring only. Logic belongs in an `internal/` package where it can be tested
directly.

**Rates are data.** A price change is an edit to
`internal/pricing/pricing.json`, never a change to logic.

**Keep the default view to roughly one screen.** There is a test for this.

## Tests

Every change that alters behaviour needs a test. Beyond that:

- **Table-driven** where there are several cases.
- **Test the property, not the snapshot**, where you can. "Every grouping sums
  to the total" survives refactoring; a golden file does not.
- **Golden files** are for rendered output. Regenerate with
  `go test ./internal/render -update` and **read the diff** before committing —
  that is the point of them.
- **Hermetic.** No test may read the real `~/.claude` or the real config
  directory. Use `t.TempDir()` and `--claude-dir`.

Verify before pushing:

```sh
go run test/run.go
```

CI runs the suite on macOS, Linux and Windows, plus the race detector and a
cross-compile check.

## Commit messages

Conventional prefixes:

```
feat:     a user-visible capability
fix:      a bug fix
docs:     documentation only
test:     tests only
refactor: no behaviour change
chore:    tooling, build, dependencies
perf:     a performance change
```

Scope in parentheses where it helps: `feat(pricing):`, `fix(render):`.

The body should explain **why**, not restate the diff. If you worked something
out — a format quirk, a wrong assumption, a trap — write it down. That reasoning
is the part that is expensive to recover later.

## Pull requests

1. Branch from `main`.
2. Keep it focused. One concern per pull request.
3. Run `go run test/run.go`.
4. Fill in the pull request template.
5. Explain the reasoning, not just the change.

Review looks for: correctness, whether the constraints above still hold, test
coverage of the new behaviour, and whether comments explain decisions rather
than narrate code.

## Adding a model to the rate table

The most common contribution, and it is data-only:

1. Add an entry to `internal/pricing/pricing.json` with absolute USD per million
   tokens for all five classes.
2. **Cite your source** in the pull request — a link to the published price
   sheet. Rates are never inferred.
3. Add the model to the pinned-rates test in
   `internal/pricing/table_test.go`, which exists so a silent edit cannot change
   every figure the tool reports.
4. If its id is a prefix of another model's, or vice versa, add a resolution
   test. This has bitten before: `claude-fable-5` and `claude-fable-5-1` differ
   only in cache reads, so a wrong match still produces a plausible total.

## Documentation

User documentation lives in [`docs/`](docs/) and is built with Starlight:

```sh
cd docs && npm install && npm run dev
```

If you change a flag, a command, or an output format, update the documentation
in the same pull request. The rate table page is generated from
`pricing.json`, so it needs no manual edit.

## Security

Do not open a public issue for a vulnerability. See [SECURITY.md](SECURITY.md).

## Questions

Open a [discussion](https://github.com/12fahed/cca/discussions) or an issue. A
question that was hard to answer usually means the documentation has a gap, so
asking is useful in itself.
