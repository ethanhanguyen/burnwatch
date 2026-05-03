# PR8: Phase A — Display Fixes

> **Workflow:** Follow `docs/plans/PR-template.md`. Review `AGENTS.md` behavioral guidelines before implementing.

## Objective

Fix three issues in the text output formatter:
1. **A1:** Misleading "N.Nx project baseline" label on cost outliers (shows ratio to μ+2σ, not μ)
2. **A2:** Session churn signals produce one line per session — group into per-day summaries
3. **A3:** Session churn detail is hidden — show the date

## Success Criteria

- [ ] Cost outlier line shows true multiple of project mean (e.g., "16.6x project baseline (μ = $2,844.48)")
- [ ] Session churn produces one line per (project, date) group instead of one per session
- [ ] Session churn line includes date: "lilysbeauty: 29 sessions on 2026-04-15 below mean ratio (0.0123)"
- [ ] Golden file `testdata/expected_report.txt` updated and matches
- [ ] All existing tests still pass
- [ ] No new dependencies

## Dependencies

- **Must merge first:** PR7 (config file, for `GroupChurn` toggle)
- **External dependencies:** None
- **Can be parallel with:** PR9 (both depend on PR7)
- **Breaking changes / Migrations needed:** Golden file must be regenerated

---

## Pre-flight

- [ ] Pull latest main: `git fetch origin && git checkout main && git pull origin main`
- [ ] Create feature branch: `git checkout -b pr8-display-fixes`
- [ ] Verify build environment works on clean main

## Files

| File | Purpose | Notes |
|------|---------|-------|
| `output/text.go` | A1: fix label + add helper. A2: group churn signals. A3: show date. | Core changes |
| `output/text_test.go` | Update golden file test expectations | |
| `testdata/expected_report.txt` | Regenerate with `go test -update` | |

---

## Implementation

### A1: Fix cost_outlier label

**Current code** (`output/text.go:242`):
```go
case "cost_outlier":
    fmt.Fprintf(b, " — %.1fx project baseline\n", s.Metric/s.Threshold)
```

**Problem:** `s.Metric` = sessionCost, `s.Threshold` = μ + 2σ. The display shows `sessionCost / (μ+2σ)`, not `sessionCost / μ`. A $47K session at 16x the mean shows "3.8x".

**Fix:** The text formatter needs access to baselines to look up `CostMean`. Add a `baselines` parameter to `writeSignalBlock`:

```go
func writeSignalBlock(b *strings.Builder, s analyze.WasteSignal, 
    rec analyze.Recommendation, baselines map[string]analyze.Baseline) {
    // ...
    case "cost_outlier":
        bl := findBaselineForSignal(s, baselines)
        if bl != nil && bl.CostMean > 0 {
            mult := s.SessionCost / bl.CostMean
            fmt.Fprintf(b, " — %.1fx project baseline (μ = $%.2f)\n", mult, bl.CostMean)
        } else {
            fmt.Fprintf(b, " — %.1fx outlier threshold\n", s.Metric/s.Threshold)
        }
}
```

Helper:
```go
func findBaselineForSignal(s analyze.WasteSignal, baselines map[string]analyze.Baseline) *analyze.Baseline {
    for k, bl := range baselines {
        if k == "*" {
            continue
        }
        if strings.HasPrefix(k, s.Project+":") {
            return &bl
        }
    }
    // fallback to global
    if gl, ok := baselines["*"]; ok {
        return &gl
    }
    return nil
}
```

**Caller change** in `FormatText`: pass `baselines` to `writeSignalBlock`.

### A2: Group session_churn signals

**Current behavior:** Each churned session gets its own signal line. One day with 66 churned sessions = 66 lines.

**Fix:** In `FormatText`, collect all `session_churn` signals, group by (Project, day extracted from Detail), and write one summary line per group:

```go
func writeChurnGroups(b *strings.Builder, signals []analyze.WasteSignal, 
    recBySignal map[analyze.WasteSignal]analyze.Recommendation) {
    
    type churnKey struct {
        project string
        date    string // from Detail
    }
    groups := make(map[churnKey][]analyze.WasteSignal)
    
    for _, s := range signals {
        if s.Reason != "session_churn" {
            continue
        }
        // Extract date from Detail: "Project X had N sessions on YYYY-MM-DD, all below..."
        date := extractDateFromDetail(s.Detail)
        key := churnKey{s.Project, date}
        groups[key] = append(groups[key], s)
    }
    
    for key, sigs := range groups {
        totalCost := 0.0
        totalSavings := 0.0
        for _, s := range sigs {
            totalCost += s.SessionCost
            if rec, ok := recBySignal[s]; ok {
                totalSavings += rec.SavingsEst
            }
        }
        fmt.Fprintf(b, "  MEDIUM %s on %s: %.0f sessions below mean ratio, $%.2f total\n",
            key.project, key.date, float64(len(sigs)), totalCost)
        fmt.Fprintf(b, "    → Consolidate fragmented sessions. Potential savings: $%.2f\n", totalSavings)
    }
}
```

Guard behind `config.Output.GroupChurn` (from PR7). When true, churn is grouped. When false, individual lines (existing behavior).

### A3: Show date in individual churn lines

When `GroupChurn` is false, the individual churn line should show the date. Update:

```go
case "session_churn":
    fmt.Fprintf(b, " — %.0f sessions below mean ratio (%s)\n", s.Metric, extractDateFromDetail(s.Detail))
```

The date is already in `s.Detail` from `checkSessionChurn` at `analyze/waste.go:272`.

### Caller changes in `FormatText`

Current signature doesn't pass baselines. Update:
```go
func FormatText(events, baselines, signals, recommendations, verbose bool, 
    cfg config.Config) string {
    // ...
    writeSignalBlock(&b, s, rec, baselines)  // added baselines
    // ...
    // After signal loop, handle churn groups:
    writeChurnGroups(&b, signals, recBySignal)
}
```

---

## Test Requirements

1. **`output/text_test.go`**:
   - Update golden file test to match new format
   - Test cost outlier line shows correct baseline multiplier
   - Test churn group output (when grouped)
   - Test churn individual output (when not grouped)
   - Test `findBaselineForSignal` helper

2. Coverage target: >=90% on modified code

---

## Approach

1. Add `findBaselineForSignal` helper to `output/text.go`
2. Fix A1: pass baselines to `writeSignalBlock`, fix label formula
3. Add `extractDateFromDetail` helper
4. Implement A2: `writeChurnGroups` with grouping logic
5. Fix A3: update individual churn line format
6. Wire `config.Output.GroupChurn` control
7. Regenerate golden file
8. Run tests

---

## Exit Criteria

- [ ] Pull latest main
- [ ] Create feature branch from main: `git checkout -b pr8-display-fixes`
- [ ] Lint passes (zero warnings) — `go vet ./... && golangci-lint run`
- [ ] Build compiles cleanly — `go build -o burnwatch .`
- [ ] Tests pass with >=90% coverage — `go test ./... -cover`
- [ ] Golden file matches: `go test ./output/ -update && go test ./...`
- [ ] Manual: run `./burnwatch` and verify cost labels show correct multiplier
- [ ] Manual: verify churn groups collapse correctly
- [ ] Self-review: run through [docs/code-review.md](../code-review.md)
- [ ] Document learnings in `docs/learnings.md`
- [ ] Commit: `fix: correct cost outlier label, group churn signals, show dates`
- [ ] Push to branch `pr8-display-fixes`
- [ ] Open pull request with description
- [ ] Perform code review
- [ ] Update `docs/plans/progress.md` to reflect merge
- [ ] Merge to main
- [ ] Delete feature branch after merge

## Notes

- `extractDateFromDetail` parses the date from churn detail strings like `"Project X had 29 sessions on 2026-04-15, all below mean ratio (0.0123)"`. Use a simple regex or string splitting. The format is deterministic (set in `checkSessionChurn`).
- The `baselines` parameter must be passed through `FormatText` → `writeSignalBlock`. Update all callers.
- Golden file update: run `go test -update` in `output/` package after making changes.
