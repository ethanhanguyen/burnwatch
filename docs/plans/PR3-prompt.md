# PR3: Claude Code Source

> **Workflow:** Follow `docs/plans/PR-template.md`. Review `AGENTS.md` behavioral guidelines before implementing.

## Objective

Implement the `Source` interface for Claude Code by reading its per-project session JSONL files and subagent JSONL files. Stream `TokenEvent` values with costs computed via the pricing table from PR1.

## Files to create

| File | Purpose |
|------|---------|
| `source/claude.go` | JSONL reader implementing `Source` interface |
| `source/claude_test.go` | Tests against anonymized test JSONL |
| `testdata/claude_sample.jsonl` | Anonymized session JSONL (multi-line, multi-type) |
| `testdata/claude_subagents/` | Anonymized subagent JSONL files |

## Dependencies

PR1 must be merged. PR3 can be built in parallel with PR2 and PR4.

## `source/claude.go`

### Data locations

```go
func defaultProjectDir() string {
    home, _ := os.UserHomeDir()
    return filepath.Join(home, ".claude", "projects")
}
```

Claude Code stores sessions as:
```
~/.claude/projects/
  -Users-hoang-lilysbeauty/              # project directory
    20834312-69fa-4496-8627-64c1865e9bcf.jsonl   # session file
    035c1d57-6d23-4221-97d8-2985ff441548/         # session directory
      subagents/
        agent-aa46fa6e6b4c8fb82.jsonl              # subagent session
        agent-aa46fa6e6b4c8fb82.meta.json          # metadata
```

### Algorithm

1. Walk `~/.claude/projects/` — each subdirectory is a project.
2. Within each project, find all `.jsonl` files at root level (these are session files).
3. For each session file, parse entries where `type == "assistant"`.
4. Extract `message.usage.input_tokens`, `.output_tokens`, `.cache_creation_input_tokens`, `.cache_read_input_tokens`.
5. Also check for matching subagent directories (`<session-uuid>/subagents/agent-*.jsonl`).
6. Compute `CostUSD` via `pricing.CostForModel()`.
7. Derive project name from directory name (strip `-` prefix, replace `-` with `/`).

### Parsing a session JSONL entry

```json
{
  "type": "assistant",
  "sessionId": "20834312-69fa-4496-8627-64c1865e9bcf",
  "message": {
    "model": "claude-sonnet-4-5-20250929",
    "usage": {
      "input_tokens": 3,
      "cache_creation_input_tokens": 48390,
      "cache_read_input_tokens": 0,
      "output_tokens": 187
    }
  },
  "timestamp": "2026-04-13T03:35:32.001Z"
}
```

### Subagent JSONL entry

```json
{
  "type": "assistant",
  "sessionId": "...",
  "agentId": "agent-aa46fa6e6b4c8fb82",
  "slug": "clever-squid",
  "message": {
    "model": "claude-haiku-4-5-20251001",
    "usage": {
      "input_tokens": 3,
      "cache_creation_input_tokens": 10895,
      "cache_read_input_tokens": 0,
      "output_tokens": 1
    }
  }
}
```

### TokenEvent mapping

```go
TokenEvent{
    SessionID:       entry.sessionId,
    ParentSessionID: parentSessionID,   // set for subagent files, "" for top-level
    AgentType:       entry.agentId,     // subagent identifier
    Model:           entry.message.model,
    Provider:        "anthropic",
    Timestamp:       parseTime(entry.timestamp),
    InputTokens:     entry.message.usage.input_tokens,
    OutputTokens:    entry.message.usage.output_tokens,
    CacheRead:       entry.message.usage.cache_read_input_tokens,
    CacheWrite:      entry.message.usage.cache_creation_input_tokens,
    ReasoningTokens: 0,                 // Claude doesn't have reasoning tokens
    CostUSD:         pricing.CostForModel(model, input, output, cacheRead, cacheWrite),
    Project:         deriveProjectName(dirName),
    Harness:         "claude-code",
    IsSubagent:      isSubagentFile,
}
```

### Error handling

- Corrupt JSON line → skip, emit to error channel.
- Missing `message.usage` → skip (not an API call — could be a title message).
- Missing `message.model` → skip.
- Timestamp parse failure → use `time.Time{}`.
- Session file not found → skip project, continue.
- Empty project directory → skip.
- Subagent directory not found → just process top-level session, don't fail.

### Performance

For large session files (14MB, 4000+ lines seen in user data), stream line-by-line — don't load the whole file into memory. Use `bufio.Scanner`.

## Test requirements (`source/claude_test.go`)

1. **Integration test**: Parse `testdata/claude_sample.jsonl` + `testdata/claude_subagents/`.
   - Verify correct event count.
   - Verify first event has correct model, tokens, cost > 0.
   - Verify subagent events have `IsSubagent == true`, `AgentType` populated.
   - Verify project name derived correctly from directory name.

2. **Table-driven tests** for JSON parsing:
   - Well-formed assistant entry → correct TokenEvent.
   - Non-assistant entry (`user`, `queue-operation`, `attachment`) → skipped.
   - Missing `message.usage` → skipped.
   - Malformed JSON line → skipped, error channel receives message.
   - Entry with all-zero tokens → valid event with zero cost.

3. **Cost computation test**: Verify `CostUSD` matches expected value from pricing table for known model + token counts.

4. **Discovery test**: Verify `Discover()` includes Claude Code source when `~/.claude/projects/` exists.

5. **Edge cases**:
   - Empty JSONL file → clean exit.
   - File with only non-assistant entries → zero events.
   - Very large file (14MB) → no memory leak, completes in <5s.

**Coverage target**: ≥90% on `claude.go`.

## Approach: TDD

1. Create `testdata/claude_sample.jsonl` by extracting one real session, anonymizing project paths, obfuscating prompt content.
2. Create `testdata/claude_subagents/` with one anonymized subagent session.
3. Write tests first (RED).
4. Implement `claude.go` (GREEN).
5. Verify coverage, add tests for missed branches.

## Exit criteria

- [ ] Pull latest main
- [ ] Create feature branch from main
- [ ] `go test ./source/... -cover` passes with ≥90% coverage on `claude.go`
- [ ] `go vet ./...` zero warnings
- [ ] `golangci-lint run` zero issues
- [ ] Update `README.md` "Supported Harnesses" section to include Claude Code
- [ ] Self-review: follow behavioral guidelines in `AGENTS.md`
- [ ] Commit: `feat: add Claude Code source (JSONL reader with subagent discovery)`
- [ ] Push to branch `pr3-claude-source`
- [ ] Open pull request
- [ ] Perform code review
- [ ] Merge to main
- [ ] Delete feature branch after merge
