# ADR: Pivot from Cost Anomaly Detection to Event-Level Waste Detection

**Date**: 2026-05-03
**Status**: Accepted
**Supersedes**: 2026-05-02-v2-assessment.md (Phase B and C)
**Implemented in**: PRs N1–N4. See `docs/plans/N1-prompt.md` through `docs/plans/N4-prompt.md`.

---

## 1. Current State

### What burnwatch does today

The pipeline ingests session data from Claude Code and OpenCode, reduces every assistant message to token counts, aggregates to per-session metrics, and applies 8 statistical heuristics:

```
Harness session data → TokenEvent (token counts only) → sessionAgg → 8 univariate heuristics → WasteSignal
```

The heuristics check one dimension each:

| Heuristic | What it measures | What it claims to catch |
|-----------|-----------------|----------------------|
| cost_outlier | session cost > μ + 2σ | runaway sessions, wrong model |
| low_signal | output/input ratio < P10 | reading without producing |
| subagent_overhead | subagent cost > 50% | over-delegation |
| cache_underutilized | cache hits < P10 | not using prompt caching |
| input_overconsumption | input tokens > μ + 2σ | context bloat |
| output_explosion | output tokens > μ + 2σ | runaway generation |
| token_efficiency | TER < P10 | low useful-work-per-token |
| fragmentation_index | sessions/day × (1−ratio) | churn, context loss |

### What data flows through the pipeline

The `TokenEvent` struct carries 15 fields. The ingestion layer strips everything except token counts, model, timestamp, and session identity:

| Preserved | Discarded |
|-----------|-----------|
| SessionID, ParentSessionID, AgentType | Tool call names and arguments |
| Model, Provider | File paths (read, write, edit) |
| InputTokens, OutputTokens | Message content and role |
| CacheRead, CacheWrite | Stop reasons |
| ReasoningTokens | Tool call results |
| CostUSD | User messages and system prompts |
| Project, Harness, IsSubagent | Operation sequences and timings |

This was an intentional design choice (ADR 2026-05-02): "Least common denominator fields. Harness-specific data is dropped. Acceptable for v1 — the waste heuristics don't need that data."

### What the current pipeline CANNOT detect

| Waste type | Detectable? | Why not |
|-----------|-------------|---------|
| Agent looping (same action repeated) | No | Tool call sequences are discarded. A session that calls `read_file("foo.go")` 12 times looks identical to one that reads 12 different files. |
| Re-reading files without caching | Partially | Cache hit rate flags low cache utilization, but cannot identify which files are being re-read or whether caching would have helped. |
| Subagents re-doing parent's work | No | No file operation data. Cannot compare parent's reads/writes against subagent's reads/writes. |
| Starting fresh instead of continuing | Yes | H9 catches this via session count × low-ratio. The one waste type the data actually supports. |
| Model-task mismatch | No | A cheap model struggling with a complex task produces more tokens, but a sigma-threshold can't distinguish "legitimate complex task" from "wrong model choice." |

### What the current pipeline actually measures

The pipeline detects **statistically unusual token/cost patterns**, not waste. Every heuristic asks: "is this session different from the average?" The answer may or may not mean waste:

- A session 3σ above mean cost could be a complex architecture migration (not waste) or an infinite retry loop (waste). The heuristic cannot tell.
- A session at P95 on tokens but P5 on cache could be a first-time codebase exploration (not waste) or a broken caching configuration (waste). The heuristic cannot tell.

The pipeline conflates "unusual" with "wasteful." The isolation forest (planned PR18) would extend this same conflation to multivariate space — it would detect "unusual in the joint feature space" but still couldn't distinguish waste from legitimate complexity.

---

## 2. The Gap

### Stated mission

> "Find sessions where AI agents wasted resources through looping, re-reading files without caching, delegating to subagents that re-do work, or starting fresh instead of continuing."

### Actual capability

> "Find sessions whose aggregate token/cost metrics differ from the project baseline."

### Root cause

The data poured into the pipeline does not carry the information needed to answer the mission's questions. Token counts are a lossy summary. Tool call types, file paths, and operation sequences are the primary evidence of waste — and they are all discarded at ingestion.

---

## 3. Decision

**Pivot burnwatch from aggregate statistical anomaly detection to event-level behavioral waste detection.** Preserve tool call names, file paths, and operation sequences in the data model. Build analysis modules that detect waste patterns from behavior, not from volume.

### Target detection capabilities (post-transition)

| Waste type | Detection method | Data needed |
|-----------|-----------------|-------------|
| Agent looping | Detect repeated identical tool calls within a session. Flag when more than N same-call sequences appear. | Tool call names + arguments per message event |
| File re-read without cache | Track file reads per session. Flag files read >3 times where cache reads between accesses are zero. | File paths per operation + cache activity timeline |
| Subagent re-work | Compare parent session's file operations against subagent's. Compute Jaccard overlap. Flag >50% overlap. | File paths for parent and subagent messages |
| Session restart / context loss | Compare first N file operations of consecutive sessions on same project. Flag near-identical initial reads. | File paths per session start |
| Cost outlier (existing) | Statistical outlier on cost. Retained as pre-filter. | TokenEvent unchanged |
| Cache underutilization (existing) | Statistical outlier on cache hit rate. Retained. | TokenEvent unchanged |

### New heuristics overview

| # | Name | Condition | Severity | Data needed |
|---|------|-----------|----------|-------------|
| H10 | Tool call loop | >N identical consecutive tool calls | HIGH | Tool call sequences |
| H11 | File re-read | Same file read ≥3 times without cache hits between reads | MEDIUM | File paths + cache timeline |
| H12 | Subagent overlap | Parent-subagent file operation Jaccard > 50% | HIGH | File paths per session tree |
| H13 | Session restart | Consecutive sessions share ≥80% initial file reads | MEDIUM | File paths per session start |

Existing H1–H9 remain as pre-filters. They catch volume anomalies before behavioral analysis runs.

---

## 4. New Data Model

### TokenEvent v3

```go
type TokenEvent struct {
    // Existing fields (all retained)
    SessionID       string
    ParentSessionID string
    AgentType       string
    Model           string
    Provider        string
    Timestamp       time.Time
    InputTokens     int64
    OutputTokens    int64
    CacheRead       int64
    CacheWrite      int64
    ReasoningTokens int64
    CostUSD         float64
    CostApproximate bool
    CostUnknown     bool
    Project         string
    Harness         string
    IsSubagent      bool

    // New fields (v3)
    ToolCalls   []ToolCall    // tools invoked in this message event
    FileOps     []FileOp      // files read/written/edited in this event
    MessageRole string        // "assistant" | "user" | "system"
    StopReason  string        // why generation stopped
    EventIndex  int           // position within session (1-based, sequential)
}
```

```go
type ToolCall struct {
    Name      string   // e.g. "read_file", "execute_command", "search"
    Arguments string   // JSON string, truncated at 1KB
}

type FileOp struct {
    Path      string   // absolute or relative file path
    Operation string   // "read", "write", "edit", "delete"
}
```

### Design notes

- **`Arguments` is a truncated JSON string, not `map[string]any`.** Avoids deserialization complexity and type-safety issues. Loop detection compares argument strings — identical tool + identical args = potential loop.
- **`FileOps` is a slice, not a map.** A single message can read 5 files. Duplicate reads within one message are still waste.
- **`EventIndex`** enables ordering within a session. Needed for loop detection (consecutive calls) and session restart analysis (first N ops).
- **`MessageRole`** is preserved so we can analyze user→assistant interaction patterns, not just assistant behavior.
- **No message content (text).** The text of what the agent said is too large to store and rarely diagnostic for waste. If needed later, add a `ContentHash uint64` for dedup detection.

### Backward compatibility

New fields have zero values when not available (older sources, future harnesses that don't expose this data). Existing heuristics (H1–H9) don't read these fields — they continue to work unchanged. Sources that can't provide tool/file data simply leave the slices empty → new heuristics produce no signals.

---

## 5. What Gets Reused vs Rewritten

### Reused (no changes or minor additions)

| Module | Fate | Notes |
|--------|------|-------|
| `source/interface.go` | Unchanged | Source interface signature stays. TokenEvent grows but the channel types are the same. |
| `source/pricing.go` | Unchanged | Pricing is orthogonal. |
| `source/pricing_fetcher.go` | Unchanged | Same. |
| `config/config.go` | Extended | Add new signal toggles + thresholds. Follows existing pattern exactly. |
| `config/config_test.go` | Extended | New defaults and validation tests. |
| `cmd/root.go` | Extended | New `--no-*` flags. Pipeline dispatch adds behavioral analysis step. |
| `output/text.go` | Extended | New signal display cases. Existing display unchanged. |
| `output/json.go` | Extended | New JSON fields. Existing serialization unchanged. |
| `analyze/baseline.go` | Unchanged | Statistical baselines still needed for pre-filter heuristics. |
| `analyze/subagent.go` | Unchanged | Tree building unchanged. Needed as input to overlap analysis (H12). |
| `analyze/recommend.go` | Extended | New recommendation types for behavioral signals. |
| `analyze/signal_filter.go` | Unchanged | Min-cost filter + dedup apply to all signals regardless of source. |
| `analyze/trend.go` | Unchanged | Trends are orthogonal. |
| All existing `*_test.go` | Kept, may need fixture updates | Heuristics unchanged, but test input events may need new fields. |
| Test data / golden files | Kept | Existing behavior verified alongside new. |
| `scripts/`, `.golangci.yml`, etc. | Unchanged | Build infrastructure unaffected. |

### Rewritten (significant changes)

| Module | Change | Effort |
|--------|--------|--------|
| `source/event.go` | Add `ToolCalls []ToolCall`, `FileOps []FileOp`, `MessageRole`, `StopReason`, `EventIndex` to TokenEvent. Add `ToolCall` and `FileOp` types. | ~30 lines |
| `source/claude.go` | Parse `tool_use` content blocks from JSONL. Extract tool name, arguments (truncated at 1KB), file paths from known tools (`read`, `write`, `edit`, `grep`). Set `EventIndex` sequentially per session file. | ~80 lines |
| `source/opencode.go` | Parse message_data JSON for tool calls and file references. OpenCode stores structured message data — tool calls have `name` and `input` fields. File paths extracted from tool call arguments. | ~80 lines |

### New files

| File | Purpose | Est. lines |
|------|---------|------------|
| `analyze/loop.go` | Loop detection: analyze tool call sequences within a session. Detect repeated patterns. | ~100 |
| `analyze/loop_test.go` | Tests: no loop, short loop, long loop, loop with interleaved ops, empty session. | ~150 |
| `analyze/reread.go` | File re-read: track per-file read count vs cache activity. Flag excessive re-reads. | ~120 |
| `analyze/reread_test.go` | Tests: single read, re-read with cache, re-read without cache, many files. | ~150 |
| `analyze/overlap.go` | Subagent overlap: compute Jaccard similarity between parent and subagent FileOps. | ~80 |
| `analyze/overlap_test.go` | Tests: no overlap, partial, full, no subagents, empty parent. | ~120 |
| `analyze/restart.go` | Session restart: compare initial file operations across consecutive sessions. | ~100 |
| `analyze/restart_test.go` | Tests: fresh start, continued session, single session, mixed start. | ~130 |

### Removed or deferred

| Item | Fate | Rationale |
|------|------|-----------|
| PR18 (Isolation Forest) | Deferred indefinitely | Multivariate anomaly detection on aggregate token counts adds a different flavor of "statistically unusual" — the same conflation problem. May be reconsidered as a pre-filter if behavioral analysis is too expensive to run on all sessions. |
| PR20 (Supervised ML) | Deferred indefinitely | Requires labeled data. Behavioral signals are interpretable enough that ML adds marginal value over explicit pattern rules. Reconsider after behavioral heuristics ship and accumulate labels. |
| PR19 (LLM Verification) | Kept, deferred to post-behavioral | LLM verification is MORE valuable with behavioral signals — the LLM can confirm "the agent really was looping" vs "it was iterating on different files." Schedule after H10–H13 ship. |

---

## 6. Pros and Cons

### Pros

1. **Answers the actual mission.** Behavioral analysis detects waste patterns (loops, re-reads, re-work) directly, not proxy metrics.

2. **Actionable output.** A signal that says "read_file('src/foo.go') called 5 times without cache" tells the user exactly what to fix. A signal that says "cost 3.2σ above baseline" does not.

3. **Reuses all infrastructure.** Source interface, config, CLI, output, pricing, baselines — everything stays. TokenEvent grows fields; nothing breaks.

4. **Gradual rollout.** New heuristics are feature-gated. Disabled by default → users opt in after calibration. Old heuristics continue to work unchanged during and after transition.

5. **Rich test data.** Claude Code JSONL and OpenCode SQLite both contain the raw data needed. No new data sources required.

6. **Interpretable.** Pattern rules (repetition count, Jaccard overlap) are transparent. Users can understand WHY a session was flagged.

### Cons

1. **Memory impact.** `ToolCalls` and `FileOps` per event increase memory. A session with 200 assistant messages × 5 tool calls × 1KB args = 1MB per session. With 1,000 sessions: ~1GB. Mitigation: truncate arguments at 1KB, deduplicate identical tool calls within analysis pass, process sessions sequentially.

2. **Source fragility.** Parsing tool calls from Claude Code JSONL and OpenCode message_data depends on schema details that could change. Mitigation: graceful degradation — if tool call parsing fails, set `ToolCalls = nil` and fall back to aggregate heuristics only. Error channel handles parse failures.

3. **Harness-specific parsing.** Each harness encodes tool calls differently. Claude Code uses Anthropic content blocks (`{type: "tool_use", name: "...", input: {...}}`). OpenCode uses a different JSON structure. Mitigation: this is already true for the existing ingestion (Claude JSONL vs OpenCode SQLite) — the pattern extends naturally.

4. **Scope increase.** Adds ~600 lines of analysis code + ~600 lines of tests + ~160 lines of source changes. Testing surface grows: each behavioral heuristic needs its own scenario tests.

5. **Noise risk.** File re-read detection may flag legitimate iteration (reading a file, editing, reading again to verify). Mitigation: threshold at ≥3 re-reads of same file without cache hits. Legitimate verify-re-read should hit cache. Pure re-reads without cache are almost never legitimate.

---

## 7. Milestones

### Phase 1: Data Model Expansion (1 PR)

**Scope**: Add new fields to `TokenEvent`. Update both sources to populate them. No heuristic changes.

- [ ] Add `ToolCall`, `FileOp` types to `source/event.go`
- [ ] Add new fields to `TokenEvent` struct
- [ ] Update `source/claude.go` to parse tool_use blocks from JSONL
- [ ] Update `source/opencode.go` to extract tool calls and file paths from message_data
- [ ] Verify: `go build` clean, existing tests pass unchanged
- [ ] Verify: new fields populated correctly in integration test (real JSONL/SQLite data)
- [ ] Commit: `feat: add ToolCall and FileOp fields to TokenEvent`

### Phase 2: Behavioral Heuristics (2 PRs)

**PR-A: Loop + Re-read detection**
- [ ] `analyze/loop.go` + `analyze/loop_test.go`
- [ ] `analyze/reread.go` + `analyze/reread_test.go`
- [ ] Config: `signals.tool_loop`, `thresholds.tool_loop_max_repeats`, `signals.file_reread`, `thresholds.file_reread_min`
- [ ] Integration: add H10, H11 to `DetectWaste`
- [ ] Output: text + JSON display
- [ ] Commit: `feat: tool call loop and file re-read detection`

**PR-B: Subagent overlap + Session restart**
- [ ] `analyze/overlap.go` + `analyze/overlap_test.go`
- [ ] `analyze/restart.go` + `analyze/restart_test.go`
- [ ] Config: `signals.subagent_overlap`, `thresholds.subagent_overlap_pct`, `signals.session_restart`, `thresholds.session_restart_pct`
- [ ] Integration: add H12, H13 to `DetectWaste`
- [ ] Output: text + JSON display
- [ ] Commit: `feat: subagent overlap and session restart detection`

### Phase 3: Refinement (ongoing)

- [ ] Performance benchmarks for event-level analysis on 1K/10K sessions
- [ ] Memory optimization (dedup tool calls, process sessions sequentially)
- [ ] Calibration support for behavioral thresholds (extend `--calibrate`)
- [ ] User feedback loop: which signals were useful, which were noise
- [ ] Reassess isolation forest as pre-filter (run behavioral analysis only on top-N anomalous sessions)

---

## 8. Risks

### Risk 1: Schema changes in harnesses

Claude Code or OpenCode could change their tool call encoding format, breaking parsing. **Mitigation**: `ToolCalls` and `FileOps` are optional slices. If parsing fails, they remain empty → behavioral heuristics produce no signals → pipeline falls back to aggregate heuristics only. Error channel reports parse failures.

### Risk 2: Performance regression

Parsing and storing tool/file data per event increases CPU and memory. **Mitigation**: Truncate arguments at 1KB. Process sessions sequentially (don't hold all tool calls in memory at once). Benchmark before shipping.

### Risk 3: Behavioral heuristics are too noisy

Loop detection with threshold 3 may flag legitimate iterative workflows. **Mitigation**: Start with conservative defaults (threshold 5 for loops, 4 for re-reads). Users tune via config. Calibration mode shows distribution of repetition counts so users can set informed thresholds.

### Risk 4: File path normalization

Claude Code may use absolute paths, OpenCode may use relative paths. Jaccard overlap comparison across harnesses requires path normalization. **Mitigation**: normalize paths to relative-from-project-root in source layer before storing in TokenEvent.

### Risk 5: Abandoning PR18–PR20

The v2 plan invested design work in isolation forest, LLM verification, and supervised ML. Deferring these abandons that investment. **Mitigation**: LLM verification (PR19) is still valuable — reschedule after behavioral signals ship. Isolation forest and ML are genuinely less useful than behavioral detection — no sunk-cost fallacy.

---

## 9. Migration from Current v2 Plan

| Old PR | Old name | New status |
|--------|----------|------------|
| PR17 | Calibration mode | Merge as-is (PR18 dependency removed, but calibration is independently valuable) |
| PR18 | Isolation forest anomaly detection | **Deferred.** May return as pre-filter if behavioral analysis proves expensive. |
| PR19 | LLM verification | **Deferred to post-behavioral.** More valuable with behavioral signal context. |
| PR20 | Supervised ML pipeline | **Deferred indefinitely.** Behavioral signals are interpretable; explicit rules > black-box model for waste detection. |

New PRs replace Phase B and C of original plan:

| New PR | Name | Dependencies |
|--------|------|-------------|
| N1 | Data model expansion (ToolCall + FileOp in TokenEvent) | PR17 (clean main with calibration merged) |
| N2 | Loop + re-read detection (H10, H11) | N1 |
| N3 | Subagent overlap + session restart (H12, H13) | N1 (parallel with N2) |
| N4 | Performance + calibration + polish | N2, N3 |

---

## 10. Success Criteria

The transition is successful when burnwatch can produce signals like:

```
HIGH ses_abc123 (project): $0.84 — tool call loop detected
  → read_file("src/handler.go") called 12 times in session
  → Pattern: read_file → edit_file → read_file → edit_file → ... (6 cycles)
  → Potential savings: $0.42

MEDIUM ses_def456 (project): $3.20 — file re-read without cache
  → config/settings.json read 5 times, 0 cache hits between reads
  → read_file("src/types.ts") read 4 times, 0 cache hits between reads
  → Enable prompt caching to avoid re-reading unchanged files.
  → Potential savings: $1.60

HIGH ses_ghi789 (project): $5.60 — subagent re-did parent's work
  → Parent session read 14 files. Subagent "explore-1" read 11 of the same 14.
  → Overlap: 79% (11/14 shared files). Subagent cost: $2.80 of $5.60 total.
  → Subagent re-read context the parent already ingested.
  → Potential savings: $2.80

MEDIUM ses_jkl012 (project): $1.10 — session restart (lost context from prior session)
  → Initial 5 file reads identical to yesterday's ses_mno345 initial reads.
  → Project had 3 sessions today. Consider continuing instead of restarting.
  → Potential savings: $0.55
```

These signals name specific files, specific tool calls, and specific patterns. A user can act on them without further investigation. That is the difference between a cost anomaly report and a waste detection report.
