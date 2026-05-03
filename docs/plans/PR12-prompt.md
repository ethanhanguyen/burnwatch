# PR12: Token Baselines — Input/Output Mean, Std, Percentiles

> **Workflow:** Follow `docs/PR-template.md`. Review `AGENTS.md` behavioral guidelines before implementing.

## Objective

Extend `Baseline` to compute statistical baselines for input tokens, output tokens, and token efficiency ratio (TER). This is a data plumbing PR — no new heuristics, no user-visible changes. It makes token-level statistics available for PR13.

## Success Criteria

- [ ] `Baseline` struct has: `InputMean`, `InputStd`, `InputP50`, `InputP90`, `OutputMean`, `OutputStd`, `OutputP50`, `OutputP90`, `TERP10`
- [ ] All new fields computed in `buildBaseline()` from existing `sessionMetrics.inputSum`/`.outputSum`
- [ ] TER = `(outputSum + cacheRead) / (inputSum + cacheWrite)` per session
- [ ] All existing tests pass unchanged (new fields are additive only)
- [ ] New fields round-tripped correctly in JSON output (visible in `--json` mode)
- [ ] No user-facing output changes in text mode (PR13 adds the heuristics)

## Dependencies

- **Must merge first:** PR11 (dynamic pricing — ensures cost baselines are accurate before adding token baselines)
- **External dependencies:** None
- **Can be parallel with:** None (needs PR11)
- **Breaking changes / Migrations needed:** Golden files — JSON output gains new baseline fields

---

## Pre-flight

- [ ] Pull latest main: `git fetch origin && git checkout main && git pull origin main`
- [ ] Create feature branch: `git checkout -b pr12-token-baselines`
- [ ] Verify build environment works on clean main

## Files

| File | Purpose | Notes |
|------|---------|-------|
| `analyze/baseline.go` | Add new fields to Baseline, compute in buildBaseline | Modify |
| `analyze/baseline_test.go` | Test new field computation | Modify |
| `output/json.go` | Serialize new baseline fields in JSON output | Modify |
| `output/json_test.go` | Verify JSON round-trip | Modify |
| `testdata/expected_report.json` | Regenerate | Update |

---

## Implementation

### `analyze/baseline.go` — Baseline struct extension

```go
type Baseline struct {
    Project      string
    Harness      string
    SessionCount int

    // Existing cost-based
    CostMean     float64
    CostStd      float64

    // Existing ratio-based
    RatioMean    float64
    RatioP10     float64
    RatioP50     float64
    RatioP90     float64

    // Existing cache-based
    CacheP10     float64
    CacheP50     float64

    // NEW: token-based
    InputMean    float64   // mean input tokens per session
    InputStd     float64   // std dev of input tokens
    InputP50     float64   // median input tokens
    InputP90     float64   // 90th percentile input tokens
    OutputMean   float64   // mean output tokens per session
    OutputStd    float64   // std dev of output tokens
    OutputP50    float64   // median output tokens
    OutputP90    float64   // 90th percentile output tokens

    // NEW: token efficiency
    TERP10       float64   // 10th percentile of token efficiency ratio

    // Raw data (for calibration, etc.)
    SessionCosts  []float64
    Ratios        []float64
    CacheRates    []float64
    InputTokens   []float64   // NEW
    OutputTokens  []float64   // NEW
    TERs          []float64   // NEW
}
```

### Token Efficiency Ratio (TER) computation

In `aggregateMetrics()`, add after the existing ratio/cacheRate computation:

```go
// TER = (output + cache_read) / (input + cache_write)
// Measures useful tokens produced per token consumed
// cache_read is "free" tokens (didn't need re-sending)
// cache_write is "waste" (evicted previous cache)
if m.inputSum+m.cacheWrite > 0 {
    m.ter = float64(m.outputSum+m.cacheRead) / float64(m.inputSum+m.cacheWrite)
}
```

Add `ter float64` to `sessionMetrics` struct.

### `buildBaseline()` — new computation

After existing cost arrays and ratio arrays, add:

```go
// Input token statistics
b.InputTokens = make([]float64, n)
for i, m := range metrics {
    b.InputTokens[i] = float64(m.inputSum)
}
sort.Float64s(b.InputTokens)
b.InputMean = mean(b.InputTokens)
if n > 1 {
    b.InputStd = stddev(b.InputTokens, b.InputMean)
}
b.InputP50 = percentile(b.InputTokens, 50)
b.InputP90 = percentile(b.InputTokens, 90)

// Output token statistics
b.OutputTokens = make([]float64, n)
for i, m := range metrics {
    b.OutputTokens[i] = float64(m.outputSum)
}
sort.Float64s(b.OutputTokens)
b.OutputMean = mean(b.OutputTokens)
if n > 1 {
    b.OutputStd = stddev(b.OutputTokens, b.OutputMean)
}
b.OutputP50 = percentile(b.OutputTokens, 50)
b.OutputP90 = percentile(b.OutputTokens, 90)

// TER statistics
b.TERs = make([]float64, n)
for i, m := range metrics {
    b.TERs[i] = m.ter
}
sort.Float64s(b.TERs)
b.TERP10 = percentile(b.TERs, 10)
```

**Constraints:**
- Float64 for token counts (even though they're int64 at event level) to avoid overflow in stats (sum of many sessions could exceed int64 for large datasets)
- `TER` can be >1.0 (output + cacheRead can exceed input + cacheWrite) — this is fine, means the session was very efficient
- n > 1 guard for stddev (same pattern as CostStd)

**Error handling:**
- If all input sums are 0 (no tokens consumed), InputMean = 0, InputStd = 0 (not NaN)

### `output/json.go` — serialize new fields

The JSON output under `"baselines"` already serializes the `Baseline` struct. Since this is Go JSON marshaling with struct tags, ensure new fields are included. Add `json:"..."` tags:

```go
InputMean    float64 `json:"input_mean"`
InputStd     float64 `json:"input_std"`
InputP50     float64 `json:"input_p50"`
InputP90     float64 `json:"input_p90"`
OutputMean   float64 `json:"output_mean"`
OutputStd    float64 `json:"output_std"`
OutputP50    float64 `json:"output_p50"`
OutputP90    float64 `json:"output_p90"`
TERP10       float64 `json:"ter_p10"`
InputTokens  []float64 `json:"input_tokens,omitempty"`
OutputTokens []float64 `json:"output_tokens,omitempty"`
TERs         []float64 `json:"ters,omitempty"`
```

---

## Test Requirements

1. **`analyze/baseline_test.go`**:
   - Compute baselines with sessions having varying input/output token counts
   - Verify InputMean, InputStd, InputP50, InputP90 correct
   - Verify OutputMean, OutputStd, OutputP50, OutputP90 correct
   - Verify TER computation: session with 100K in, 50K out, 10K cacheRead, 5K cacheWrite → TER = 60000/105000 ≈ 0.571
   - Edge case: all sessions have 0 input tokens → all token fields = 0 (no NaN)
   - Edge case: n=1 → std should be 0 (no division by zero)
   - All existing baselines tests continue to pass
   - Table-driven: test with pre-computed expected values for a 5-session dataset

2. **`output/json_test.go`**:
   - JSON output includes new baseline fields
   - Round-trip: parse JSON, verify fields present and correct
   - Golden file update

3. Coverage target: >=90% on new code

---

## Approach

1. Extend `sessionMetrics` with `ter float64`
2. Compute `ter` in `aggregateMetrics()`
3. Extend `Baseline` struct with new fields + json tags
4. Compute new fields in `buildBaseline()`
5. Write baseline tests (verify computed stats against known dataset)
6. Regenerate JSON golden file
7. Verify existing tests pass unchanged
8. Full test suite + lint

---

## Exit Criteria

- [ ] Pull latest main
- [ ] Create feature branch from main: `git checkout -b pr12-token-baselines`
- [ ] Lint passes (zero warnings) — `golangci-lint run`
- [ ] Build compiles cleanly — `go build -o burnwatch .`
- [ ] Tests pass with >=90% coverage on new code — `go test ./... -cover`
- [ ] Self-review: run through [docs/code-review.md](../code-review.md)
- [ ] Document learnings in `docs/learnings.md`
- [ ] Commit: `feat: token baselines — input/output mean, std, percentiles, TER`
- [ ] Push to branch `pr12-token-baselines`
- [ ] Open pull request
- [ ] Perform code review
- [ ] Update `docs/plans/progress.md` to reflect merge
- [ ] Merge to main
- [ ] Delete feature branch after merge

---

## Notes

- `TER` uses `cacheRead`/`cacheWrite` which are already in `sessionMetrics`. No new data plumbing needed.
- `InputTokens`/`OutputTokens`/`TERs` slices are raw data arrays (like existing `SessionCosts`/`Ratios`/`CacheRates`). They use `omitempty` in JSON to avoid bloating output.
- `percentile()` and `stddev()` already exist — reuse them, don't reimplement.
- Float64 conversion from int64 is safe up to 2^53 (~9 quadrillion) which is ~9 trillion tokens — well beyond any realistic session.
