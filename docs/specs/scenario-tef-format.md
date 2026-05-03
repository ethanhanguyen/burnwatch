# Scenario Format: Token Event Format (TEF)

> How scenario fixtures work across harnesses. Read this before writing scenario JSONL files.

## Design

Scenario tests exercise heuristics, not source parsers. The pipeline is:

```
JSONL (TEF format) → parseScenarioLine() → TokenEvent → heuristics → WasteSignal → assertions
```

The JSONL format looks like Claude Code data (fields like `type: "assistant"`, `message.usage`, `message.content`), but the output is harness-agnostic `TokenEvent` values. The scenario pipeline does NOT run through Claude or OpenCode source parsers — it constructs TokenEvent directly.

**Source parsers are tested independently** in `source/claude_test.go` and `source/opencode_test.go`. Those tests verify that each harness produces equivalent TokenEvent values from equivalent raw data.

**The behavioral heuristics operate on TokenEvent.** They are harness-agnostic by design. A loop detection test passes regardless of whether the data came from Claude or OpenCode, because the heuristics only see normalized TokenEvent fields.

## Why this works across harnesses

| Concern | Solution | Where |
|---------|----------|-------|
| Tool names differ: `Read` (Claude) vs `read` (OpenCode) | Source layer canonicalizes to lowercase before storing in `ToolCall.Name` | N1: `source/claude.go`, `source/opencode.go` |
| File paths differ: `/Users/hoang/project/src/main.go` vs `src/main.go` | Source layer normalizes to relative paths before storing in `FileOp.Path` | N1: `source/claude.go`, `source/opencode.go` |
| OpenCode uses `part` table, Claude uses inline `content` | Source implementations parse their native formats into the same `ToolCall`/`FileOp` types | N1: each source |
| Heuristics compare tool names and file paths | Heuristics operate on normalized values in `TokenEvent` | N2/N3: `analyze/loop.go`, `analyze/reread.go`, etc. |

## TEF JSONL format

Each line is a JSON object with these fields:

```jsonc
{
  "type": "assistant",                        // required, only "assistant" lines are loaded
  "sessionId": "ses_example_waste",           // required, maps to TokenEvent.SessionID
  "parentSessionId": "ses_example_parent",    // optional, maps to TokenEvent.ParentSessionID
  "timestamp": "2026-05-03T10:00:00.000Z",    // optional, defaults to 2026-05-01T10:00:00Z
  "message": {
    "model": "claude-sonnet-4-5-20250929",    // required, used for CostForModel pricing
    "role": "assistant",                      // optional, defaults to "assistant"
    "usage": {
      "input_tokens": 100,                    // required
      "output_tokens": 50,                    // required
      "cache_creation_input_tokens": 5000,    // required
      "cache_read_input_tokens": 5000         // required
    },
    "content": [                              // optional, arrays of tool_use blocks
      {
        "type": "tool_use",
        "name": "Read",                       // canonicalized: lowercase in TokenEvent
        "input": {"file_path": "src/main.go"} // raw input JSON, truncated at 1KB
      }
    ]
  }
}
```

## Field mapping to TokenEvent

| TEF field | TokenEvent field | Notes |
|-----------|-----------------|-------|
| `sessionId` | `SessionID` | |
| `parentSessionId` | `ParentSessionID` | If present, `IsSubagent = true` |
| `timestamp` | `Timestamp` | RFC3339 parsed |
| `message.model` | `Model` | |
| `message.role` | `MessageRole` | Default `"assistant"` |
| `message.usage.*` | `InputTokens`, `OutputTokens`, `CacheRead`, `CacheWrite` | |
| `message.content[].name` | `ToolCalls[].Name` | Lowercased by `canonicalizeToolName()` |
| `message.content[].input` | `ToolCalls[].Arguments` | JSON-stringified, truncated at 1KB |
| Computed from `tool_use` blocks | `FileOps[].Path` | Extracted via `extractFilePath()` |
| Computed from `tool_use` blocks | `FileOps[].Operation` | Mapped via `mapToolToFileOp()` |
| Per-session counter | `EventIndex` | Sequential 1-based within session |
| `CostForModel(model, tokens)` | `CostUSD`, `CostApproximate` | |
| Fixed | `Project = "scenario-test"`, `Harness = "claude-code"`, `Provider = "test"` | |

## Adding a scenario for a new harness

No changes needed. Scenario tests are harness-agnostic — they test heuristics via TokenEvent. To validate a new harness's source parser, write unit tests in `source/<harness>_test.go` that assert the parser produces equivalent TokenEvent values for known inputs.

## Cross-harness validation

N4 adds `TestCrossHarness_EquivalentEvent` in `source/cross_test.go`:
- Loads the same logical session from both Claude JSONL and OpenCode SQLite
- Asserts that `TokenEvent` fields are equivalent after normalization
- Covers: tool calls, file ops, paths, costs, session identity

## File naming

- `<heuristic>.jsonl` — positive test (exactly one waste session triggers the heuristic)
- `*_edge.jsonl` — boundary cases (threshold, below-threshold, multi-subagent, chains)
- `*_mixed.jsonl` — mixed waste/normal scenarios within one file
