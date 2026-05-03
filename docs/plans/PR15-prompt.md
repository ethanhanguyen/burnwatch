# PR15: Calibration Mode — Distribution Analysis + Threshold Recommendations

> **Workflow:** Follow `docs/PR-template.md`. Review `AGENTS.md` behavioral guidelines before implementing.

## Objective

Add a `--calibrate` mode that prints the full statistical distribution of every metric across all user sessions and suggests threshold values based on the data. Turns the "statistically calibrated" claim into a real user workflow.

## Success Criteria

- [ ] `burnwatch --calibrate` prints full distribution for: session cost, input tokens, output tokens, output/input ratio, cache hit rate, token efficiency ratio, subagent overhead %
- [ ] Each distribution shows: count, mean, σ, P10, P25, P50, P75, P90, P95, P99
- [ ] Output includes "Suggested thresholds" section with config file snippets
- [ ] `--calibrate --json` outputs JSON format for machine consumption
- [ ] `--calibrate` respects `--harness`, `--project`, `--days` filters
- [ ] `--calibrate` output is compact enough to read on a terminal (<80 lines)
- [ ] Works with zero-config (no `.burnwatch.toml` needed)
- [ ] All existing tests pass (no regressions)

## Dependencies

- **Must merge first:** PR13 (token heuristics — calibration analyzes the same metrics)
- **External dependencies:** None
- **Can be parallel with:** PR14 (different files, no shared state)
- **Breaking changes / Migrations needed:** None (new mode, additive)

---

## Pre-flight

- [ ] Pull latest main: `git fetch origin && git checkout main && git pull origin main`
- [ ] Create feature branch: `git checkout -b pr15-calibrate`
- [ ] Verify build environment works on clean main

## Files

| File | Purpose | Notes |
|------|---------|-------|
| `analyze/calibrate.go` | CalibrationReport struct, ComputeCalibration function | New file |
| `analyze/calibrate_test.go` | Test distribution computation, suggestion logic | New file |
| `output/calibrate_text.go` | Text-formatted calibration output | New file |
| `output/calibrate_json.go` | JSON-formatted calibration output | New file |
| `output/calibrate_test.go` | Test both output formats | New file |
| `cmd/root.go` | Add `--calibrate` flag, route to calibration output | Modify |

---

## Implementation

### `analyze/calibrate.go` — CalibrationReport

```go
package analyze

type DistStats struct {
    Count  int       `json:"count"`
    Mean   float64   `json:"mean"`
    Std    float64   `json:"std"`
    P10    float64   `json:"p10"`
    P25    float64   `json:"p25"`
    P50    float64   `json:"p50"`
    P75    float64   `json:"p75"`
    P90    float64   `json:"p90"`
    P95    float64   `json:"p95"`
    P99    float64   `json:"p99"`
    Min    float64   `json:"min"`
    Max    float64   `json:"max"`
}

type CalibrationReport struct {
    TotalSessions      int       `json:"total_sessions"`
    TotalSubagents     int       `json:"total_subagents"`
    ProjectCount       int       `json:"project_count"`
    DateRangeStart     string    `json:"date_range_start"`
    DateRangeEnd       string    `json:"date_range_end"`

    SessionCost        DistStats `json:"session_cost"`
    InputTokens        DistStats `json:"input_tokens"`
    OutputTokens       DistStats `json:"output_tokens"`
    Ratio              DistStats `json:"output_input_ratio"`
    CacheHitRate       DistStats `json:"cache_hit_rate"`
    TokenEfficiency    DistStats `json:"token_efficiency_ratio"`
    SubagentOverhead   DistStats `json:"subagent_overhead_pct"`

    Suggestions        []ThresholdSuggestion `json:"suggestions"`
}

type ThresholdSuggestion struct {
    ConfigKey   string  `json:"config_key"`
    Value       float64 `json:"value"`
    Rationale   string  `json:"rationale"`
}

func ComputeCalibration(events []source.TokenEvent, baselines map[string]Baseline) CalibrationReport
```

### `ComputeCalibration()` algorithm

1. Aggregate events into per-session metrics (reuse `sessionMetrics` from baseline.go, or duplicate the aggregation — simpler to just copy)
2. For each metric, collect all session values into a sorted slice
3. Compute DistStats for each metric
4. Generate suggestions:

```go
func generateSuggestions(cost, input, output, ratio, cache, ter, overhead DistStats) []ThresholdSuggestion {
    var s []ThresholdSuggestion

    // Cost outlier: sigma such that ~2% of sessions are flagged (P98 boundary)
    suggestedSigma := (cost.P98 - cost.Mean) / cost.Std
    if suggestedSigma < 1.5 { suggestedSigma = 2.0 }  // floor
    s = append(s, ThresholdSuggestion{
        ConfigKey: "cost_outlier_sigma",
        Value:     math.Round(suggestedSigma*10) / 10,
        Rationale: fmt.Sprintf("flags ~%.0f%% of sessions as cost outliers", 100-percentileRank(cost, cost.Mean+suggestedSigma*cost.Std)),
    })

    // Input overconsumption: same logic on input tokens
    // Output explosion: same logic on output tokens
    // Low signal percentile: use P5 value (stricter than P10 default)
    s = append(s, ThresholdSuggestion{
        ConfigKey: "low_signal_percentile",
        Value:     5.0,
        Rationale: "stricter than default P10 — only flags bottom 5% of ratios",
    })

    // Subagent overhead: suggest P75 as threshold (current 50% catches too many)
    s = append(s, ThresholdSuggestion{
        ConfigKey: "subagent_overhead_pct",
        Value:     math.Round(overhead.P75),
        Rationale: fmt.Sprintf("P75 of subagent overhead — flags sessions in top quartile"),
    })

    // Fragmentation index: suggest 2x median daily sessions
    // ...

    return s
}
```

### `output/calibrate_text.go` — Text output

```
Your data: 908 main sessions, 1255 subagent sessions across 10 projects
Period: 2026-04-10 to 2026-05-02

Session costs ($):
  n=908  μ=1172.37  σ=5231.44
  P10=0.02  P25=0.08  P50=0.47  P75=1.84  P90=8.21  P95=27.50  P99=2844.49
  min=0.00  max=94510.79

Input tokens:
  n=908  μ=187234  σ=751892
  P10=1234  P25=4872  P50=12451  P75=45231  P90=187234  P95=520123  P99=2928341
  min=0  max=20500000

Output tokens:
  n=908  μ=42156  σ=156234
  P10=518  P25=1523  P50=3187  P75=12451  P90=41234  P95=92156  P99=306231
  min=0  max=3062000

Output/input ratio:
  n=908  μ=0.52
  P10=0.02  P25=0.08  P50=0.31  P75=0.87  P90=1.82  P95=3.45  P99=12.31
  min=0.00  max=45.21

Cache hit rate (%):
  n=908
  P10=52.1  P25=65.3  P50=74.3  P75=87.6  P90=94.2  P95=97.1  P99=99.5
  min=0.0  max=100.0

Token efficiency ratio:
  n=908  μ=0.52
  P10=0.08  P25=0.21  P50=0.47  P75=0.89  P90=1.91  P95=3.12  P99=8.74
  min=0.00  max=25.13

Subagent overhead (%):
  n=226 (sessions with subagents)
  P10=8.3  P25=32.1  P50=72.1  P75=84.3  P90=92.5  P95=96.2  P99=100.0
  min=0.3  max=100.0

Suggested thresholds (for .burnwatch.toml):
  [thresholds]
  cost_outlier_sigma = 2.5
  input_overconsumption_sigma = 2.5
  output_explosion_sigma = 2.5
  low_signal_percentile = 5.0
  token_efficiency_percentile = 5.0
  subagent_overhead_pct = 75.0
  fragmentation_index_threshold = 3.0

  [signals]
  ... (all enabled by default)
```

### `output/calibrate_json.go` — JSON output

Serialize `CalibrationReport` as JSON. Follow existing `output/json.go` conventions.

### `cmd/root.go` — New flag

```go
flag.BoolVar(&flags.Calibrate, "calibrate", false, "Show data distribution and suggest thresholds")

// In Execute():
if flags.Calibrate {
    // ... load events, compute baselines (same as normal flow) ...
    report := analyze.ComputeCalibration(allEvents, baselines)
    if flags.JSON {
        output.WriteCalibrationJSON(os.Stdout, report)
    } else {
        output.WriteCalibrationText(os.Stdout, report)
    }
    return
}
```

---

## Test Requirements

1. **`analyze/calibrate_test.go`**:
   - 5 sessions with known values → DistStats computed correctly
   - P50 of [1,2,3,4,5] = 3.0
   - Single session → P10=P50=P90=value, std=0
   - No sessions → all DistStats zero/empty
   - Suggestions generated from stats have reasonable values (sigma > 0)
   - Date range: min/max timestamps extracted correctly
   - Total sessions / subagent count correct

2. **`output/calibrate_test.go`**:
   - Text output contains all expected sections (cost, input, output, ratio, cache, ter, overhead, suggestions)
   - Text output formats numbers correctly (K/M suffixes, 2 decimal places for $)
   - JSON output round-trips: marshal → unmarshal → fields match
   - Golden file for calibration text output

3. Coverage target: >=90% on new code

---

## Approach

1. Define `CalibrationReport`, `DistStats`, `ThresholdSuggestion` in `analyze/calibrate.go`
2. Implement `ComputeCalibration()`
3. Write calibration tests (RED → GREEN)
4. Implement text output in `output/calibrate_text.go`
5. Implement JSON output in `output/calibrate_json.go`
6. Wire `--calibrate` flag in `cmd/root.go`
7. Golden file for calibration output
8. Full test suite + lint

---

## Exit Criteria

- [ ] Pull latest main
- [ ] Create feature branch from main: `git checkout -b pr15-calibrate`
- [ ] Lint passes (zero warnings) — `golangci-lint run`
- [ ] Build compiles cleanly — `go build -o burnwatch .`
- [ ] Tests pass with >=90% coverage on new code — `go test ./... -cover`
- [ ] Self-review: run through [docs/code-review.md](../code-review.md)
- [ ] Document learnings in `docs/learnings.md`
- [ ] Commit: `feat: calibration mode — distribution analysis + threshold suggestions`
- [ ] Push to branch `pr15-calibrate`
- [ ] Open pull request
- [ ] Perform code review
- [ ] Update `docs/plans/progress.md` to reflect merge
- [ ] Merge to main
- [ ] Delete feature branch after merge

---

## Notes

- `DistStats` reuses the existing `percentile()` and `stddev()` functions from `baseline.go`. Extract them to a shared `analyze/stats.go` or keep them in `baseline.go` and reference from `calibrate.go` (same package, no import needed).
- Session costs for calibration should use the fixed PR11 pricing to be accurate.
- Subagent overhead stats only include sessions that have subagents (n=226 in the example).
- `formatTokens()` from `output/text.go` is reused for token display in calibration text.
- The `--calibrate` mode does NOT load config (no self-referential loop). It computes stats from raw data and prints suggestions.
- P98 is used for sigma suggestions because sigma-based outlier detection with sigma=2.0 flags ~2.5% of normally distributed data. P98 is close to mean+2σ.
