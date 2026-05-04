# N2: Tool Call Loop + File Re-read Detection (H10, H11)

> **Workflow:** Follow `docs/PR-template.md`. Review `AGENTS.md` behavioral guidelines before implementing.

## Objective

Add two behavioral waste detection heuristics that analyze per-event tool call sequences and file read patterns. H10 detects repeated identical tool calls (agent looping). H11 detects files read multiple times without cache hits between accesses.

## Success Criteria

- [ ] **H10 — Tool call loop:** Sessions with ≥N consecutive identical tool calls (same `Name` + `Arguments`) are flagged as HIGH
- [ ] **H11 — File re-read:** Sessions where files are read ≥3 times with 0 cache read hits between accesses are flagged as MEDIUM
- [ ] Both heuristics default to disabled (feature-gated), opt-in via config `[signals]` or CLI `--tool-loop`, `--file-reread`
- [ ] H10 threshold: `tool_loop_max_repeats` (default 5, configurable via `[thresholds]`)
- [ ] H11 threshold: `file_reread_min_count` (default 3, configurable via `[thresholds]`)
- [ ] Both produce actionable output naming specific files and tool calls
- [ ] Scenario tests: one waste session + normal sessions per heuristic
- [ ] All existing tests pass (H1–H9 unchanged)

## Dependencies

- **Must merge first:** N1 (data model expansion — needs `ToolCalls`, `FileOps`, `EventIndex` fields on TokenEvent)
- **External dependencies:** None
- **Can be parallel with:** N3 (disjoint files, same foundation)
- **Breaking changes / Migrations needed:** New `WasteSignal` reason values: `"tool_call_loop"`, `"file_reread"`. Output formatters extended.

---

## Pre-flight

- [ ] Pull latest main: `git fetch origin && git checkout main && git pull origin main`
- [ ] Create feature branch: `git checkout -b n2-loop-reread`
- [ ] Verify build environment works on clean main

## Files

| File | Purpose | Notes |
|------|---------|-------|
| `analyze/loop.go` | `detectToolCallLoops()` — scan consecutive identical tool calls | New file, ~100 lines |
| `analyze/loop_test.go` | Tests: no loop, short loop, long loop, interleaved ops, empty session | New file, ~150 lines |
| `analyze/reread.go` | `detectFileReReads()` — track per-file read count vs cache activity | New file, ~120 lines |
| `analyze/reread_test.go` | Tests: single read, re-read with cache, re-read without cache, many files | New file, ~150 lines |
| `analyze/waste.go` | Add H10/H11 calls in `DetectWaste`, gated by config toggles | Modify, ~15 lines |
| `output/text.go` | Display format for `tool_call_loop` and `file_reread` signals | Modify, ~30 lines |
| `output/json.go` | JSON fields for new signal types | Modify, ~15 lines |
| `testdata/scenarios/tool_loop.jsonl` | Scenario: one looping session + 2 normal sessions | Already created |
| `testdata/scenarios/file_reread.jsonl` | Scenario: one re-read session + 2 normal sessions | Already created |
| `testdata/labels/labels.jsonl` | Add labels for new scenario sessions | Modify |
| `output/scenario_test.go` | Add `TestScenario_ToolLoop`, `TestScenario_FileReRead` | Modify, ~60 lines |

---

## Implementation

### H10 — Tool Call Loop (`analyze/loop.go`)

```go
func detectToolCallLoops(events []source.TokenEvent, maxRepeats int) []WasteSignal {
    // Group events by SessionID
    // For each session, sort by EventIndex
    // Scan consecutive tool calls: same (Name + Arguments) → increment counter
    // When count >= maxRepeats, flag the session
}
```

**Algorithm:**
1. Group events by `SessionID`
2. For each session, filter to events with `len(ToolCalls) > 0`
3. Sort by `EventIndex` (ascending)
4. Iterate events, tracking: `prevToolCall` (Name+Arguments), `repeatCount`
5. When `repeatCount >= maxRepeats`, create a WasteSignal with the looped tool name and file path in Detail
6. Only flag once per session (earliest detection)

**Signal format:**
```
Severity: "high"
Reason:   "tool_call_loop"
Detail:   "read_file(\"src/handler.go\") called 12 times consecutively in session"
Metric:   repeatCount (float64)
Threshold: maxRepeats (float64)
```

**Edge cases:**
- Empty session → no signal
- Single tool call → no signal
- Different tools interleaved → separate counters for each tool
- Loop at end of session → still flagged
- `Arguments` truncated at 1KB → compare truncated forms (same tool + same truncated args = potential loop)

**Savings estimate:** `sessionCost * 0.5` (looping typically wastes ~50% of session cost). Existing `checkSubagentOverhead` uses proportional savings pattern — follow same convention.

### H11 — File Re-read (`analyze/reread.go`)

```go
func detectFileReReads(events []source.TokenEvent, minReReads int) []WasteSignal {
    // Group events by SessionID
    // For each session, track per-file timeline:
    //   - When file is read: increment readCount
    //   - Track cache activity between reads
    // Flag files where readCount >= minReReads AND cache reads between accesses == 0
}
```

**Algorithm:**
1. Group events by `SessionID`
2. For each session, sort by `EventIndex`
3. Track per-file state:
   - `readCount int` — total reads of this file
   - `cacheHitsBetween int` — sum of CacheRead across events between first and last read
   - `firstReadIdx int` — EventIndex of first read
   - `lastReadIdx int` — EventIndex of last read
4. After processing all events, flag files where `readCount >= minReReads` AND cache read tokens between first and last read are zero
5. One WasteSignal per file that fails the check

**Signal format:**
```
Severity: "medium"
Reason:   "file_reread"
Detail:   "config/settings.json read 5 times, 0 cache hits between reads"
Metric:   readCount (float64)
Threshold: minReReads (float64)
```

**Edge cases:**
- File read once → no signal (even if minReReads=1, that's normal)
- File read twice with cache between → no signal (caching worked)
- File read 3 times, cache hits only on last read → still flagged (first 2 reads had no cache)
- File read + written + read → flagged if cache=0 (write invalidates cache anyway — the re-read is legitimate but still detectable)
- Multiple events with 0 FileOps → skip them

**Savings estimate:** `sessionCost * (readCount - 1) / readCount * 0.5` — proportional to excess reads.

### Integration into `DetectWaste`

```go
func DetectWaste(events []source.TokenEvent, baselines map[string]Baseline,
    trees []SubagentTree, cfg config.Config) []WasteSignal {

    // ... existing aggregation and H1–H9 ...

    // NEW behavioral heuristics (operate on raw events, not sessionAgg)
    if cfg.Signals.ToolLoop {
        signals = append(signals, detectToolCallLoops(events, cfg.Thresholds.ToolLoopMaxRepeats)...)
    }
    if cfg.Signals.FileReread {
        signals = append(signals, detectFileReReads(events, cfg.Thresholds.FileRereadMinCount)...)
    }

    sortSignals(signals)
    return signals
}
```

The behavioral heuristics receive the full `[]source.TokenEvent` slice (not just `sessionAgg`). They operate independently — no coupling to baselines or tree structures. This is intentional: behavioral detection is evidence-based, not statistical.

### Output format

```
  HIGH ses_loop_waste (project): $2.34 — tool call loop detected
    Model: claude-sonnet-4-5-20250929, 800 in / 400 out
    → read_file("src/handler.go") called 8 times in session
    → Pattern: Read → Edit → Read → Edit → ... (4 cycles)
    → Potential savings: $1.17

  MEDIUM ses_reread_waste (project): $5.20 — file re-read without cache
    Model: claude-sonnet-4-5-20250929, 700 in / 350 out
    → config/settings.json read 4 times, 0 cache hits between reads
    → src/types.go read 1 time (below threshold)
    → Enable prompt caching to avoid re-reading unchanged files.
    → Potential savings: $1.95
```

---

## Test Requirements

### `analyze/loop_test.go`

| Test | Input | Expected |
|------|-------|----------|
| No loop | 3 different tool calls | no signal |
| Short loop | 3 same tool calls, maxRepeats=5 | no signal (below threshold) |
| Long loop | 6 same tool calls (Read, same args), maxRepeats=5 | signal flagged |
| Interleaved loop | Read→Edit→Read→Edit→Read→Edit (Read/Edit/Read/Edit pattern) | loop detected on Read (3 repeats pattern) |
| Loop at end | 10 events, last 8 are same tool | signal flagged |
| Loop mid-session | 15 events, positions 5-10 loop | signal with correct Detail |
| Different args | Same tool, different file_path | not the same tool call — no loop |
| Empty session | 0 events | no signal |
| Multiple sessions | 3 sessions, one loops | only waste session flagged |

### `analyze/reread_test.go`

| Test | Input | Expected |
|------|-------|----------|
| Single read | file read once | no signal |
| Re-read with cache | file read 3 times, cache hits between | no signal (caching worked) |
| Re-read no cache | file read 4 times, cache=0 between | signal flagged |
| Mixed files | file A read 4 times (no cache), file B read 2 times | one signal for file A |
| Write between reads | read→write→read, cache=0 | signal flagged (re-read to verify change) |
| Multiple sessions | 3 sessions, one has re-reads | only waste session flagged |
| Edge: zero cache | `CacheRead=0` throughout session | all re-reads flagged |
| Edge: minReReads=1 | file read once, threshold=1 | signal (but default is 3) |

### Scenario tests in `output/scenario_test.go`

```go
func TestScenario_ToolLoop(t *testing.T) {
    events := loadScenarioJSONL(t, "tool_loop.jsonl")
    cfg := // ... H10 enabled, threshold=5
    signals := runPipelineWithSignals(t, events, cfg)

    sig := findSignalByID(signals, "ses_loop_waste")
    if sig == nil {
        t.Fatal("expected ses_loop_waste to be flagged as tool_call_loop")
    }
    if sig.Severity != "high" {
        t.Errorf("expected severity high, got %s", sig.Severity)
    }
    if sig.Reason != "tool_call_loop" {
        t.Errorf("expected reason tool_call_loop, got %s", sig.Reason)
    }

    // Normal sessions should NOT be flagged
    for _, id := range []string{"ses_loop_normal_01", "ses_loop_normal_02"} {
        if s := findSignalByID(signals, id); s != nil {
            t.Errorf("normal session %s was flagged unexpectedly (reason=%s)", id, s.Reason)
        }
    }
}

func TestScenario_FileReRead(t *testing.T) {
    events := loadScenarioJSONL(t, "file_reread.jsonl")
    cfg := // ... H11 enabled, minReReads=3
    signals := runPipelineWithSignals(t, events, cfg)

    sig := findSignalByID(signals, "ses_reread_waste")
    if sig == nil {
        t.Fatal("expected ses_reread_waste to be flagged as file_reread")
    }
    if sig.Severity != "medium" {
        t.Errorf("expected severity medium, got %s", sig.Severity)
    }
    if sig.Reason != "file_reread" {
        t.Errorf("expected reason file_reread, got %s", sig.Reason)
    }
    if !strings.Contains(sig.Detail, "config/settings.json") {
        t.Errorf("expected Detail to mention config/settings.json, got %s", sig.Detail)
    }
}
```

### Labels update

Add to `testdata/labels/labels.jsonl`:
```jsonl
{"session_id":"ses_loop_waste","verdict":"waste","reason":"repeated tool calls (Read/Edit loop on handler.go)","source":"scenario","created_at":"2026-05-03T00:00:00Z"}
{"session_id":"ses_loop_normal_01","verdict":"not_waste","reason":"normal tool call sequence","source":"scenario","created_at":"2026-05-03T00:00:00Z"}
{"session_id":"ses_loop_normal_02","verdict":"not_waste","reason":"normal tool call sequence","source":"scenario","created_at":"2026-05-03T00:00:00Z"}
{"session_id":"ses_reread_waste","verdict":"waste","reason":"settings.json re-read 4 times without cache","source":"scenario","created_at":"2026-05-03T00:00:00Z"}
{"session_id":"ses_reread_normal_01","verdict":"not_waste","reason":"normal file read with cache","source":"scenario","created_at":"2026-05-03T00:00:00Z"}
{"session_id":"ses_reread_normal_02","verdict":"not_waste","reason":"normal file read with cache","source":"scenario","created_at":"2026-05-03T00:00:00Z"}
```

---

## Approach

1. Write `analyze/loop_test.go` (RED)
2. Implement `analyze/loop.go` `detectToolCallLoops()` (GREEN)
3. Write `analyze/reread_test.go` (RED)
4. Implement `analyze/reread.go` `detectFileReReads()` (GREEN)
5. Wire H10/H11 into `DetectWaste` with config gates
6. Update output formatters (text + JSON)
7. Add scenario tests in `output/scenario_test.go`
8. Update labels
9. Run full test suite + lint
10. Verify: `--tool-loop` and `--file-reread` flags work via CLI

---

## Exit Criteria

- [ ] Pull latest main
- [ ] Create feature branch from main: `git checkout -b n2-loop-reread`
- [ ] Lint passes (zero warnings) — `golangci-lint run`
- [ ] Build compiles cleanly — `go build -o burnwatch .`
- [ ] Tests pass with >=90% coverage on new code — `go test ./... -cover`
- [ ] Self-review: run `./scripts/review-check.sh`, then verify Phases 1-3 in `docs/code-review.md`
- [ ] Document learnings in `docs/learnings.md`
- [ ] Commit: `feat: tool call loop and file re-read detection (H10, H11)`
- [ ] Push to branch `n2-loop-reread`
- [ ] Open pull request
- [ ] Dispatch CodeReviewer subagent against the PR diff
- [ ] Update `docs/plans/progress.md` to reflect merge
- [ ] Merge to main
- [ ] Delete feature branch after merge

---

## Notes

- **Behavioral heuristics operate on raw events, not sessionAgg.** H10/H11 need event sequences (tool call order, file read timeline). The `sessionAgg` structure is built from token counts only — it doesn't carry tool call data. H10/H11 receive `events []source.TokenEvent` directly and iterate in parallel with existing aggregation. No restructuring of `DetectWaste` needed.
- **Sequential processing is memory-safe.** Both heuristics iterate events grouped by session. A session with 200 events × 5 tool calls × 1KB = 1MB. With 1K sessions: ~1GB. Mitigation: process sessions one at a time, don't hold all ToolCalls in memory at once. The event slice is already fully in memory from the source read — tracking per-file or per-tool state within a session adds minimal overhead.
- **Loop detection compares tool Name + Arguments.** `Arguments` is the JSON-serialized input, truncated at 1KB. If two `Edit` calls have different `old_string`/`new_string` values, they're not a loop — they're legitimate iterations. If they have identical arguments AND are consecutive, it's a loop.
- **File re-read without cache is almost always waste.** The only legitimate case is verify-re-read after a write, which should hit cache on modern LLM APIs. If cache is zero throughout, the model/provider doesn't support caching or it's misconfigured — both are useful signals.
- **Thresholds are conservative by default.** `maxRepeats=5` means 5+ consecutive identical calls to flag. `minReReads=3` means 3+ reads of the same file. These are the values from the ADR.
