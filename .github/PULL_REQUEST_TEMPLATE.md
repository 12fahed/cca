## What this changes

<!-- What does it do, and why? The diff shows what changed; explain why it
     should. If it fixes an issue, link it: Fixes #123 -->

## How it was verified

<!-- Which tests cover it? If a figure changes, what did you check it against? -->

```sh
go run test/run.go
```

## Checklist

- [ ] `go run test/run.go` passes
- [ ] Behaviour changes are covered by a test
- [ ] Documentation in `docs/` updated, if a flag, command or output changed
- [ ] Commit messages use a conventional prefix (`feat:`, `fix:`, `docs:`, ...)

## Constraints

<!-- Tick these or explain. See CONTRIBUTING.md. -->

- [ ] Nothing under `~/.claude` is written, moved, or deleted
- [ ] No network calls at runtime
- [ ] No new runtime dependency (or the reason is stated below)
- [ ] No real transcripts committed as fixtures
- [ ] Any figure that rests on an assumption prints that assumption

## Anything else

<!-- Trade-offs, things you were unsure about, follow-up work. -->
