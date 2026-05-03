# PR9: Phase B — Noise Reduction

> **Workflow:** Follow `docs/plans/PR-template.md`. Review `AGENTS.md` behavioral guidelines before implementing.

## Objective

Reduce the 765-signal firehose to an actionable report through four mechanisms:
1. **B1:** Min-cost filter — hide signals below a dollar threshold
2. **B2:** Severity deduplication — keep only the highest-severity signal per session
3. **B3:** Configurable sigma — let users tighten cost outlier threshold (e.g., 3σ)
4. **B4:** Fix cost double-counting — use consistent session cost across all signal types

## Success Criteria

- [ ] `--min-cost 1.00` filters out all signals with SessionCost < $1.00
- [ ] A session with both HIGH and MEDIUM signals only appears once (with HIGH)
- [ ] `config.thresholds.cost_outlier_sigma = 3.0` reduces cost outlier signals
- [ ] Same session shows same cost regardless of signal type
- [ ] Existing tests pass (may need golden file update for B4 cost changes)
- [ ] No signals with `$0.00` cost in output

## Dependencies

- **Must merge first:** PR7 (config file)
- **External dependencies:** None
- **Can be parallel with:** PR8 (display fixes — both depend on PR7)
- **Breaking changes / Migrations needed:** Golden file may change due to B4 cost fix

---

## Pre-flight

- [ ] Pull latest main: `git fetch origin && git checkout main && git pull origin main`
- [ ] Create feature branch: `git checkout -b pr9-noise-reduction`
- [ ] Verify build environment works on clean main

## Files

| File | Purpose | Notes |
|------|---------|-------|
| `analyze/signal_filter.go` | B1: min-cost filter, B2: severity dedup | New file |
| `analyze/signal_filter_test.go` | Filter and dedup tests | New file |
| `analyze/waste.go` | B3: sigma param, B4: cost fix | Modify |
| `analyze/waste_test.go` | Tests for B3 and B4 | Modify |
| `cmd/root.go` | Wire `--min-cost` flag, pass sigma from config | Modify |
| `output/text_test.go` | Update golden expectations post-cost-fix | Modify |
| `testdata/expected_report.txt` | Regenerate | |

---

## Implementation

### B1: Min-cost filter (`analyze/signal_filter.go`)

```go
func FilterByMinCost(signals []WasteSignal, minCost float64) []WasteSignal {
    if minCost <= 0 {
        return signals
    }
    var filtered []WasteSignal
    for _, s := range signals {
        if s.SessionCost >= minCost {
            filtered = append(filtered, s)
        }
    }
    return filtered
}
```

Applied in `cmd/root.go` after detection, before output:
```go
signals = analyze.DetectWaste(events, baselines)
if cfg.Filters.MinCost > 0 {
    signals = analyze.FilterByMinCost(signals, cfg.Filters.MinCost)
}
```

**CLI flag:**
```go
flag.Float64Var(&flags.MinCost, "min-cost", 0, "Hide waste signals below this dollar amount")
```
CLI flag overrides config value: if `--min-cost` is explicitly set (non-zero), use it instead of config.

### B2: Severity dedup (`analyze/signal_filter.go`)

```go
func Deduplicate(signals []WasteSignal) []WasteSignal {
    if len(signals) == 0 {
        return signals
    }
    
    best := make(map[string]WasteSignal)
    for _, s := range signals {
        existing, ok := best[s.SessionID]
        if !ok || signalRank(s) > signalRank(existing) {
            best[s.SessionID] = s
        }
    }
    
    result := make([]WasteSignal, 0, len(best))
    for _, s := range best {
        result = append(result, s)
    }
    
    // Sort consistent with DetectWaste output: severity desc, reason asc, sessionID asc
    sortSignals(result)
    return result
}

func signalRank(s WasteSignal) int {
    // Severity: high=3, medium=2, low=1 (first sort key)
    sev := map[string]int{"high": 6, "medium": 4, "low": 2}
    // Reason priority within severity (second sort key)
    reason := map[string]int{
        "cost_outlier": 5,
        "subagent_overhead": 4,
        "session_churn": 3,
        "low_signal": 2,
        "cache_underutilized": 1,
    }
    return sev[s.Severity] + reason[s.Reason]
}
```

Tiebreaking: if two signals of same severity+reason for different sessions, it doesn't matter (different sessionIDs, not deduped).

Applied after min-cost filter (config-controlled):
```go
if cfg.Filters.Deduplicate {
    signals = analyze.Deduplicate(signals)
}
```

### B3: Configurable sigma multiplier (`analyze/waste.go`)

Change `checkCostOutlier` signature to accept sigma:

```go
func checkCostOutlier(a *sessionAgg, bl Baseline, sigma float64) *WasteSignal {
    if bl.SessionCount < 2 {
        return nil
    }
    threshold := bl.CostMean + sigma*bl.CostStd
    if a.cost > threshold {
        // ... same as before, but detail message includes sigma
        return &WasteSignal{
            // ...
            Detail: fmt.Sprintf("Session cost $%.2f exceeds project baseline μ+%.0fσ = $%.2f", 
                a.cost, sigma, threshold),
        }
    }
    return nil
}
```

`DetectWaste` needs a new parameter:
```go
func DetectWaste(events []source.TokenEvent, baselines map[string]Baseline, costSigma float64) []WasteSignal {
    // ...
    if signal := checkCostOutlier(a, bl, costSigma); signal != nil {
        signals = append(signals, *signal)
    }
    // ...
}
```

**Constraint:** Sigma must be > 0. Default is 2.0. Config-driven.

### B4: Fix cost double-counting (`analyze/waste.go`)

**Problem:** In `DetectWaste`, `a.cost` sums only parent session events (line 57). But `checkSubagentOverhead` uses `tree.TotalCost` which includes subagent costs. The same session reports different costs in different signal types.

**Fix:** After building aggregation and trees, override `a.cost` with the tree's TotalCost when available:

```go
func DetectWaste(events []source.TokenEvent, baselines map[string]Baseline, costSigma float64) []WasteSignal {
    // ... existing aggregation (a.cost = sum of parent events) ...
    // ... existing ratio/cacheRate computation ...

    trees := BuildSubagentTree(events)
    treeBySession := make(map[string]*SubagentTree)
    for i := range trees {
        treeBySession[trees[i].SessionID] = &trees[i]
    }

    // B4: Use tree total cost when available (includes subagent costs)
    for _, a := range agg {
        if tree := treeBySession[a.sessionID]; tree != nil && tree.TotalCost > 0 {
            a.cost = tree.TotalCost
        }
    }

    // ... existing signal detection (now uses consistent cost) ...
}
```

This ensures `a.cost` (used by cost outlier, low signal, cache checks, session churn) matches `tree.TotalCost` (used by subagent overhead).

---

## Test Requirements

### `analyze/signal_filter_test.go`

| Test | Input | Expected |
|------|-------|----------|
| `TestFilterByMinCost_ZeroThreshold` | signals with costs [0, 5, 10], min=0 | all 3 pass |
| `TestFilterByMinCost_Negative` | min=-1 | all pass (no-op) |
| `TestFilterByMinCost_Filters` | signals with costs [0.50, 2.00, 5.00], min=2.00 | 2 pass (2.00, 5.00) |
| `TestFilterByMinCost_AllFiltered` | all below min | empty slice |
| `TestDeduplicate_SinglePerSession` | all unique sessions | same signals |
| `TestDeduplicate_HighBeatsMedium` | 1 session, HIGH + MEDIUM | only HIGH survives |
| `TestDeduplicate_MediumBeatsLow` | 1 session, MEDIUM + LOW | only MEDIUM survives |
| `TestDeduplicate_CostBeatsChurn` | same severity, cost_outlier vs session_churn | cost_outlier wins |
| `TestDeduplicate_MultiSession` | 3 sessions, some with duplicates | 3 signals, one per session |

### `analyze/waste_test.go`

| Test | Input | Expected |
|------|-------|----------|
| `TestCheckCostOutlier_Sigma3` | session cost, baseline, sigma=3 | higher threshold, fewer flags |
| `TestCheckCostOutlier_Sigma1` | session cost, baseline, sigma=1 | lower threshold, more flags |
| `TestCheckCostOutlier_ZeroSigma` | sigma=0 | no-op or error |
| `TestDetectWaste_WithSigma` | events with subagent costs | verify sigma propagated |
| `TestCostConsistency` | events with subagents | same session cost in all signal types |

**Coverage target:** >=90% on new + modified code.

---

## Approach

1. Write `analyze/signal_filter_test.go` (RED)
2. Implement `analyze/signal_filter.go` (GREEN)
3. Add B3: sigma param to `checkCostOutlier` and `DetectWaste` (RED tests first)
4. Add B4: cost consistency fix in `DetectWaste` (RED tests first)
5. Update `cmd/root.go`: wire `--min-cost`, pass sigma from config
6. Regenerate golden file (cost fix changes report numbers)
7. Run full test suite

---

## Exit Criteria

- [ ] Pull latest main
- [ ] Create feature branch from main: `git checkout -b pr9-noise-reduction`
- [ ] Lint passes (zero warnings) — `go vet ./... && golangci-lint run`
- [ ] Build compiles cleanly — `go build -o burnwatch .`
- [ ] Tests pass with >=90% coverage — `go test ./... -cover`
- [ ] `./burnwatch --min-cost 1.00` filters out sub-$1 signals
- [ ] With config `deduplicate=true`, no session appears twice in waste signals
- [ ] With config `cost_outlier_sigma=3.0`, fewer HIGH signals than default 2.0
- [ ] Same session shows consistent cost across signal types
- [ ] Self-review: run through [docs/code-review.md](../code-review.md)
- [ ] Document learnings in `docs/learnings.md`
- [ ] Commit: `feat: add min-cost filter, dedup, configurable sigma, fix cost consistency`
- [ ] Push to branch `pr9-noise-reduction`
- [ ] Open pull request with description
- [ ] Perform code review
- [ ] Update `docs/plans/progress.md` to reflect merge
- [ ] Merge to main
- [ ] Delete feature branch after merge

## Notes

- B4 cost fix changes signal costs. Golden file MUST be regenerated. Run `go test -update` in `output/` after making the fix.
- Sigma change is backward-compatible: default 2.0 preserves existing behavior. Only changes when config overrides.
- The `DetectWaste` signature change (adding `costSigma` param) breaks callers. Update `cmd/root.go` and any test callers.
- Min-cost filter is POST-detection (doesn't affect baselines). This is intentional — baselines should see all data even if we hide small signals.
