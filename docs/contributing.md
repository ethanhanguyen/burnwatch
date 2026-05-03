# Contributing

## Local setup

```bash
git clone https://github.com/ethanhanguyen/burnwatch
cd burnwatch
go mod download
```

## Testing

```bash
go test ./... -cover              # all tests with coverage
go test ./... -cover -coverprofile=coverage.out
go tool cover -html=coverage.out  # visual coverage report
```

Coverage target: ≥90% across all packages.

## Linting

```bash
go vet ./...
golangci-lint run
```

## Adding a new harness

1. Create `source/<harness>.go` implementing the `Source` interface.
2. Create `source/<harness>_test.go` with table-driven parse tests and integration tests.
3. Add testdata for the harness.
4. Add pricing entries to `source/pricing.go` if the harness uses new models.
5. Register auto-discovery in `source/interface.go` `Discover()`.
6. Update [`README.md`](/README.md) "Supported Harnesses" section.
7. Run full test suite and verify golden files still pass.

### Source interface

```go
type Source interface {
    Name() string
    Events() (<-chan TokenEvent, <-chan error)
}
```

- `Name()` returns a short identifier (`"opencode"`, `"claude-code"`, etc.).
- `Events()` streams parsed token events. The error channel receives non-fatal parse warnings (skip that entry, continue). Close both channels when done.
- Auto-discovery: add a well-known path check in `Discover()`. Use `os.UserHomeDir()` for paths.

### Testdata conventions

- Anonymize: replace real project names, session slugs, and user content.
- Keep representative: include subagent sessions, multiple projects, varied models.
- Keep small: 10 sessions max per test file, under 1MB total.

## Release

```bash
git tag vX.Y.Z
git push --tags
```

`go install github.com/ethanhanguyen/burnwatch@vX.Y.Z` picks up the new version automatically.

## Doc rules

- `docs/index.md` is the entrypoint — keep it short.
- Move superseded plans/specs into `docs/archive/`.
- Every ADR goes in `docs/decisions/` with date prefix.
- Active specs go in `docs/specs/`.
- Implementation plans go in `docs/plans/`.
