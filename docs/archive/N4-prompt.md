# N4: Performance, Calibration, and Polish

> **Workflow:** Follow `docs/PR-template.md`. Review `AGENTS.md` behavioral guidelines before implementing.

## Objective

Polish the v3 behavioral waste detection pipeline: optimize memory for event-level analysis on large datasets, extend calibration mode to cover behavioral heuristics, and add performance benchmarks.

## Success Criteria

- [ ] DetectWaste processes events sequentially (one session at a time) to limit memory, not loading all ToolCalls/FileOps into memory simultaneously
- [ ] `--calibrate` mode shows distributions for tool call loop repeats and file re-read counts per session
- [ ] `--calibrate` suggests behavioral thresholds: `tool_loop_max_repeats`, `file_reread_min_count`
- [ ] Benchmark for 1K and 10K sessions in `analyze/bench_test.go` — verify no >20% regression from pre-behavioral baseline
- [ ] Path normalization is consistent: Claude and OpenCode source produce comparable file paths for overlap/restart analysis
- [ ] File path normalization handles edge cases: leading `./`, trailing slashes, `..` segments
- [ ] All tests pass, coverage maintained
- [ ] Report in `docs/decisions/2026-05-03-event-level-waste-detection.md` status updated to "Accepted"

## Dependencies

- **Must merge first:** N2, N3 (needs all behavioral heuristics implemented)
- **External dependencies:** None
- **Can be parallel with:** None (sequential cleanup)
- **Breaking changes / Migrations needed:** None (internal optimizations + calibration extension)

---

## Pre-flight

- [ ] Pull latest main: `git fetch origin && git checkout main && git pull origin main`
- [ ] Create feature branch: `git checkout -b n4-polish`
- [ ] Verify build environment works on clean main

## Files

| File | Purpose | Notes |
|------|---------|-------|
| `analyze/waste.go` | Restructure DetectWaste for sequential session processing | Modify |
| `analyze/calibrate.go` | Extend CalibrationReport with behavioral metrics | Modify |
| `analyze/calibrate_test.go` | Test behavioral calibration stats | Modify |
| `output/calibrate_text.go` | Display behavioral distributions + suggestions | Modify |
| `output/calibrate_json.go` | JSON fields for behavioral calibration data | Modify |
| `output/calibrate_test.go` | Update golden file for calibration output | Modify |
| `analyze/bench_test.go` | Add BenchBehavioralDetectWaste benchmark | New, ~50 lines |
| `source/path.go` | `NormalizePath()` shared path normalization helper | New, ~40 lines |
| `source/path_test.go` | Tests for path normalization edge cases | New, ~80 lines |
| `source/cross_test.go` | Cross-harness equivalence: same logical session → same TokenEvent | New, ~120 lines |
| `docs/decisions/2026-05-03-event-level-waste-detection.md` | Update status to Accepted | Modify |

---

## Implementation

### Sequential session processing

Current `DetectWaste`:

```go
func DetectWaste(events []source.TokenEvent, ...) []WasteSignal {
    agg := make(map[string]*sessionAgg)
    for _, e := range events {
        // Aggregate all events into session-level metrics
    }
    // Run all heuristics
}
```

Problem: All events are in memory. For behavioral heuristics (H10-H13), we iterate separately. For 1K sessions × 200 events × 5 tool calls × 1KB = ~1GB.

**Refactor to session-at-a-time processing:**

```go
func DetectWaste(events []source.TokenEvent, baselines map[string]Baseline,
    trees []SubagentTree, cfg config.Config) []WasteSignal {

    // Step 1: Pre-group events by SessionID (one pass)
    sessionEvents := groupBySession(events)

    // Step 2: Compute session-level aggregates (H1-H9 still need these)
    var aggs []*sessionAgg
    for sid, events := range sessionEvents {
        agg := computeSessionAgg(sid, events)
        aggs = append(aggs, agg)
    }

    // Step 3: Run aggregate heuristics (H1-H9)
    var signals []WasteSignal
    for _, a := range aggs {
        // ... existing H1-H9 checks on sessionAgg ...
    }

    // Step 4: Run behavioral heuristics (H10-H13)
    if cfg.Signals.ToolLoop {
        for sid, events := range sessionEvents {
            signals = append(signals, detectToolCallLoopsForSession(sid, events, cfg.Thresholds.ToolLoopMaxRepeats)...)
        }
    }
    // ... similar for H11, H12, H13 ...

    sortSignals(signals)
    return signals
}
```

Wait — this is premature optimization. Current memory is already `len(events) × sizeof(TokenEvent)`. Adding ToolCalls/FileOps per event increases per-event size from ~200 bytes to ~500 bytes. 1K sessions × 200 events × 500 bytes = 100MB, not 1GB. The 1GB estimate in the ADR was worst-case (every event has 5 tool calls × 1KB args). In practice, most events have 0-3 tool calls.

**Decision:** Don't restructure `DetectWaste` in N4. The existing batching approach is fine. Only optimize if benchmarks show a problem.

**Instead, N4 focuses on:**
1. Path normalization helper
2. Calibration extension for behavioral heuristics
3. Benchmarks
4. ADR status update

### Path normalization (`source/path.go`)

```go
// NormalizePath converts a file path to a canonical relative form
// for consistent comparison across harnesses.
//
// Rules:
// 1. Strip leading "./" prefix
// 2. Strip leading "/" (absolute → relative)
// 3. Collapse ".." segments where unambiguous
// 4. Convert "\" to "/" (Windows → Unix)
// 5. Strip trailing "/"
func NormalizePath(path string) string {
    path = strings.ReplaceAll(path, "\\", "/")
    path = strings.TrimPrefix(path, "./")
    path = strings.TrimPrefix(path, "/")
    if path == "" {
        return "."
    }
    // Clean handles ".." segments
    path = filepath.Clean(path)
    path = strings.TrimSuffix(path, "/")
    return path
}
```

Use this in both Claude and OpenCode sources after extracting file paths. This ensures H12/H13 comparisons work across harnesses.

### Cross-harness equivalence test (`source/cross_test.go`)

The behavioral heuristics are harness-agnostic (they operate on `TokenEvent`), but source parsers are harness-specific. We need to verify both parsers produce equivalent `TokenEvent` values for equivalent session data.

```go
// TestCrossHarness_EquivalentPathNormalization verifies that
// the same logical file read produces the same FileOp.Path
// regardless of harness origin.
func TestCrossHarness_PathNormalization(t *testing.T) {
    tests := []struct {
        name     string
        claudePath  string // e.g. "/Users/hoang/burnwatch/src/main.go"
        opencodePath string // e.g. "src/main.go"
        want        string // e.g. "src/main.go"
    }{
        {"simple file", "/Users/hoang/burnwatch/src/main.go", "src/main.go", "src/main.go"},
        {"dot prefix", "/Users/hoang/burnwatch/src/main.go", "./src/main.go", "src/main.go"},
        {"nested dir", "/Users/hoang/burnwatch/pkg/util/helper.go", "pkg/util/helper.go", "pkg/util/helper.go"},
        {"windows backslash", "/Users/hoang/burnwatch/src\\main.go", "src\\main.go", "src/main.go"},
        {"config file", "/Users/hoang/burnwatch/config/settings.json", "config/settings.json", "config/settings.json"},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            claudeNormalized := NormalizePath(tt.claudePath, "/Users/hoang/burnwatch")
            opencodeNormalized := NormalizePath(tt.opencodePath, "")
            if claudeNormalized != tt.want || opencodeNormalized != tt.want {
                t.Errorf("claude: %q → %q, opencode: %q → %q, both want %q",
                    tt.claudePath, claudeNormalized, tt.opencodePath, opencodeNormalized, tt.want)
            }
        })
    }
}

// TestCrossHarness_ToolNameCanonicalization verifies
// that tool names are lowercase after canonicalization regardless of harness.
func TestCrossHarness_ToolNameCanonicalization(t *testing.T) {
    tests := []struct{
        harness string
        rawName string
        want    string
    }{
        {"claude", "Read", "read"},
        {"claude", "Write", "write"},
        {"claude", "Edit", "edit"},
        {"claude", "Glob", "glob"},
        {"claude", "Bash", "bash"},
        {"claude", "Skill", "skill"},
        {"opencode", "read", "read"},
        {"opencode", "write", "write"},
        {"opencode", "edit", "edit"},
        {"opencode", "glob", "glob"},
    }
    for _, tt := range tests {
        t.Run(tt.harness+"/"+tt.rawName, func(t *testing.T) {
            got := canonicalizeToolName(tt.rawName)
            if got != tt.want {
                t.Errorf("canonicalizeToolName(%q) = %q, want %q", tt.rawName, got, tt.want)
            }
        })
    }
}
```

**Integration test with real data (run locally, not in CI):**

```go
func TestCrossHarness_RealData_EquivalentEvents(t *testing.T) {
    // Requires both Claude and OpenCode session data on disk.
    // Manual test: compare TokenEvent output from both sources.
    t.Skip("manual integration test — requires local session data")

    // Load events from Claude
    claudeSrc := &ClaudeSource{projectsDir: os.Getenv("BURNWATCH_CLAUDE_PROJECTS")}
    claudeEvents, _ := claudeSrc.Events()
    // Load events from OpenCode
    opencodeSrc := &OpenCodeSource{dbPath: os.Getenv("BURNWATCH_OPENCODE_DB")}
    opencodeEvents, _ := opencodeSrc.Events()
    // Assert: for any given session, TokenEvent fields (Model, Cost, SessionID)
    // match between harnesses. Tool names are lowercase. File paths are relative.
}
```

### Calibration extension (`analyze/calibrate.go`)

Add to `CalibrationReport`:

```go
type CalibrationReport struct {
    // ... existing fields ...

    // Behavioral distributions (new)
    ToolLoopMaxRepeats       DistStats `json:"tool_loop_max_repeats"`
    FileReReadMaxCount       DistStats `json:"file_reread_max_count"`
    SubagentOverlapPcts      DistStats `json:"subagent_overlap_pcts"`
    SessionRestartOverlapPct DistStats `json:"session_restart_overlap_pct"`
}
```

**Compute behavioral stats:**

```go
func computeBehavioralCalibration(events []source.TokenEvent, trees []SubagentTree) (loop, reread, overlap, restart DistStats) {
    // Tool loop: for each session, find max consecutive same-tool repeats
    sessionEvents := groupBySession(events)
    var loopRepeats []float64
    for _, evts := range sessionEvents {
        maxRepeat := computeMaxRepeats(evts)
        loopRepeats = append(loopRepeats, float64(maxRepeat))
    }
    loop = computeDistStats(loopRepeats)

    // File re-read: for each session, find max re-read count for any file
    var rereadCounts []float64
    for _, evts := range sessionEvents {
        maxReRead := computeMaxReReads(evts)
        rereadCounts = append(rereadCounts, float64(maxReRead))
    }
    reread = computeDistStats(rereadCounts)

    // Subagent overlap: per tree, compute overlap for each subagent
    var overlapPcts []float64
    for _, tree := range trees {
        pcts := computeOverlapPcts(events, tree)
        for _, p := range pcts {
            overlapPcts = append(overlapPcts, p)
        }
    }
    overlap = computeDistStats(overlapPcts)

    // Session restart: per project, compute overlap between consecutive sessions
    var restartPcts []float64
    // ...
    restart = computeDistStats(restartPcts)
}
```

### Suggestions for behavioral thresholds

```go
// In generateSuggestions():
s = append(s, ThresholdSuggestion{
    ConfigKey: "tool_loop_max_repeats",
    Value:     math.Ceil(toolLoop.P95),
    Rationale: fmt.Sprintf("P95 of consecutive same-tool repeats is %.0f — flag sessions exceeding this"),
})
s = append(s, ThresholdSuggestion{
    ConfigKey: "file_reread_min_count",
    Value:     math.Ceil(fileReRead.P95),
    Rationale: fmt.Sprintf("P95 of file re-read count per session is %.0f — flag sessions exceeding this"),
})
```

### Benchmarks (`analyze/bench_test.go`)

```go
func BenchmarkBehavioralDetectWaste(b *testing.B) {
    // Generate 500 sessions with realistic tool call and file op patterns
    events := generateBenchEvents(b, 500, 50) // 500 sessions, ~50 events each
    baselines := ComputeBaselines(events, config.Defaults())
    trees := BuildSubagentTree(events)
    cfg := // all heuristics enabled

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        DetectWaste(events, baselines, trees, cfg)
    }
}

func generateBenchEvents(tb testing.TB, sessions, eventsPerSession int) []source.TokenEvent {
    rng := rand.New(rand.NewSource(42))
    var events []source.TokenEvent
    // ... generate realistic tool call patterns ...
    // 10% of sessions have loops, 15% have re-reads, etc.
}
```

---

## Approach

1. Implement `NormalizePath` in `source/path.go` + tests
2. Integrate into Claude and OpenCode sources (wrap extracted file paths)
3. Extend calibration with behavioral stats
4. Update calibration output (text + JSON) with new distributions and suggestions
5. Add benchmarks
6. Run benchmarks, verify no regression
7. Update ADR status to "Accepted"
8. Full test suite + lint

---

## Exit Criteria

- [ ] Pull latest main
- [ ] Create feature branch from main: `git checkout -b n4-polish`
- [ ] Lint passes (zero warnings) — `golangci-lint run`
- [ ] Build compiles cleanly — `go build -o burnwatch .`
- [ ] Tests pass with >=90% coverage on new code — `go test ./... -cover`
- [ ] Benchmark passes: `go test -bench=BehavioralDetectWaste -benchmem ./analyze`
- [ ] Self-review: run `./scripts/review-check.sh`, then verify Phases 1-3 in `docs/code-review.md`
- [ ] Document learnings in `docs/learnings.md`
- [ ] Commit: `feat: behavioral calibration, path normalization, benchmarks, ADR accepted`
- [ ] Push to branch `n4-polish`
- [ ] Open pull request
- [ ] Dispatch CodeReviewer subagent against the PR diff
- [ ] Update `docs/plans/progress.md` to reflect merge
- [ ] Merge to main
- [ ] Delete feature branch after merge

---

## Notes

- **Don't prematurely optimize memory.** The ADR's 1GB estimate assumes worst-case (all events have 5 tool calls × 1KB args). In practice, most events have 0–3 tool calls with 100–300B args. Per-session sequential processing adds complexity for marginal benefit. Run benchmarks first, optimize only if needed.
- **Path normalization is the most impactful N4 change for correctness.** Without it, H12/H13 overlap computations produce false negatives when comparing Claude (stripped absolute paths) vs OpenCode (relative paths with `./` prefix). N4 ensures they align.
- **Calibration for behavioral thresholds must work with zero-config.** Same as existing `--calibrate` mode — no `.burnwatch.toml` needed. Compute stats from raw events.
- **Behavioral stats in calibration may be sparse.** If no sessions have loops, `ToolLoopMaxRepeats` distribution shows P50=1 (single tool calls, no repeats). This is expected — suggest a conservative default (e.g., 5) as the floor.
- **ADR status update:** Change line 5 from `Status: Proposed` to `Status: Accepted`. Add a note: "Implemented in PRs N1–N4. See docs/plans/N1-prompt.md through N4-prompt.md."
