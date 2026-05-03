# PR10: Phase C — Deeper Improvements

> **Workflow:** Follow `docs/plans/PR-template.md`. Review `AGENTS.md` behavioral guidelines before implementing.

## Objective

Make burnwatch output more actionable:
1. **C1:** Show model + token counts in waste signals (so users know what to investigate)
2. **C2:** Signal toggle flags (`--no-cost-outlier`, `--no-churn`, etc.) for selective reporting
3. **C3:** Time-trend summary — are costs/efficiency improving or worsening?

## Success Criteria

- [ ] Cost outlier line shows model name and input/output token counts
- [ ] `--no-low-signal` suppresses all low_signal waste signals
- [ ] `--no-churn` suppresses all session_churn waste signals
- [ ] Trends section appears between projects and waste signals sections
- [ ] Trends show cost, ratio, and session count direction over the lookback period
- [ ] When `--days 7`, trends compare first 3 days vs last 3 days
- [ ] When `--days 0` (all time), trends compare first month vs latest month

## Dependencies

- **Must merge first:** PR7 (config), PR8 (display), PR9 (noise reduction)
- **External dependencies:** None
- **Can be parallel with:** None (serial after PR9)
- **Breaking changes / Migrations needed:** Golden file update

---

## Pre-flight

- [ ] Pull latest main: `git fetch origin && git checkout main && git pull origin main`
- [ ] Create feature branch: `git checkout -b pr10-deeper-insights`
- [ ] Verify build environment works on clean main

## Files

| File | Purpose | Notes |
|------|---------|-------|
| `analyze/waste.go` | C1: add model + tokens to WasteSignal and sessionAgg | Modify |
| `analyze/trend.go` | C3: weekly aggregation + trend direction | New file |
| `analyze/trend_test.go` | Trend computation tests | New file |
| `output/text.go` | C1: show model/tokens. C3: trends section. | Modify |
| `output/text_test.go` | Update golden expectations | |
| `cmd/root.go` | C2: signal toggle flags | Modify |
| `testdata/expected_report.txt` | Regenerate | |

---

## Implementation

### C1: Per-signal model + token context

**Add to `WasteSignal` struct** (`analyze/waste.go:12`):
```go
type WasteSignal struct {
    // ... existing fields ...
    SessionID   string
    Project     string
    Severity    string
    Reason      string
    Detail      string
    Metric      float64
    Threshold   float64
    SessionCost float64

    // C1: investigation context
    Model        string
    InputTokens  int64
    OutputTokens int64
}
```

**Populate in `DetectWaste`:** Already have `a.inputSum`, `a.outputSum` from aggregation. Need to track the model. In the event loop where we build `sessionAgg`, also capture the primary model (use the model from the first assistant event or the one with highest cost):

```go
type sessionAgg struct {
    // ... existing fields ...
    model      string
    inputSum   int64
    outputSum  int64
    // ...
}

// In DetectWaste, track model:
a.model = e.Model  // last model wins (simplest), or track per-model cost
```

**Display in `output/text.go`** — update `writeSignalBlock`:
```go
case "cost_outlier":
    // ... existing multiplier line ...
    if s.Model != "" {
        fmt.Fprintf(b, "    Model: %s, %s in / %s out\n",
            s.Model, formatTokens(s.InputTokens), formatTokens(s.OutputTokens))
    }
```

Helper:
```go
func formatTokens(n int64) string {
    if n >= 1_000_000 {
        return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
    }
    if n >= 1_000 {
        return fmt.Sprintf("%.1fK", float64(n)/1_000)
    }
    return fmt.Sprintf("%d", n)
}
```

### C2: Signal toggle flags

Add CLI flags that override the config `[signals]` toggles:
```go
var flags struct {
    // ... existing ...
    NoCostOutlier      bool
    NoLowSignal        bool
    NoSubagentOverhead bool
    NoCacheUnderutil   bool
    NoSessionChurn     bool
}

flag.BoolVar(&flags.NoCostOutlier, "no-cost-outlier", false, "Disable cost outlier detection")
flag.BoolVar(&flags.NoLowSignal, "no-low-signal", false, "Disable low output/input ratio detection")
flag.BoolVar(&flags.NoSubagentOverhead, "no-subagent-overhead", false, "Disable subagent overhead detection")
flag.BoolVar(&flags.NoCacheUnderutil, "no-cache-underutil", false, "Disable cache underutilization detection")
flag.BoolVar(&flags.NoSessionChurn, "no-churn", false, "Disable session churn detection")
```

**Merge logic** in `cmd/root.go`:
```go
// CLI flags override config
cfg.Signals.CostOutlier = cfg.Signals.CostOutlier && !flags.NoCostOutlier
cfg.Signals.LowSignal = cfg.Signals.LowSignal && !flags.NoLowSignal
cfg.Signals.SubagentOverhead = cfg.Signals.SubagentOverhead && !flags.NoSubagentOverhead
cfg.Signals.CacheUnderutilized = cfg.Signals.CacheUnderutilized && !flags.NoCacheUnderutil
cfg.Signals.SessionChurn = cfg.Signals.SessionChurn && !flags.NoSessionChurn
```

**Wire into `DetectWaste`:** Pass the config signals as a struct or map:
```go
type SignalToggles struct {
    CostOutlier        bool
    LowSignal          bool
    SubagentOverhead   bool
    CacheUnderutilized bool
    SessionChurn       bool
}
```

In `DetectWaste`, guard each check:
```go
if toggles.CostOutlier {
    if signal := checkCostOutlier(a, bl, sigma); signal != nil {
        signals = append(signals, *signal)
    }
}
// ... etc for each signal type
```

### C3: Time-trend summary

**`analyze/trend.go`:**

```go
type WeeklyAgg struct {
    WeekStart    time.Time
    SessionCount int
    TotalCost    float64
    TotalInput   int64
    TotalOutput  int64
    Ratio        float64 // avg output/input
}

type Trend struct {
    CostDirection    string // "↑" or "↓"
    CostChange       float64 // percent
    RatioDirection   string
    RatioChange      float64
    SessionDirection string
    SessionChange    float64
}

func ComputeTrends(events []source.TokenEvent) *Trend {
    // 1. Group events by week (Mon-Sun)
    // 2. Compute WeeklyAgg per week
    // 3. Compare first meaningful week to last meaningful week
    // 4. Return Trend with direction arrows
}
```

**Algorithm:**
1. Truncate each event timestamp to Monday of that week.
2. Group by week, sum costs, count sessions, sum tokens.
3. Sort weeks chronologically.
4. If <= 1 week of data: return nil (not enough data).
5. If >= 4 weeks: compare first week vs last week.
6. If 2-3 weeks: compare first vs last.
7. Return changes as percentages.

**Output in text** (before waste signals, after projects):
```
Trends:
  Cost:    $1,234.56/wk → $987.65/wk (↓ 20%)
  Sessions: 45/wk → 36/wk (↓ 20%)
  Output/input ratio: 0.12 → 0.18 (↑ 50%)
```

Controlled by `config.Output.ShowTrends` (default: false in config, shown via `--show-trends` flag).

---

## Test Requirements

### `analyze/waste_test.go`

| Test | Input | Expected |
|------|-------|----------|
| `TestWasteSignalHasModel` | events with known model | WasteSignal.Model populated |
| `TestDetectWaste_ToggleOff` | events, all toggles false | zero signals |

### `analyze/trend_test.go`

| Test | Input | Expected |
|------|-------|----------|
| `TestComputeTrends_SingleWeek` | 1 week of data | nil (not enough data) |
| `TestComputeTrends_TwoWeeks` | 2 weeks, costs [100, 80] | CostDirection = "↓", CostChange = -20% |
| `TestComputeTrends_RatioUp` | 2 weeks, ratios [0.10, 0.15] | RatioDirection = "↑", RatioChange = +50% |
| `TestComputeTrends_NoChange` | 2 identical weeks | all changes = 0% |
| `TestComputeTrends_Empty` | no events | nil |
| `TestWeeklyAggregation` | events across 8 days spanning 2 weeks | 2 WeeklyAggs, correct boundaries |

### `output/text_test.go`
- Update golden file for new model/token lines
- Test trends section formatting

### `cmd/root_test.go`
- Test `--no-churn` flag suppresses churn signals
- Test `--no-cost-outlier` flag suppresses cost signals
- Test all flags together produce expected output

**Coverage target:** >=90% on new + modified code.

---

## Approach

1. Add model tracking to `sessionAgg` and populate in `DetectWaste` (RED test: `TestWasteSignalHasModel`)
2. Add `Model`, `InputTokens`, `OutputTokens` to `WasteSignal`
3. Display model/tokens in text output → regenerate golden file
4. Add `SignalToggles` struct and wire into `DetectWaste`
5. Add `--no-*` flags to CLI, merge with config
6. Write `analyze/trend.go` and tests (RED first)
7. Add trends section to text output
8. Add `--show-trends` flag
9. Run full test suite, regenerate golden file

---

## Exit Criteria

- [ ] Pull latest main
- [ ] Create feature branch from main: `git checkout -b pr10-deeper-insights`
- [ ] Lint passes (zero warnings) — `go vet ./... && golangci-lint run`
- [ ] Build compiles cleanly — `go build -o burnwatch .`
- [ ] Tests pass with >=90% coverage — `go test ./... -cover`
- [ ] `./burnwatch` shows model + token counts in cost outlier lines
- [ ] `./burnwatch --no-churn` has zero session_churn signals
- [ ] `./burnwatch --show-trends` displays trends section between projects and waste signals
- [ ] Trends show correct direction and percentage for cost, ratio, sessions
- [ ] Self-review: run through [docs/code-review.md](../code-review.md)
- [ ] Document learnings in `docs/learnings.md`
- [ ] Commit: `feat: model/token context, signal toggles, time-trend analysis`
- [ ] Push to branch `pr10-deeper-insights`
- [ ] Open pull request with description
- [ ] Perform code review
- [ ] Update `docs/plans/progress.md` to reflect merge
- [ ] Merge to main
- [ ] Delete feature branch after merge

## Notes

- `formatTokens` should handle edge cases: 0 tokens, negative (clamped), very large numbers.
- For model tracking: simple approach is to use the model from the first assistant event. If a session uses multiple models, this is an approximation — good enough for v1.
- Trend direction arrows use Unicode (↑ ↓). If terminal compatibility is a concern, use "up" / "down" text instead.
- `--show-trends` requires enough data. If < 2 weeks, print "Not enough data for trends (need >= 2 weeks)." instead of nothing.
