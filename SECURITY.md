# Security Policy

## Supported versions

`cca` is pre-1.0. Security fixes land on `main` and in the next release; there
are no maintained release branches.

| Version | Supported |
| --- | --- |
| Latest release | Yes |
| `main` | Yes |
| Anything older | No — please upgrade |

## Reporting a vulnerability

**Do not open a public issue.**

Use GitHub's
[private vulnerability reporting](https://github.com/12fahed/cca/security/advisories/new),
or email **fahedkhan12092004@gmail.com** with `cca security` in the subject.

Please include what the issue is, how to reproduce it, and what an attacker
could achieve. A proof of concept helps, but a clear description is enough to
start.

Expect an acknowledgement within a week. You will be credited in the advisory
unless you would rather not be.

> [!CAUTION]
> Never attach a real transcript to a report. They contain your source code and
> your conversations. Construct a minimal synthetic file that reproduces the
> issue instead — `GETTING_STARTED.md` shows how.

## What counts as a vulnerability

`cca` is a local, offline, read-only tool, so its threat model is narrow and
specific. The following are in scope:

- **Anything that writes to, moves, or deletes files under `~/.claude`.** The
  tool guarantees it never does. A path that breaks that guarantee is a
  vulnerability, not a bug.
- **Any network activity.** `cca` must make no outbound connection ever. A
  build that does is a vulnerability.
- **Data leaking beyond the terminal** — transcript contents reaching a file, an
  environment variable, a log, or a subprocess.
- **Path traversal** via a crafted `--claude-dir`, `--pricing`, or config value
  that reads or writes outside the intended directory.
- **Crashes exploitable beyond denial of service**, such as a malformed
  transcript causing memory corruption or arbitrary code execution.
- **Supply chain issues** — a compromised release artifact, or a checksum that
  does not match its binary.

## What does not

- **An incorrect cost figure.** That is a correctness bug — please
  [open an issue](https://github.com/12fahed/cca/issues). The tool is explicit
  that its numbers are estimates.
- **A malformed transcript causing a skipped record.** Tolerating bad input by
  skipping it is the designed behaviour, and `--verbose` counts every skip.
- **`cca projects` revealing your directory names.** That is the feature. Be
  careful where you paste output.
- **Vulnerabilities in Claude Code itself.** Report those to Anthropic.

## Verifying a release

Every release ships a `checksums.txt`:

```sh
sha256sum --check --ignore-missing checksums.txt
```

Binaries are built by
[GitHub Actions](https://github.com/12fahed/cca/actions/workflows/release.yml)
from a tagged commit, so the workflow log shows exactly what was built and from
what source.

## Verifying the guarantees yourself

The two central promises are enforced by tests you can run:

```sh
go test ./internal/transcript/ -run 'NeverModifies|WritingCalls|OnlyReadOnly' -v
```

That hashes a directory tree before and after a full run and requires it to be
byte-identical, and separately parses the package to fail if any file-writing
call appears in it.

For the network claim, watch it yourself:

```sh
strace -f -e trace=network ./cca 2>&1 | grep -v ENOSYS
```
