# N1: Data Model Expansion — ToolCall + FileOp in TokenEvent

> **Workflow:** Follow `docs/PR-template.md`. Review `AGENTS.md` behavioral guidelines before implementing.

## Objective

Add `ToolCalls`, `FileOps`, `MessageRole`, `StopReason`, and `EventIndex` fields to `TokenEvent`. Update both source implementations (Claude Code JSONL, OpenCode SQLite) to populate them from raw data. No heuristic changes — all existing H1–H9 continue to work unchanged.

## Success Criteria

- [ ] `TokenEvent` struct extended with: `ToolCalls []ToolCall`, `FileOps []FileOp`, `MessageRole string`, `StopReason string`, `EventIndex int`
- [ ] `ToolCall` and `FileOp` types defined in `source/event.go`
- [ ] Claude source parses `tool_use` content blocks from `message.content`, extracts tool name + arguments (truncated at 1KB), extracts file paths from `Read`/`Write`/`Edit` tools
- [ ] Claude source normalizes file paths from absolute to relative (strips project root prefix)
- [ ] Claude source sets `EventIndex` sequentially per session file (1-based)
- [ ] Claude source sets `MessageRole` from `message.role` (or `"assistant"` for all assistant entries)
- [ ] OpenCode source joins `part` table to messages, extracts tool calls from parts where `json_extract(data, '$.type') = 'tool'`
- [ ] OpenCode source extracts file paths from tool part `state.input.filePath` for `read`/`write`/`edit` tools
- [ ] OpenCode source sets `EventIndex` sequentially per session
- [ ] All existing tests pass unchanged (new fields are zero-valued for old test data, old heuristics ignore them)
- [ ] Existing Claude fixture updated with `tool_use` content blocks; test assertion count still passes
- [ ] No heuristic changes — `DetectWaste` uses new fields in subsequent PRs

## Dependencies

- **Must merge first:** PR17 (calibration mode — clean main baseline)
- **External dependencies:** None
- **Can be parallel with:** None (foundation for N2, N3)
- **Breaking changes / Migrations needed:** `TokenEvent` struct change impacts all source implementations and test helpers that construct events. New fields are additive with zero values — backward compatible.

---

## Pre-flight

- [ ] Pull latest main: `git fetch origin && git checkout main && git pull origin main`
- [ ] Create feature branch: `git checkout -b n1-data-model-expansion`
- [ ] Verify build environment works on clean main

## Files

| File | Purpose | Notes |
|------|---------|-------|
| `source/event.go` | Add `ToolCall`, `FileOp` types. Extend `TokenEvent`. | ~35 lines |
| `source/claude.go` | Parse `tool_use` blocks from `message.content`. Extract file paths. Normalize paths. | ~100 lines |
| `source/claude_test.go` | Update test fixture path. Verify new fields populated. | ~60 lines |
| `source/opencode.go` | Join `part` table. Parse tool parts. Extract file paths. | ~90 lines |
| `source/opencode_test.go` | Update sample DB schema. Verify tool call extraction. | ~80 lines |
| `testdata/claude_projects/sample-project/<sid>.jsonl` | Updated fixture with `content` + `tool_use` blocks | Replace |
| `testdata/claude_projects/sample-project/<sid>/subagents/agent-*.jsonl` | Updated subagent fixture with `content` + `tool_use` | Replace |
| `config/config.go` | Add new signal toggles + thresholds (default `false`/`0`) | ~40 lines |
| `config/config_test.go` | Test new defaults | ~30 lines |
| `cmd/root.go` | Add new `--no-*` flags for H10–H13 | ~20 lines |

---

## Implementation

### `source/event.go` — New types + TokenEvent extension

```go
type ToolCall struct {
    Name      string // e.g. "Read", "Write", "Edit", "Glob", "Bash"
    Arguments string // JSON string of input args, truncated at 1KB
}

type FileOp struct {
    Path      string // normalized relative path from project root
    Operation string // "read", "write", "edit"
}

// Add to TokenEvent:
type TokenEvent struct {
    // ... existing fields (unchanged) ...

    ToolCalls   []ToolCall
    FileOps     []FileOp
    MessageRole string // "assistant", "user", "system"
    StopReason  string // "end_turn", "tool_use", "max_tokens", etc.
    EventIndex  int    // 1-based sequential position within session
}
```

**Constraints:**
- `Arguments` truncated at 1KB via `truncateString(s, 1024)` helper
- `FileOps` derived from `ToolCalls` by mapping known tool names to file operation types
- `MessageRole` defaults to `"assistant"` for all current assistant entries
- `EventIndex` resets per session file, starts at 1

### `source/claude.go` — Parse `tool_use` content blocks

**Struct changes:**

```go
type claudeMessage struct {
    Model   string               `json:"model"`
    Role    string               `json:"role"`
    Usage   *claudeUsage         `json:"usage"`
    Content []claudeContentBlock `json:"content"`
}
```

New types:

```go
type claudeContentBlock struct {
    Type  string          `json:"type"`
    Name  string          `json:"name"`
    Input json.RawMessage `json:"input"`
}
```

**`parseLine()` changes:**

After building the TokenEvent, populate new fields:

```go
// Extract tool calls and file ops from content blocks
var toolCalls []ToolCall
var fileOps []FileOp
for _, block := range entry.Message.Content {
    if block.Type != "tool_use" {
        continue
    }
    args := truncateString(string(block.Input), 1024)
    toolCalls = append(toolCalls, ToolCall{
        Name:      block.Name,
        Arguments: args,
    })
    // Map known tools to FileOps
    fo := fileOpFromClaudeTool(block.Name, block.Input, projRoot)
    if fo != nil {
        fileOps = append(fileOps, *fo)
    }
}
```

**`fileOpFromClaudeTool()` mapping:**

| Tool name | Condition | FileOp.Operation | Path extraction |
|-----------|-----------|-----------------|-----------------|
| `Read` | `input.file_path` exists | `"read"` | normalize `file_path` |
| `Write` | `input.file_path` exists | `"write"` | normalize `file_path` |
| `Edit` | `input.file_path` exists | `"edit"` | normalize `file_path` |
| `Glob` | — | no FileOp | (skip, no specific file) |
| `Bash` | — | no FileOp | (skip, not a file op) |
| any other | — | no FileOp | (skip) |

**Tool name canonicalization:**

```
Claude:  "Read"  → TokenEvent stores "read"
Claude:  "Write" → TokenEvent stores "write"
Claude:  "Edit"  → TokenEvent stores "edit"
Claude:  "Glob"  → TokenEvent stores "glob"
Claude:  "Bash"  → TokenEvent stores "bash"
Claude:  "Skill" → TokenEvent stores "skill"
OpenCode: "read" → TokenEvent stores "read" (already lowercase)
OpenCode: "write"→ TokenEvent stores "write"
OpenCode: "edit" → TokenEvent stores "edit"
OpenCode: "glob" → TokenEvent stores "glob"
```

`ToolCall.Name` is stored as lowercase. This makes H10 loop detection and H12/H13 file op matching harness-agnostic — `Read` and `read` are the same operation.

Implementation: `canonicalizeToolName(name string) string { return strings.ToLower(name) }`. Call in both `parseLine` (Claude) and `toolPartToEvent` (OpenCode).

**Path normalization:**

```go
// NormalizePath converts any file path to a canonical relative form.
// Claude Code emits absolute paths (/Users/hoang/project/src/main.go).
// OpenCode emits relative paths (src/main.go, ./src/main.go).
// Future harnesses may use Windows paths (src\main.go) or other conventions.
// After normalization, all paths are Unix-style relative from project root.
func NormalizePath(path, projectRoot string) string {
    // 1. Convert Windows separators
    path = strings.ReplaceAll(path, "\\", "/")
    // 2. Strip project root prefix (handles Claude absolute paths)
    if projectRoot != "" {
        path = strings.TrimPrefix(path, projectRoot)
        path = strings.TrimPrefix(path, "/") // trailing slash after stripping root
    }
    // 3. Strip leading ./
    path = strings.TrimPrefix(path, "./")
    // 4. Strip leading /
    path = strings.TrimPrefix(path, "/")
    // 5. Clean double slashes, .. segments
    path = filepath.Clean(path)
    if path == "." {
        return ""
    }
    return path
}
```

**Project root derivation:**

For Claude: derive from `projName` (e.g. `-Users-hoang-burnwatch`):
```go
func claudeProjectRoot(projName string) string {
    s := strings.TrimPrefix(projName, "-")
    s = strings.ReplaceAll(s, "-", "/")
    return "/" + s
}
```

For OpenCode: project root is not needed (paths are already relative). Pass `""` as projectRoot.

Call `NormalizePath` in both source implementations before storing in `FileOp.Path`. This ensures H12 (overlap) and H13 (restart) compare file paths correctly regardless of harness origin.

### `source/opencode.go` — Join `part` table

**Query change:**

The current query is:
```sql
SELECT s.id, s.parent_id, s.project_id, p.name, m.id AS message_id, m.data AS message_data, m.time_created
FROM message m
JOIN session s ON s.id = m.session_id
LEFT JOIN project p ON p.id = s.project_id
WHERE json_extract(m.data, '$.role') = 'assistant'
  AND json_extract(m.data, '$.tokens') IS NOT NULL
ORDER BY m.time_created
```

The tool call data is in a separate table `part`. We need to join it. Two approaches:

**Approach A — Two-pass:** Query messages as before, then separately query tool parts and merge in Go. Simpler, avoids message duplication.

**Approach B — Single query with LEFT JOIN:** Join part table. One message with 3 tool calls → 3 rows. Group in Go.

Use **Approach A** (two-pass). It maps cleanly to the current event iteration pattern:

```go
// Pass 1: Query tool parts
toolParts := make(map[string][]toolPart) // keyed by message_id
partRows, _ := db.Query(`
    SELECT p.message_id, p.data
    FROM part p
    WHERE json_extract(p.data, '$.type') = 'tool'
    ORDER BY p.time_created
`)
for partRows.Next() {
    var msgID, partData string
    partRows.Scan(&msgID, &partData)
    // ... parse partData, append to toolParts[msgID]
}

// Pass 2: Query messages (unchanged query)
for rows.Next() {
    // ... existing parsing ...
    // Attach tool calls from toolParts[messageID]
    for _, tp := range toolParts[messageID] {
        tc, fo := opencodeToolPartToEvent(tp)
        toolCalls = append(toolCalls, tc...)
        fileOps = append(fileOps, fo...)
    }
}
```

**`opencodeToolPartToEvent()` mapping:**

```go
type opencodePart struct {
    Type  string          `json:"type"`
    Tool  string          `json:"tool"`
    State opencodeState   `json:"state"`
}

type opencodeState struct {
    Input json.RawMessage `json:"input"`
}

type opencodeToolInput map[string]any
```

Parse the part data, check `type == "tool"`, extract tool name from `tool` field, extract input from `state.input`, extract file path from `filePath` key.

| Part.tool | Condition | FileOp.Operation | Path extraction |
|-----------|-----------|-----------------|-----------------|
| `read` | `state.input.filePath` exists | `"read"` | `state.input.filePath` |
| `write` | `state.input.filePath` exists | `"write"` | `state.input.filePath` |
| `edit` | `state.input.filePath` exists | `"edit"` | `state.input.filePath` |
| `glob` | — | no FileOp | (skip) |
| any other | — | no FileOp | (skip) |

**Path normalization for OpenCode:** OpenCode paths are already relative (e.g., `src/main.go`, `AGENTS.md`). No normalization needed. But to be safe, strip any leading `./` prefix.

**EventIndex:** Since the query is `ORDER BY m.time_created`, track per-session counter:

```go
eventIndex := make(map[string]int) // keyed by sessionID
// For each event:
eventIndex[sid]++
ev.EventIndex = eventIndex[sid]
```

**Error handling:**
- `part` table query fails → log error to errs channel, continue with messages only (graceful degradation)
- Part JSON unmarshal fails → log error, skip that part
- Tool name not recognized → still create ToolCall, just no FileOp

---

## Test Requirements

### `source/claude_test.go`

Update the existing `TestClaudeSource_Events` fixture and assertions:

1. **Fixture update**: The file at `testdata/claude_projects/sample-project/20834312-69fa-4496-8627-64c1865e9bcf.jsonl` now contains `content` blocks with `tool_use`. The subagent file also updated.

2. **Old assertions unchanged**:
   - 6 events total (4 top-level + 2 subagent)
   - First event model = `claude-sonnet-4-5-20250929`
   - First event harness = `claude-code`
   - First event input tokens = 3

3. **New assertions**:
   - First event has 1 ToolCall: `Name == "Read"`, `Arguments` contains `"file_path"`
   - First event has 1 FileOp: `Path == "src/main.go"`, `Operation == "read"`
   - First event `EventIndex == 1`
   - First event `MessageRole == "assistant"`
   - Third event (index 2 in file, 2nd assistant) has 2 ToolCalls (Edit + Glob)
   - Third event has 1 FileOp (Edit only; Glob doesn't map to FileOp)
   - Subagent events have correct `ParentSessionID`
   - Subagent event has ToolCalls and FileOps populated
   - Last assistant entry (text-only, no tool_use) has empty ToolCalls and FileOps

4. **Edge case tests** (new test functions):
   - `TestClaudeSource_ToolCallParsing`: Content block with `type: "thinking"` → skipped (not tool_use)
   - `TestClaudeSource_ArgumentsTruncation`: Arg string > 1024 bytes → truncated to 1024
   - `TestClaudeSource_FileOpMapping`: Write → `Operation == "write"`, correct path
   - `TestClaudeSource_PathNormalization`: `/Users/hoang/project/src/main.go` → `src/main.go`

### `source/opencode_test.go`

1. **Sample DB update**: The `testdata/opencode_sample.db` needs:
   - `part` table added to schema
   - At least 3 part rows for tool calls linked to existing messages
   - Part data uses the real OpenCode format: `{"type":"tool","tool":"read","state":{"input":{"filePath":"..."},...}}`

2. **New assertions**:
   - At least one event has `len(ToolCalls) > 0`
   - ToolCall name extracted correctly
   - FileOp path extracted from `filePath`
   - EventIndex sequential per session

3. **Edge case tests**:
   - `TestOpenCodeSource_PartParseFailure`: Corrupt part data → error channel, message still produced
   - `TestOpenCodeSource_MissingPartTable`: DB without part table → events produced normally, ToolCalls empty

### `config/config_test.go`

- Default `Signals.ToolLoop == false`, `FileReread == false`, `SubagentOverlap == false`, `SessionRestart == false`
- Default thresholds for new signals match expected conservative values

### `cmd/root_test.go` (new)

- `--no-tool-loop` disables `cfg.Signals.ToolLoop`
- Flag names match: `tool-loop`, `file-reread`, `subagent-overlap`, `session-restart`

---

## E2E Scenario Tests

N1 does not add heuristics — no new scenario tests needed. Existing scenario tests must continue to pass. The `parseScenarioLine` function WILL need updating to parse the new fields from scenario JSONL so that future PRs can use them.

### `parseScenarioLine` update

```go
type scenarioEntry struct {
    Type            string                `json:"type"`
    SessionID       string                `json:"sessionId"`
    ParentSessionID string                `json:"parentSessionId"`
    Message         scenarioMessage       `json:"message"`
    Timestamp       string                `json:"timestamp"`
}

type scenarioMessage struct {
    Model   string                 `json:"model"`
    Role    string                 `json:"role"`
    Usage   *scenarioUsage         `json:"usage"`
    Content []scenarioContentBlock `json:"content"`
}

type scenarioContentBlock struct {
    Type  string          `json:"type"`
    Name  string          `json:"name"`
    Input json.RawMessage `json:"input"`
}
```

In `parseScenarioLine`, populate:
- `ToolCalls` from `entry.Message.Content` blocks where `Type == "tool_use"`
- `FileOps` from tool calls using same `fileOpFromClaudeTool` mapping (scenarios use Claude-format JSONL)
- `MessageRole` from `entry.Message.Role`, default `"assistant"`
- `EventIndex` via per-session counter (or default 0 for now)
- `IsSubagent` from `entry.ParentSessionID != ""`
- `ParentSessionID` from `entry.ParentSessionID`

Existing scenarios (e.g., `cost_outlier.jsonl`) don't have `content` or `role` fields → zero values for new fields → backward compatible.

---

## Approach

1. Define `ToolCall`, `FileOp` types in `source/event.go`
2. Extend `TokenEvent` with new fields
3. Update `scenarioEntry` in `output/scenario_test.go` to include new fields (backward compatible parse)
4. Update `parseScenarioLine` to populate new fields from scenario JSONL
5. Write tests for Claude source parsing (RED)
6. Implement Claude source changes (GREEN)
7. Write tests for OpenCode source parsing (RED — needs sample DB update)
8. Update OpenCode sample DB with part table + test rows
9. Implement OpenCode source changes (GREEN)
10. Add new config fields (RED → GREEN)
11. Add new CLI flags
12. Run full test suite
13. Verify `go build` clean
14. Verify `golangci-lint run` clean

---

## Exit Criteria

- [ ] Pull latest main
- [ ] Create feature branch from main: `git checkout -b n1-data-model-expansion`
- [ ] Lint passes (zero warnings) — `golangci-lint run`
- [ ] Build compiles cleanly — `go build -o burnwatch .`
- [ ] Tests pass with >=90% coverage on new code — `go test ./... -cover`
- [ ] Self-review: run `./scripts/review-check.sh`, then verify Phases 1-3 in `docs/code-review.md`
- [ ] Document learnings (gotchas, mistakes, patterns, hidden coupling) in `docs/learnings.md`
- [ ] Commit: `feat: add ToolCall and FileOp fields to TokenEvent`
- [ ] Push to branch `n1-data-model-expansion`
- [ ] Open pull request with description
- [ ] Dispatch CodeReviewer subagent against the PR diff
- [ ] Update `docs/plans/progress.md` to reflect merge
- [ ] Merge to main
- [ ] Delete feature branch after merge

---

## Notes

- **Hidden coupling warning** (from learnings.md): "Adding a new field to TokenEvent requires touching: (1) all source implementations that create TokenEvent (claude.go, opencode.go), (2) all test helpers that construct events (scenario_test.go, bench_test.go), (3) all aggregation structs (sessionAgg in waste.go), (4) all output structs (WasteSignal, JSONWasteSignal)." The new fields are additive and zero-valued by default — existing heuristics and output won't read them, but the struct changes propagate to all those files.
- Claude Code tool names are capitalized: `Read`, `Write`, `Edit`, `Glob`, `Bash`. OpenCode tool names are lowercase: `read`, `write`, `edit`, `glob`. The mapping functions handle both.
- Arguments truncation at 1KB: `Edit` tool calls can have long `old_string`/`new_string` values. Loop detection compares arguments directly — truncation prevents memory issues while still catching loops (identical truncated args = likely same loop).
- Path normalization: Claude Code `file_path` values are absolute (`/Users/hoang/burnwatch/src/main.go`). The source derives the project root from the directory naming convention and strips it. OpenCode paths are already relative.
- The existing test fixture (`testdata/claude_projects/sample-project/20834312-69fa-4496-8627-64c1865e9bcf.jsonl`) and its subagent file have already been updated with `content` blocks + `tool_use` entries in this commit. The test assertions must match.
- The OpenCode sample DB `testdata/opencode_sample.db` needs the `part` table added. Create a .sql migration file at `testdata/opencode_part_migration.sql` that can be applied to the existing sample DB.
- `EventIndex` is 1-based. Session restart detection (H13) uses event ordering; 1-based is more intuitive for threshold comparison.
