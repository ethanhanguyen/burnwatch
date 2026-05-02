# PR2: OpenCode Source

> **Workflow:** Follow `docs/plans/PR-template.md`. Review `AGENTS.md` behavioral guidelines before implementing.

## Objective

Implement the `Source` interface for OpenCode by reading its SQLite database (`~/.local/share/opencode/opencode.db`). Stream `TokenEvent` values from the `message`, `session`, and `project` tables.

## Files to create

| File | Purpose |
|------|---------|
| `source/opencode.go` | SQLite reader implementing `Source` interface |
| `source/opencode_test.go` | Tests against anonymized test DB |
| `testdata/opencode_sample.db` | Anonymized 10-session SQLite DB |

## Dependencies

PR1 must be merged first. Use `go get` for the SQLite driver — use `modernc.org/sqlite` (pure Go, no CGO):

```bash
go get modernc.org/sqlite
```

## `source/opencode.go`

### Paths to check
```go
func defaultDBPath() string {
    home, _ := os.UserHomeDir()
    return filepath.Join(home, ".local", "share", "opencode", "opencode.db")
}
```

### SQL query

```sql
SELECT
    s.id,
    s.parent_id,
    s.project_id,
    s.slug,
    p.name AS project_name,
    m.id AS message_id,
    m.data AS message_data,
    m.time_created
FROM message m
JOIN session s ON s.id = m.session_id
LEFT JOIN project p ON p.id = s.project_id
WHERE json_extract(m.data, '$.role') = 'assistant'
  AND json_extract(m.data, '$.tokens') IS NOT NULL
ORDER BY m.time_created
```

### Parsing `message.data`

Each `data` field is a JSON blob with:
```json
{
  "role": "assistant",
  "agent": "build",
  "modelID": "google/gemini-3-pro-preview",
  "providerID": "vercel",
  "cost": 0.093594,
  "tokens": {
    "total": 45122,
    "input": 44787,
    "output": 22,
    "reasoning": 313,
    "cache": { "write": 0, "read": 0 }
  },
  "time": { "created": 1775925856369 }
}
```

### TokenEvent mapping

```go
TokenEvent{
    SessionID:       session.id,
    ParentSessionID: session.parent_id,
    AgentType:       data.agent,
    Model:           data.modelID,
    Provider:        data.providerID,
    Timestamp:       time.UnixMilli(data.time.created),
    InputTokens:     data.tokens.input,
    OutputTokens:    data.tokens.output,
    CacheRead:       data.tokens.cache.read,
    CacheWrite:      data.tokens.cache.write,
    ReasoningTokens: data.tokens.reasoning,
    CostUSD:         data.cost,         // OpenCode pre-computes
    Project:         project.name,
    Harness:         "opencode",
    IsSubagent:      session.parent_id != "",
}
```

### Error handling

- Missing/corrupt `data` blob → skip that message, log to error channel.
- Missing `tokens` field → skip.
- DB connection failure → return error, close channels.
- Empty result set (no assistant messages with tokens) → close channels cleanly, no error.
- Missing project name → use project_id as fallback.

### Streaming

The `Events()` method should:
1. Open DB connection.
2. Execute query.
3. Stream rows one at a time over the events channel.
4. Close both channels when done.
5. Use `defer` to ensure DB is always closed.

## `testdata/opencode_sample.db`

Create an anonymized SQLite DB from the user's actual OpenCode data:
1. Copy schema only from `~/.local/share/opencode/opencode.db`.
2. Select 10 representative sessions with:
   - At least 2 with subagents (parent_id not null)
   - At least 2 from different projects
   - Mix of agent types (build, explore, general)
   - Mix of models
3. Anonymize: replace real project names with generic ones ("project-a", "project-b"), replace slugs similarly, replace dollar costs with representative values.

## Test requirements (`source/opencode_test.go`)

1. **Integration test**: Run `OpenCodeSource.Events()` against `testdata/opencode_sample.db`.
   - Verify total event count matches expected.
   - Verify first and last events have correct fields.
   - Verify subagent events have `IsSubagent == true` and `ParentSessionID != ""`.
   - Verify project names are populated.
   - Verify cost > 0 for all events.

2. **Table-driven tests** for JSON parsing:
   - Well-formed message JSON → correct TokenEvent.
   - Missing `tokens` → skipped (no event).
   - Corrupt JSON → skipped (error channel receives message).
   - Missing optional fields (agent, cost) → zero values.

3. **Discovery test**: Verify `Discover()` includes an OpenCode source when the DB exists.

4. **Edge case**: DB with zero assistant messages → clean exit, no panic.

**Coverage target**: ≥90% on `opencode.go`.

## Approach: TDD

1. Create `testdata/opencode_sample.db` manually (or via a Go helper in a `_test.go` file that creates it in `TestMain`).
2. Write tests first (they'll fail — no implementation).
3. Implement `opencode.go` until tests pass.
4. Run coverage check, add tests for any uncovered branches.

## Exit criteria

- [ ] Pull latest main
- [ ] Create feature branch from main
- [ ] `go test ./source/... -cover` passes with ≥90% coverage on `opencode.go`
- [ ] `go vet ./...` zero warnings
- [ ] `golangci-lint run` zero issues
- [ ] Update `README.md` with "Supported Harnesses" section listing OpenCode
- [ ] Self-review: follow behavioral guidelines in `AGENTS.md`
- [ ] Commit: `feat: add OpenCode source (SQLite reader)`
- [ ] Push to branch `pr2-opencode-source`
- [ ] Open pull request
- [ ] Perform code review
- [ ] Merge to main
- [ ] Delete feature branch after merge

## Notes

- Use `database/sql` with `modernc.org/sqlite` driver. Register the driver in an `init()` or in the constructor.
- The `data` field is JSON text — unmarshal with `encoding/json` into a struct with `json:"cost"`, etc.
- Do not keep the DB connection open across `Events()` calls. Open in `Events()`, close when done.
- Make DB path configurable via an environment variable `BURNWATCH_OPENCODE_DB` for testing.
