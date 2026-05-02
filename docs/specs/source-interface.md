# Source Interface v1

## Contract

```go
type Source interface {
    Name() string
    Events() (<-chan TokenEvent, <-chan error)
}

func Discover() []Source
```

## Name()

Returns a short, lowercase identifier for the harness:
- `"opencode"`
- `"claude-code"`
- Future: `"codex"`, `"gemini-cli"`, `"aider"`

Used for filtering (`--harness opencode`) and display.

## Events()

Returns two channels:
- `<-chan TokenEvent`: Stream of parsed token events. Closed when all events are emitted.
- `<-chan error`: Non-fatal parse warnings. The source continues after emitting an error. Closed after the events channel.

Contract:
- The caller must read from both channels concurrently to avoid blocking.
- Error channel receives warnings (skipping malformed entries), not fatal errors (the source returns a fatal error by closing both channels immediately without emitting all events — the caller detects this via incomplete data).
- Events are emitted in chronological order (by timestamp).
- Both channels are closed when the source exhausts its data.
- The source owns the DB/file handle — opens it in `Events()`, closes it when done.

## Discover()

Detects installed harnesses by checking well-known paths:

```go
func Discover() []Source {
    home, _ := os.UserHomeDir()
    var sources []Source

    opencodeDB := filepath.Join(home, ".local", "share", "opencode", "opencode.db")
    if _, err := os.Stat(opencodeDB); err == nil {
        sources = append(sources, &OpenCodeSource{dbPath: opencodeDB})
    }

    claudeProjects := filepath.Join(home, ".claude", "projects")
    if fi, err := os.Stat(claudeProjects); err == nil && fi.IsDir() {
        sources = append(sources, &ClaudeCodeSource{projectDir: claudeProjects})
    }

    return sources
}
```

## TokenEvent

The universal event type. See [burnwatch-v1.md](./burnwatch-v1.md#tokenevent) for full schema.

Fields every source must populate:
- `SessionID` — required. Use the harness's native session identifier.
- `Timestamp` — required. Set to message time, or `time.Time{}` if unavailable.
- `InputTokens`, `OutputTokens` — required. Set to 0 if not tracked.
- `Harness` — required. Source's `Name()`.
- `IsSubagent` — required. True if `ParentSessionID != ""`.

Optional fields (set to zero value if unavailable):
- `ParentSessionID` — subagent link. Empty string for top-level sessions.
- `AgentType` — subagent type identifier. Empty for top-level.
- `CostUSD` — 0 if the harness doesn't pre-compute cost. Analysis layer fills it via pricing.
- `ReasoningTokens` — 0 if the harness doesn't track reasoning tokens.
- `Project` — derived from working directory or project table.

## Adding a new harness

1. Create `source/<harness>.go`.
2. Define a struct implementing `Source`.
3. Parse the harness's native data format into `TokenEvent`.
4. Handle errors gracefully — corrupt entries emit to error channel, not panic.
5. Register in `Discover()`.
6. Create `source/<harness>_test.go` with:
   - Table-driven parse tests (valid entries, missing fields, corrupt data).
   - Integration test against real (anonymized) test data.
7. Add `testdata/<harness>_sample.*` files.
8. Add pricing entries to `source/pricing.go` for any new models.
9. Update `README.md`.
10. Run `go test ./... -cover` — ensure ≥90% coverage on the new source file.

## Reference implementations

- [OpenCode source](/source/opencode.go) — SQLite query, JSON blob parsing.
- [Claude Code source](/source/claude.go) — JSONL streaming, subagent file discovery, pricing computation.
