# N3: Subagent Overlap + Session Restart Detection (H12, H13)

> **Workflow:** Follow `docs/PR-template.md`. Review `AGENTS.md` behavioral guidelines before implementing.

## Objective

Add two behavioral waste detection heuristics that analyze file operation patterns across session trees and consecutive sessions. H12 detects subagents re-reading files the parent already ingested (Jaccard overlap). H13 detects new sessions that re-read the same initial files as the previous session (context loss / session restart).

## Success Criteria

- [ ] **H12 — Subagent overlap:** Subagent file operations with Jaccard similarity >50% to parent are flagged as HIGH
- [ ] **H13 — Session restart:** Consecutive sessions on the same project with ≥80% shared initial file reads are flagged as MEDIUM
- [ ] Both heuristics default to disabled, opt-in via config or CLI
- [ ] H12 threshold: `subagent_overlap_pct` (default 50.0)
- [ ] H13 threshold: `session_restart_pct` (default 80.0), only compares first-N file reads (`session_restart_initial_ops`, default 10)
- [ ] Both produce actionable output with specific filenames
- [ ] Scenario tests: one waste case + normal sessions per heuristic
- [ ] All existing tests pass

## Dependencies

- **Must merge first:** N1 (data model expansion — needs `FileOps`, `ParentSessionID` on TokenEvent)
- **External dependencies:** None
- **Can be parallel with:** N2 (different files)
- **Breaking changes / Migrations needed:** New `WasteSignal` reason values: `"subagent_overlap"`, `"session_restart"`. Output formatters extended.

---

## Pre-flight

- [ ] Pull latest main: `git fetch origin && git checkout main && git pull origin main`
- [ ] Create feature branch: `git checkout -b n3-overlap-restart`
- [ ] Verify build environment works on clean main

## Files

| File | Purpose | Notes |
|------|---------|-------|
| `analyze/overlap.go` | `detectSubagentOverlap()` — compute Jaccard between parent and subagent FileOps | New file, ~80 lines |
| `analyze/overlap_test.go` | Tests: no overlap, partial, full, no subagents, empty parent | New file, ~120 lines |
| `analyze/restart.go` | `detectSessionRestarts()` — compare initial file reads across consecutive sessions | New file, ~100 lines |
| `analyze/restart_test.go` | Tests: fresh start, continued session, single session, mixed start | New file, ~130 lines |
| `analyze/waste.go` | Add H12/H13 calls in `DetectWaste`, gated by config | Modify, ~15 lines |
| `output/text.go` | Display format for new signal types | Modify, ~30 lines |
| `output/json.go` | JSON fields for new signal types | Modify, ~15 lines |
| `testdata/scenarios/subagent_overlap.jsonl` | Scenario: parent reads 4 files, subagent re-reads 3 of them | Already created |
| `testdata/scenarios/session_restart.jsonl` | Scenario: two sessions sharing initial reads at 80% | Already created |
| `testdata/labels/labels.jsonl` | Add labels for new scenario sessions | Modify |
| `output/scenario_test.go` | Add `TestScenario_SubagentOverlap`, `TestScenario_SessionRestart` | Modify, ~60 lines |

---

## Implementation

### H12 — Subagent Overlap (`analyze/overlap.go`)

```go
func detectSubagentOverlap(events []source.TokenEvent, trees []SubagentTree, thresholdPct float64) []WasteSignal {
    // For each SubagentTree:
    //   1. Collect parent's unique file paths from FileOps (operation=read)
    //   2. For each subagent, collect its unique file paths from FileOps (operation=read)
    //   3. Compute Jaccard: |parent ∩ subagent| / |parent ∪ subagent|
    //   4. If overlap > thresholdPct, flag with specific shared file list
}
```

**Algorithm:**
1. Map events by SessionID for quick lookup
2. For each `SubagentTree` from `BuildSubagentTree`:
   a. Find all events where `SessionID == tree.SessionID` → collect `unique(FileOps.Paths)` where `Operation == "read"` → `parentFiles`
   b. For each subagent in `tree.Subagents`, find events with matching `SessionID` → collect `unique(FileOps.Paths)` → `subFiles`
   c. Compute: `intersection = parentFiles ∩ subFiles`, `union = parentFiles ∪ subFiles`
   d. `jaccard = float64(len(intersection)) / float64(len(union))`
   e. If `jaccard > thresholdPct/100`, create WasteSignal with shared file list

3. Use `unique(paths []string) []string` helper to deduplicate paths before comparison.

**Signal format:**
```
Severity: "high"
Reason:   "subagent_overlap"
Detail:   "Parent session read 4 unique files. Subagent \"explore-1\" read 3 of the same 4.
           Overlap: 75% (3 shared: src/handler.go, src/types.go, config/settings.json)"
Metric:   jaccard * 100
Threshold: thresholdPct
```

**Edge cases:**
- No subagents → no signal
- Parent with 0 file reads → no signal (no files to compare)
- Multiple subagents → check each independently, produce one signal per overlapping subagent
- Parent reads `src/main.go` twice (deduped to 1 unique path)
- Subagent reads subset of parent files (partial overlap, e.g., 2/6 = 33%) → only flag if >50%

**Savings estimate:** Subagent cost (from `SubagentNode.Cost`). The subagent's entire cost is attributed to overlap waste.

### H13 — Session Restart (`analyze/restart.go`)

```go
func detectSessionRestarts(events []source.TokenEvent, thresholdPct float64, initialOps int) []WasteSignal {
    // Group sessions by project, sorted by timestamp
    // For consecutive sessions on same project:
    //   1. Extract first-N file reads from each session (EventIndex 1..N)
    //   2. Compute overlap percentage: |setA ∩ setB| / min(|setA|, |setB|) * 100
    //   3. If overlap >= thresholdPct, flag the LATER session as restart
}
```

**Algorithm:**
1. Group events by `(Project, SessionID)` 
2. Determine session start time: `min(Timestamp)` per session
3. Sort sessions by start time within each project
4. For each consecutive pair of sessions (A, B):
   a. Extract first `initialOps` FileOps from session A → `initialA` (unique paths, operation=read)
   b. Extract first `initialOps` FileOps from session B → `initialB` (unique paths, operation=read)
   c. `shared = len(initialA ∩ initialB)`
   d. `overlapPct = float64(shared) / float64(min(len(initialA), len(initialB))) * 100`
   e. If `overlapPct >= thresholdPct`, flag session B

**Signal format:**
```
Severity: "medium"
Reason:   "session_restart"
Detail:   "First 5 file reads are identical to previous session ses_restart_a.
           4 shared: src/main.go, src/types.go, config/settings.json, src/handler.go.
           Consider continuing the prior session instead of starting fresh."
Metric:   overlapPct
Threshold: thresholdPct
```

**Edge cases:**
- Only 1 session on project → no signal (no consecutive sessions to compare)
- Second session has < initialOps read events → use whatever is available
- Non-consecutive session comparisons → only compare adjacent sessions by time
- Session A has 10 file reads, session B has 3 → compare first 3 of A vs all 3 of B
- Same project, same day → multiple sessions compared pairwise

**Savings estimate:** `sessionCost * 0.5`. The first half of a restarted session is re-doing work the prior session already did.

### Integration into `DetectWaste`

```go
if cfg.Signals.SubagentOverlap {
    signals = append(signals, detectSubagentOverlap(events, trees, cfg.Thresholds.SubagentOverlapPct)...)
}
if cfg.Signals.SessionRestart {
    signals = append(signals, detectSessionRestarts(events, cfg.Thresholds.SessionRestartPct, cfg.Thresholds.SessionRestartInitialOps)...)
}
```

### Output format

```
  HIGH ses_overlap_parent (project): $5.60 — subagent re-did parent work
    Model: claude-sonnet-4-5-20250929, 700 in / 300 out
    → Parent session read 4 unique files.
    → Subagent "explore-1" read 2 of the same 4 files.
    → Overlap: 50% (2 shared: src/handler.go, src/router.go)
    → Subagent cost: $1.50. Potential savings: $1.50

  MEDIUM ses_restart_b (project): $2.20 — session restart detected
    Model: claude-sonnet-4-5-20250929, 450 in / 220 out
    → First 5 file reads are 80% identical to previous session ses_restart_a.
    → 4 shared: src/main.go, src/types.go, config/settings.json, src/handler.go
    → Consider continuing the prior session instead of starting fresh.
    → Potential savings: $1.10
```

---

## Test Requirements

### `analyze/overlap_test.go`

| Test | Input | Expected |
|------|-------|----------|
| No subagents | Parent only, no subagent events | no signals |
| No file ops | Parent with 0 ToolCalls/FileOps | no signals |
| No overlap | Parent reads file A, subagent reads file B | no signal (0% overlap) |
| Partial overlap | Parent reads [A,B,C], subagent reads [A,B] | no signal (40% < 50%) |
| Full overlap | Parent reads [A,B], subagent reads [A,B] | signal (100% overlap) |
| Threshold boundary | Parent reads [A,B,C,D], subagent reads [A,B,C] | signal if threshold=50 (75% > 50%) |
| Multiple subagents | Parent reads [A,B,C], sub1 reads [A,B,C], sub2 reads [D,E] | one signal for sub1, none for sub2 |
| Empty events | Empty event list | no signals |

### `analyze/restart_test.go`

| Test | Input | Expected |
|------|-------|----------|
| Single session | One session on project | no signals |
| Different projects | Sessions on different projects | no signals (not consecutive) |
| Fresh start | Session A reads [A,B,C], B reads [D,E,F] | no signal (0% overlap) |
| Restart | Session A reads [A,B,C], B reads [A,B,C] | signal for B (100% overlap) |
| Partial restart | Session A reads [A,B,C,D,E], B reads [A,B,C] | signal for B (60% < 80%? depends on threshold) |
| Continuation | Session A reads [A,B,C,D,E], B reads [C] | no signal (20% < 80%) |
| Different order | A reads [A,B,C], B reads [B,C,A] | same set → 100% overlap → signal |
| initialOps limit | A reads 5 files, B reads 3, initialOps=5 | compare first 3 of A vs first 3 of B |
| Multiple consecutive | A, B, C sessions | compare A→B, B→C separately |

### Scenario tests in `output/scenario_test.go`

```go
func TestScenario_SubagentOverlap(t *testing.T) {
    events := loadScenarioJSONL(t, "subagent_overlap.jsonl")
    trees := analyze.BuildSubagentTree(events)
    cfg := config.Defaults()
    cfg.Signals.SubagentOverlap = true
    cfg.Thresholds.SubagentOverlapPct = 50.0

    baselines := analyze.ComputeBaselines(events, config.Defaults())
    signals := analyze.DetectWaste(events, baselines, trees, cfg)

    sig := findSignalByID(signals, "ses_overlap_parent")
    if sig == nil {
        t.Fatal("expected ses_overlap_parent to be flagged as subagent_overlap")
    }
    if sig.Reason != "subagent_overlap" {
        t.Errorf("expected reason subagent_overlap, got %s", sig.Reason)
    }

    // Normal sessions should NOT be flagged
    for _, id := range []string{"ses_overlap_normal_01", "ses_overlap_normal_02"} {
        if s := findSignalByID(signals, id); s != nil && s.Reason == "subagent_overlap" {
            t.Errorf("normal session %s was flagged as subagent_overlap", id)
        }
    }
}

func TestScenario_SessionRestart(t *testing.T) {
    events := loadScenarioJSONL(t, "session_restart.jsonl")
    cfg := config.Defaults()
    cfg.Signals.SessionRestart = true
    cfg.Thresholds.SessionRestartPct = 80.0
    cfg.Thresholds.SessionRestartInitialOps = 10

    baselines := analyze.ComputeBaselines(events, config.Defaults())
    signals := analyze.DetectWaste(events, baselines, nil, cfg)

    sig := findSignalByID(signals, "ses_restart_b")
    if sig == nil {
        t.Fatal("expected ses_restart_b to be flagged as session_restart")
    }
    if sig.Reason != "session_restart" {
        t.Errorf("expected reason session_restart, got %s", sig.Reason)
    }

    // Continued session should NOT be flagged
    if s := findSignalByID(signals, "ses_restart_continued"); s != nil && s.Reason == "session_restart" {
        t.Errorf("ses_restart_continued was flagged unexpectedly as session_restart")
    }
}
```

### Labels update

```jsonl
{"session_id":"ses_overlap_parent","verdict":"waste","reason":"subagent re-read 2 of 4 parent files","source":"scenario","created_at":"2026-05-03T00:00:00Z"}
{"session_id":"ses_overlap_sub","verdict":"waste","reason":"re-did parent work (overlap)","source":"scenario","created_at":"2026-05-03T00:00:00Z"}
{"session_id":"ses_overlap_normal_01","verdict":"not_waste","reason":"normal session, no subagents","source":"scenario","created_at":"2026-05-03T00:00:00Z"}
{"session_id":"ses_overlap_normal_02","verdict":"not_waste","reason":"normal session, no subagents","source":"scenario","created_at":"2026-05-03T00:00:00Z"}
{"session_id":"ses_restart_a","verdict":"not_waste","reason":"first session, nothing to compare","source":"scenario","created_at":"2026-05-03T00:00:00Z"}
{"session_id":"ses_restart_b","verdict":"waste","reason":"80% initial file overlap with prior session","source":"scenario","created_at":"2026-05-03T00:00:00Z"}
{"session_id":"ses_restart_continued","verdict":"not_waste","reason":"continued from prior session (1 shared file)","source":"scenario","created_at":"2026-05-03T00:00:00Z"}
{"session_id":"ses_restart_standalone","verdict":"not_waste","reason":"different project/context, fresh start","source":"scenario","created_at":"2026-05-03T00:00:00Z"}
```

---

## Approach

1. Write `analyze/overlap_test.go` (RED)
2. Implement `analyze/overlap.go` (GREEN)
3. Write `analyze/restart_test.go` (RED)
4. Implement `analyze/restart.go` (GREEN)
5. Wire H12/H13 into `DetectWaste`
6. Update output formatters
7. Add scenario tests
8. Update labels
9. Run full test suite + lint

---

## Exit Criteria

- [ ] Pull latest main
- [ ] Create feature branch from main: `git checkout -b n3-overlap-restart`
- [ ] Lint passes (zero warnings) — `golangci-lint run`
- [ ] Build compiles cleanly — `go build -o burnwatch .`
- [ ] Tests pass with >=90% coverage on new code — `go test ./... -cover`
- [ ] Self-review: run `./scripts/review-check.sh`, then verify Phases 1-3 in `docs/code-review.md`
- [ ] Document learnings in `docs/learnings.md`
- [ ] Commit: `feat: subagent overlap and session restart detection (H12, H13)`
- [ ] Push to branch `n3-overlap-restart`
- [ ] Open pull request
- [ ] Dispatch CodeReviewer subagent against the PR diff
- [ ] Update `docs/plans/progress.md` to reflect merge
- [ ] Merge to main
- [ ] Delete feature branch after merge

---

## Notes

- **H12 reuses `BuildSubagentTree`.** The tree already contains SessionID→SubagentNode mappings. H12 adds file operation comparison on top. No changes to `analyze/subagent.go`.
- **Jaccard overlap is used because it's self-normalizing.** A parent reading 100 files with subagent reading 50 of them (50% overlap) is treated the same as parent reading 4 files with subagent reading 2 (also 50%). Both cases are half the work being re-done.
- **H13 compares first-N file reads only**, not all file operations. Why? Because the first operations of a session are typically context-loading (reading project files to understand the codebase). Later operations are task-specific and expected to differ. The first-N read pattern is the fingerprint that identifies a "fresh start" vs a "continuation."
- **H13 groups by project.** Two consecutive sessions on `burnwatch` project are compared. A session on `burnwatch` followed by a session on `other-repo` are NOT compared — different projects, different context.
- **Session ordering by timestamp, not session ID.** Session IDs are UUIDs with no ordering. Use `min(Timestamp)` per session as the start time for ordering.
- **File path normalization is critical for H12/H13.** If Claude stores `src/main.go` (relative after N1 normalization) and OpenCode stores `./src/main.go`, the overlap computation would miss the match. N1's path normalization in the source layer MUST strip leading `./` and normalize separators. If N1 doesn't do this, H12/H13 must do it in their analysis.
