# N5: Session Drill-Down — `--explain <session-id>`

> **Workflow:** Follow `docs/PR-template.md`. Review `AGENTS.md` behavioral guidelines before implementing.
> **UX spec:** `docs/plans/v3-ux-plan.md` Phase I

## Objective

Add `--explain <session-id>` flag that shows an annotated event timeline for a specific session, with waste patterns highlighted inline. This is the drill-down from "signal detected" to "here's the evidence."

## Success Criteria

- [ ] `burnwatch --explain ses_abc123` shows annotated timeline for that session
- [ ] Header shows: session ID, project, duration, cost, model, harness, event count, tool call count, file count, subagent count
- [ ] Waste signals summary lists all signals for this session with severity, reason, and detail
- [ ] Annotated timeline sorts events by EventIndex, shows each event with tool calls and file ops
- [ ] H10 tool loop: repeated tool calls annotated with `[LOOP REPEAT N/M]` inline
- [ ] H11 file re-read: re-read file operations annotated with `[RE-READ N/M]` inline
- [ ] Subagent events annotated with `[SUBAGENT START]` and indented children
- [ ] Subagent cost breakdown section lists subagents with cost and file ops
- [ ] File re-read breakdown section lists files re-read without cache with event indices
- [ ] Unknown session ID prints error to stderr and exits with code 1
- [ ] Session with zero events prints "No events found for session <id>"
- [ ] Session with no signals prints timeline without annotations
- [ ] All existing tests pass unchanged

## Dependencies

- **Must merge first:** N2 (H10 tool loop, H11 file re-read detection merged)
- **External dependencies:** None
- **Can be parallel with:** N3 (H12/H13 — `--explain` handles their signal types as generic fallback)
- **Breaking changes / Migrations needed:** None (new flag, new output package file, no existing code touched)

---

## Pre-flight

- [ ] Pull latest main: `git fetch origin && git checkout main && git pull origin main`
- [ ] Create feature branch: `git checkout -b n5-explain`
- [ ] Verify build environment works on clean main

## Files

| File | Purpose | Notes |
|------|---------|-------|
| `output/explain.go` | `FormatExplain()` — annotated timeline formatting | New, ~300 lines |
| `output/explain_test.go` | Table-driven tests for formatting | New, ~300 lines |
| `testdata/scenarios/explain_loop.jsonl` | Session with tool call loop (H10) | New, ~20 lines |
| `testdata/scenarios/explain_reread.jsonl` | Session with file re-read (H11) | New, ~20 lines |
| `testdata/scenarios/explain_mixed.jsonl` | Session with loop + re-read | New, ~25 lines |
| `testdata/scenarios/explain_clean.jsonl` | Session with no waste (all clean) | New, ~15 lines |
| `cmd/root.go` | Add `--explain <id>` flag and handler | Modify, ~45 lines |

---

## Implementation

### `output/explain.go` — Core formatting

**Function signature:**

```go
// FormatExplain produces an annotated event timeline for a single session.
// events: all TokenEvents across all harnesses (caller should filter by SessionID)
// signals: all WasteSignals (caller should filter by SessionID)
// trees: all SubagentTrees (caller should filter by SessionID)
func FormatExplain(
    sessionID string,
    events []source.TokenEvent,
    signals []analyze.WasteSignal,
    trees []analyze.SubagentTree,
) string
```

**The function must NOT filter events by session — the caller does that.** This keeps FormatExplain focused on formatting and makes it testable with pre-filtered data.

**Output structure (use `strings.Builder` for efficiency):**

```
1. HEADER — session metadata
   - Session ID, Project, Harness
   - Duration: first event timestamp → last event timestamp
   - Duration: format as "1h 23m" or "34s" (human-readable)
   - Cost: $X.XX (or $?.?? if cost unknown)
   - Model (or "unknown")
   - Counts: events, tool calls, file ops, subagents

2. WASTE SIGNAL SUMMARY (if any)
   - For each signal: severity (upper), reason code, detail string
   - Sorted by severity (high → medium → low), then by reason

3. ANNOTATED TIMELINE
   - Events sorted by EventIndex ascending
   - Format per event:
     "  #{index:<4}  <action>  <path/name>  [ANNOTATION]"
   - Tool calls shown as: "  #{n}  <tool_name>  <arg_summary>"
     - arg_summary: first file_path or first argument field value, truncated at 60 chars
   - File ops shown as: "  #{n}  <operation>  <path>"
   - Subagent spawn shown as: "  #{n}  ▶ subagent:<name>"
   - Subagent events indented 2 extra spaces
   - Annotations:
     - H10 loop: "← [LOOP REPEAT N/M]" where N is repetition number, M is total
     - H11 re-read: "← [RE-READ N/M]" where N is occurrence number, M is total
     - Subagent start: "← [SUBAGENT START — $X.XX]"
     - Subagent overlap (H12, future): "← [OVERLAP] also read by parent at #N"
     - Session restart (H13, future): "← [RESTART] same file as session <id>"

4. SUBAGENT COST BREAKDOWN (if subagents exist)
   - For each subagent: name, cost, overlap stats if available
   - Shared files listed (truncated to first 10, with count of remaining)

5. FILE RE-READ BREAKDOWN (if re-read signals exist)
   - For each re-read file: path, read count, event indices where read occurred
```

**Annotation computation helpers (package-private):**

```go
// annotation represents a marker to display on a timeline event.
type annotation struct {
    EventIndex int
    Text       string   // e.g. "← [LOOP REPEAT 3/12]"
}

// computeLoopAnnotations finds event indices where tool calls repeat the same tool+args pattern.
// Uses the same detection logic as analyze/detectSessionLoops but returns annotations instead of signals.
func computeLoopAnnotations(events []source.TokenEvent, signals []analyze.WasteSignal) []annotation

// computeReReadAnnotations finds event indices where files flagged by H11 are read.
func computeReReadAnnotations(events []source.TokenEvent, signals []analyze.WasteSignal) []annotation

// computeSubagentAnnotations marks subagent spawn points and child events.
func computeSubagentAnnotations(events []source.TokenEvent, trees []analyze.SubagentTree, signals []analyze.WasteSignal) []annotation
```

**Key algorithm — `computeLoopAnnotations`:**

The H10 signal's `Detail` field contains the tool name and repeat count (e.g. `read_file("src/handler.go") called 12 times consecutively in session`). To find where these repeats occur in the timeline:

```go
func computeLoopAnnotations(events []source.TokenEvent, signals []analyze.WasteSignal) []annotation {
    // Sort events by EventIndex
    // For each H10 signal:
    //   1. Extract tool name and file path from Detail string
    //   2. Walk events, build flat tool call list (same as detectSessionLoops)
    //   3. Find consecutive repeats matching the pattern
    //   4. Create annotation for each repeat instance with [LOOP REPEAT N/M]
    // Return annotations keyed by EventIndex
}
```

Parse the Detail string to extract tool name and file path. The Detail format from `analyze/loop.go:84-86` is:
- `"<name>"` or `"<name>(\"<file_path>\")"` followed by ` called <N> times consecutively in session`

```go
// parseLoopDetail extracts tool name and file path from a H10 Detail string.
// Returns "", "" if not parseable.
func parseLoopDetail(detail string) (toolName string, filePath string) {
    // Find the opening paren or space before " called"
    // Extract tool name and optional quoted file path
}
```

**Constraints:**
- Max timeline lines: if session has >500 events, show first 50 + last 20 + a gap line `... (430 events omitted) ...` with waste-adjacent events always included (never omitted). Waste-adjacent = ±5 events from any annotation.
- Use `strconv.Itoa` and `fmt.Fprintf` for formatting. Avoid `text/template` — this is a terminal report, not a web page.
- All file paths displayed as-is from TokenEvent.FileOps (already normalized by source).
- Tool call arguments truncated at 60 characters, ellipsis (`...`) appended if truncated.
- Duration calculation: use `time.Duration` formatting. Round to nearest second for <1min, nearest minute for <1hr.
- Subagents: derive subagent name from TokenEvent.AgentType field. Subagent events have IsSubagent=true and ParentSessionID set.
- Unknown cost: display `$?` when any event has CostUnknown, `≈ $` when CostApproximate but not CostUnknown.

**Error handling:**
- Unknown session ID → `fmt.Fprintf(os.Stderr, "Session %s not found.\n", id)` + `os.Exit(1)` (handled in cmd/root.go, not FormatExplain)
- Zero events for session → FormatExplain returns "No events found for session <id>.\n" (no error)
- Corrupted EventIndex (out of order) → sort by EventIndex anyway, show warning `[event order mismatch]` annotation on first out-of-order event
- Signal Detail string parse failure → skip annotation for that signal, continue (don't crash)

### `cmd/root.go` — Wiring

Add a flag and branch:

```go
// In the flag declarations block:
explainID := flag.String("explain", "", "Show annotated timeline for session ID")

// After flag.Parse(), before config loading:
if *explainID != "" {
    // Collect events from all sources
    // Filter events to the session
    // Run full pipeline to get signals + trees (needed for annotations)
    // Filter signals to the session
    // Format and print
    // Exit
}
```

The explain path is standalone — it runs its own pipeline and exits. It does NOT load config, does NOT fetch pricing (uses whatever pricing is cached), and does NOT need `.burnwatch.toml`.

```go
if *explainID != "" {
    sources := source.Discover()
    if len(sources) == 0 {
        fmt.Fprintln(os.Stderr, "No data sources found.")
        os.Exit(1)
    }
    events := output.CollectEvents(sources)
    
    // Filter to target session
    var sessionEvents []source.TokenEvent
    for _, e := range events {
        if e.SessionID == *explainID {
            sessionEvents = append(sessionEvents, e)
        }
    }
    if len(sessionEvents) == 0 {
        fmt.Fprintf(os.Stderr, "Session %q not found.\n", *explainID)
        os.Exit(1)
    }
    
    // Run detection to get signals and trees
    baselines := analyze.ComputeBaselines(events, config.Defaults())
    cfg := config.Defaults()
    trees := analyze.BuildSubagentTree(events)
    signals := analyze.DetectWaste(events, baselines, trees, cfg)
    
    // Filter signals and trees to the target session
    var sessionSignals []analyze.WasteSignal
    for _, s := range signals {
        if s.SessionID == *explainID {
            sessionSignals = append(sessionSignals, s)
        }
    }
    
    var sessionTrees []analyze.SubagentTree
    for _, t := range trees {
        if t.RootSessionID == *explainID || hasSubagentForSession(t, *explainID) {
            sessionTrees = append(sessionTrees, t)
        }
    }
    
    text := output.FormatExplain(*explainID, sessionEvents, sessionSignals, sessionTrees)
    fmt.Print(text)
    return
}
```

The explain flag should be in the same flag set block as existing flags. Add it between `Calibrate` and `versionFlag` for logical grouping.

---

## Test Requirements

### `output/explain_test.go`

Table-driven tests using the same scenario JSONL pattern as `output/scenario_test.go`.

**Test cases:**

1. **`TestFormatExplain_Loop`** — session with H10 tool loop
   - Assert header contains session ID, project, cost
   - Assert waste summary contains H10 signal with correct severity
   - Assert timeline contains `[LOOP REPEAT 1/6]`, `[LOOP REPEAT 2/6]`, etc.
   - Assert non-loop events have no loop annotation

2. **`TestFormatExplain_ReRead`** — session with H11 file re-read
   - Assert waste summary contains H11 signal
   - Assert timeline marks re-read files with `[RE-READ N/M]`
   - Assert file re-read breakdown section lists the file with event indices

3. **`TestFormatExplain_Mixed`** — session with both loop and re-read
   - Assert both signal types appear in summary
   - Assert both annotation types appear on correct events
   - Assert an event can have multiple annotations

4. **`TestFormatExplain_Clean`** — session with no waste signals
   - Assert waste summary section is empty or says "No waste signals"
   - Assert timeline has no annotations

5. **`TestFormatExplain_Subagent`** — session with subagents
   - Assert subagent spawn events marked with `[SUBAGENT START]`
   - Assert child events indented
   - Assert subagent breakdown section present

6. **`TestFormatExplain_Empty`** — zero events
   - Returns "No events found" message

7. **`TestFormatExplain_Duration`** — verify duration formatting
   - 45 seconds → "45s"
   - 5 minutes → "5m"
   - 1 hour 30 minutes → "1h 30m"
   - 3 hours 2 minutes → "3h 2m"

8. **`TestParseLoopDetail`** — unit test for the helper
   - `read_file("src/main.go") called 12 times...` → ("read_file", "src/main.go")
   - `Bash called 5 times...` → ("Bash", "")
   - `malformed string` → ("", "")

**Coverage target:** >=90% on new code.

### E2E Scenario Tests

Scenarios use the same JSONL format as existing tests (`testdata/scenarios/*.jsonl`). Each is a Claude-format JSONL file with `type: "assistant"` entries containing `tool_use` content blocks and file operations.

**`testdata/scenarios/explain_loop.jsonl`:**
- 1 session (`ses_explain_loop`) with 15 events
- Events #5-#10: repeated `read_file("src/handler.go")` — triggers H10
- Other events: varied tool calls (bash, glob, grep) — not loops
- ToolCalls populated, FileOps populated, EventIndex set

**`testdata/scenarios/explain_reread.jsonl`:**
- 1 session (`ses_explain_reread`) with 20 events
- `config/settings.json` read at events #2, #7, #12, #18 — triggers H11 (4 reads, 0 cache)
- Other events read different files
- CacheRead=0 for all events (to trigger H11)

**`testdata/scenarios/explain_mixed.jsonl`:**
- 1 session (`ses_explain_mixed`) with 25 events
- Loop pattern: `read_file("src/types.go")` at events #3-#8 — triggers H10
- Re-read: `docs/api.md` at events #5, #11, #17 — triggers H11
- Both waste types in one session

**`testdata/scenarios/explain_clean.jsonl`:**
- 1 session (`ses_explain_clean`) with 10 events
- All events: varied tool calls on different files
- No loops, no re-reads, all CacheRead > 0 when appropriate
- ZERO waste signals detected

**Scenario test format (add to `output/scenario_test.go`):**

```go
func TestScenario_ExplainLoop(t *testing.T) {
    events := loadScenarioJSONL(t, "explain_loop.jsonl")
    cfg := allOnConfig()  // enable all signals
    baselines := analyze.ComputeBaselines(events, cfg)
    trees := analyze.BuildSubagentTree(events)
    signals := analyze.DetectWaste(events, baselines, trees, cfg)
    
    // Verify detection
    found := findSignalsByReason(signals, "tool_call_loop")
    if len(found) == 0 {
        t.Fatal("expected tool_call_loop signal, got none")
    }
    
    // Verify formatting
    sessionSignals := filterBySession(signals, "ses_explain_loop")
    sessionEvents := filterBySession(events, "ses_explain_loop")
    output := FormatExplain("ses_explain_loop", sessionEvents, sessionSignals, trees)
    
    // Assert key substrings
    mustContain(t, output, "ses_explain_loop")
    mustContain(t, output, "[LOOP REPEAT")
    mustContain(t, output, "tool_call_loop")
}
```

Same pattern for explain_reread, explain_mixed, explain_clean.

**Helper functions needed in scenario_test.go:**
```go
func mustContain(t *testing.T, haystack, needle string) {
    t.Helper()
    if !strings.Contains(haystack, needle) {
        t.Errorf("expected output to contain %q", needle)
    }
}

func mustNotContain(t *testing.T, haystack, needle string) {
    t.Helper()
    if strings.Contains(haystack, needle) {
        t.Errorf("expected output to NOT contain %q", needle)
    }
}
```

---

## Approach

1. Write scenario JSONL files first (test fixtures)
2. Write `FormatExplain` header + event list (no annotations yet) — verify it compiles
3. Write `computeLoopAnnotations` + `parseLoopDetail` — get loop scenario test passing
4. Write `computeReReadAnnotations` — get re-read scenario test passing
5. Write `computeSubagentAnnotations` — get subagent section working
6. Add duration formatting + edge cases (empty, unknown session)
7. Wire `--explain` flag in `cmd/root.go`
8. Full test suite + lint
9. Verify with real data: `burnwatch --explain <real-session-id>`

---

## Exit Criteria

- [ ] Pull latest main
- [ ] Create feature branch from main: `git checkout -b n5-explain`
- [ ] Lint passes (zero warnings) — `golangci-lint run`
- [ ] Build compiles cleanly — `go build -o burnwatch .`
- [ ] Tests pass with >=90% coverage on new code — `go test ./... -cover`
- [ ] Self-review: run `./scripts/review-check.sh`, then verify Phases 1-3 in `docs/code-review.md`
- [ ] Document learnings in `docs/learnings.md`
- [ ] Commit: `feat: session drill-down with --explain flag (annotated timeline)`
- [ ] Push to branch `n5-explain`
- [ ] Open pull request
- [ ] Dispatch CodeReviewer subagent against the PR diff
- [ ] Update `docs/plans/progress.md` to reflect merge
- [ ] Merge to main
- [ ] Delete feature branch after merge

---

## Notes

- **Metadata comes from TokenEvents, not WasteSignal.** Session duration, event count, tool call count, file count are all computed from the events slice. WasteSignal only has aggregated fields.
- **The session cost in WasteSignal.SessionCost may differ from summing all event costs** due to how the heuristic aggregates. `--explain` should sum event costs directly for the header to show the actual cost, then show WasteSignal.SessionCost for comparison only if they differ significantly (>5%).
- **Don't `os.Exit(1)` inside `output/explain.go`.** FormatExplain returns a string. The caller (cmd/root.go) handles exit codes. This keeps the formatting function testable.
- **Sort consistently with existing code.** `sort.Slice` by EventIndex. The same pattern is used in `analyze/loop.go:32-34` and `analyze/reread.go:36-38`.
- **Use `groupEventsBySession` from `analyze/loop.go`?** It's in the `analyze` package. The `output` package can call it via `analyze.GroupEventsBySession` (would need to export it). Better: re-implement a local `groupBySession` in `output/explain.go` — it's 6 lines and avoids coupling.
- **Subagent trees:** `SubagentTree` has `RootSessionID`, `Children`, and `CostUSD` fields. The parent session is the root. Check `analyze/subagent.go` for the exact struct.
- **H12/H13 (N3) future-proofing:** `computeSubagentAnnotations` should accept signals and trees but gracefully handle the absence of overlap/restart signals. When signals list contains "subagent_overlap" reason, extract the overlap info from the Detail string. When it doesn't, skip.
- **The `--explain` path skips config entirely.** No `.burnwatch.toml` needed. All heuristics enabled (via `config.Defaults()`). This is deliberate — the user wants to see ALL waste in a session regardless of their current config toggles.
