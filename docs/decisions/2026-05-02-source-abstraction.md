# ADR: Source Abstraction — TokenEvent + Interface

**Date**: 2026-05-02
**Status**: Accepted

## Context

Burnwatch supports multiple AI agent harnesses (OpenCode, Claude Code, and future additions). Each harness stores session data differently:

- OpenCode: SQLite database with relational schema (sessions, messages, projects)
- Claude Code: Per-project JSONL files with embedded usage data
- (Future) Codex: Session JSON files
- (Future) Gemini CLI: Yet another format

We need a way to ingest all of these into a single analysis pipeline without duplicating the waste detection logic.

## Decision

Define a common `TokenEvent` type and a `Source` interface. Each harness implements `Source`, mapping its native schema to `TokenEvent`. The analysis pipeline operates on `[]TokenEvent` independently of where the data came from.

```go
type Source interface {
    Name() string
    Events() (<-chan TokenEvent, <-chan error)
}
```

Key design choices:
1. **Least common denominator fields**: `TokenEvent` includes fields that all harnesses can populate. Harness-specific data (e.g., Claude Code's `stop_reason`, OpenCode's `mode`) is dropped. Acceptable for v1 — the waste heuristics don't need that data.
2. **Streaming via channels**: `Events()` returns channels, not a slice. This lets sources emit events incrementally (don't load 14MB JSONL files into memory). The analysis layer collects into a slice when it needs full data (for percentile computation).
3. **Error channel**: Non-fatal parse errors (corrupt JSON, missing fields) go to a separate error channel. The source continues processing. This prevents one bad entry from killing the entire run.

## Alternatives considered

### Per-harness analysis
- **Rejected**: Each harness would have its own baseline, waste, and output code. 5 harnesses × 5 files = 25 files vs current 5 files + 5 source files. Duplication that diverges over time.

### Common log format
- **Rejected**: Would require harnesses to adopt a standard output format. Burnwatch can't mandate this. Parse-adapters are more practical than format standards.

### Adapter pattern per harness
- **Rejected**: Same as Source interface but with more boilerplate. The Source interface IS the adapter — it maps native → TokenEvent.

## Consequences

### Positive
- Adding a harness = ~100 lines of Go (implement Source + register in Discover).
- Waste detection logic is harness-agnostic. Bug fixes apply to all harnesses.
- Can compare efficiency across harnesses (e.g., "My OpenCode sessions are 30% cheaper than Claude Code sessions").

### Negative
- Harness-specific data is lost. If a future heuristic needs Claude Code's `stop_reason`, we'd need to add an `Extra map[string]any` field to `TokenEvent` (acceptable extension).
- The interface assumes events are independent. For Claude Code's conversation threading (parent_uuid), we lose the message tree. The subagent tree is preserved via `ParentSessionID`.
- Pricing tables must be maintained separately for each harness's model set. OpenCode uses `providerID:modelID`, Claude Code uses model names directly. Normalization needed.

## When to revisit

- When a heuristic needs harness-specific data → add `Extra map[string]any` to `TokenEvent`.
- When streaming becomes a bottleneck (50K+ events) → consider batching events into pages.
- When the list of supported harnesses exceeds 5 → consider a plugin system (dynamic loading of source `.so` files).
