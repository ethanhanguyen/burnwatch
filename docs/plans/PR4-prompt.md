# PR4: Analysis Engine

> **Workflow:** Follow `docs/plans/PR-template.md`. Review `AGENTS.md` behavioral guidelines before implementing.

## Objective

Build the waste detection pipeline. Given a stream of `TokenEvent` values, compute per-project baselines, flag waste signals using statistical outlier detection, build subagent cost trees, and generate actionable recommendations.

## Files to create

| File | Purpose |
|------|---------|
| `analyze/baseline.go` | Compute µ, σ, percentiles from events |
| `analyze/baseline_test.go` | Tests on constructed event sets |
| `analyze/waste.go` | 5 waste heuristics with outlier detection |
| `analyze/waste_test.go` | Test each heuristic against known data |
| `analyze/subagent.go` | Subagent cost tree + overhead percentage |
| `analyze/subagent_test.go` | Test tree construction and cost attribution |
| `analyze/recommend.go` | Map waste signals → human-readable recommendations |
| `analyze/recommend_test.go` | Test recommendation text generation |

## Dependencies

PR1 must be merged. PR4 can be built in parallel with PR2 and PR3. Tests use constructed `TokenEvent` slices — no real data needed until PR5.

## `analyze/baseline.go`

```go
type Baseline struct {
    Project    string
    Harness    string
    SessionCount int
    CostMean   float64   // μ
    CostStd    float64   // σ
    RatioP10   float64   // 10th percentile output/input ratio
    RatioP50   float64   // median output/input ratio
    RatioP90   float64   // 90th percentile
    CacheP10   float64   // 10th percentile cache hit rate
    CacheP50   float64   // median cache hit rate
    SessionCosts []float64  // sorted, for percentile computation
    Ratios       []float64  // sorted, output/input per session
    CacheRates   []float64  // sorted, cache_read/(cache_read+cache_write) per event
}

func ComputeBaselines(events []TokenEvent) map[string]Baseline
```

### Algorithm

1. Group events by `Project` + `Harness` key.
2. For each group, aggregate per-session metrics:
   - `sessionCost = sum(event.CostUSD)`
   - `totalInput = sum(event.InputTokens)`, `totalOutput = sum(event.OutputTokens)`
   - `ratio = totalOutput / totalInput` (0 if no input)
   - `cacheHitRate = sum(event.CacheRead) / (sum(event.CacheRead) + sum(event.CacheWrite))` (0 if no cache activity)
3. Compute µ and σ for session costs.
4. Compute percentiles for ratios and cache rates (P10, P50, P90).
5. Return map keyed by `"project:harness"`.

### Edge cases
- Single session → σ = 0, percentiles = the single value.
- All identical values → σ = 0 (no outliers will fire).
- Empty input → empty map, no panic.
- Zero total input → ratio = 0 (not NaN).
- Negative tokens (shouldn't happen, but guard) → treat as 0.

## `analyze/waste.go`

```go
type WasteSignal struct {
    SessionID string
    Project   string
    Severity  string   // "high", "medium", "low"
    Reason    string   // machine-readable: "cost_outlier", "low_signal", etc.
    Detail    string   // human-readable explanation
    Metric    float64  // the value that triggered the signal
    Threshold float64  // the threshold that was exceeded
}

func DetectWaste(events []TokenEvent, baselines map[string]Baseline) []WasteSignal
```

### 5 Heuristics

| # | Name | Condition | Severity |
|---|------|-----------|----------|
| H1 | Cost outlier | `session_cost > μ + 2σ` | high |
| H2 | Low signal | `output/input_ratio < P10` of all ratios | medium |
| H3 | Subagent overhead | `subagent_cost > 50%` of total session cost | medium |
| H4 | Cache underutilized | `cache_hit_rate < P10` | low |
| H5 | Session churn | ≥3 sessions same project/day, all below μ_ratio | medium |

### H3: Subagent overhead

Uses data from `analyze/subagent.go`:
```go
type SubagentTree struct {
    SessionID    string
    Subagents    []SubagentNode
    TotalCost    float64
    SubagentCost float64
    OverheadPct  float64
}

type SubagentNode struct {
    SessionID  string
    AgentType  string
    Cost       float64
    Children   []SubagentNode
}

func BuildSubagentTree(events []TokenEvent) []SubagentTree
```

Tree construction:
1. Group events by `SessionID`.
2. For each session, separate top-level events (`IsSubagent == false`) from subagent events (`IsSubagent == true`).
3. Subagent events link to parent via `ParentSessionID`.
4. Sum costs at each level.
5. Compute `OverheadPct = SubagentCost / TotalCost * 100`.

### H5: Session churn

1. Group sessions by project + day.
2. If a project has ≥3 sessions in one day and ALL have `output/input < μ_ratio` → flag.

## `analyze/recommend.go`

```go
type Recommendation struct {
    Signal       WasteSignal
    Action       string    // one-line prescription
    Detail       string    // expanded explanation
    SavingsEst   float64   // projected savings if fixed
}

func GenerateRecommendations(signals []WasteSignal, baselines map[string]Baseline) []Recommendation
```

### Signal → Recommendation mapping

| Signal | Action | Savings estimate |
|--------|--------|-----------------|
| Cost outlier | "Investigate session for unnecessary loops or re-prompts" | `cost - μ` |
| Low signal | "Consider whether this task needed full agent interaction" | `session_cost * 0.5` |
| Subagent overhead | "Evaluate whether subagent delegation was necessary vs inline" | `subagentCost * 0.7` |
| Cache underutilized | "Consider CLAUDE.md / skills optimization for better caching" | `session_cost * 0.2` |
| Session churn | "Consolidate fragmented sessions — fewer, longer sessions cache better" | Sum of affected session costs |

## Test requirements

### `baseline_test.go`
- 2 projects, 3 sessions each, known costs → verify µ, σ, percentiles correct.
- Single session → σ=0, percentiles match.
- All identical → σ=0.
- Empty input → empty map.
- Session with zero input → ratio=0.

### `waste_test.go`
- Construct events where one session costs 5x others → H1 fires.
- Construct events with low output ratio → H2 fires.
- Construct events with subagent cost >50% → H3 fires.
- Construct events with zero cache reads → H4 fires.
- Construct 3+ sessions in one project with all low ratio → H5 fires.
- Edge: zero events → no signals.
- Edge: all normal (no outliers) → no signals.

### `subagent_test.go`
- Construct parent + 2 child subagents with known costs → tree is correct.
- Session with no subagents → tree has empty children, overhead 0%.
- Multiple levels (parent → child → grandchild) → correct nesting.
- Subagent with unknown parent → still included, parent unknown.

### `recommend_test.go`
- Each signal type → correct recommendation text generated.
- Multiple signals → multiple recommendations.
- No signals → empty recommendations list.

**Coverage target**: ≥90% on each `.go` file.

## Approach: TDD

1. Write baseline_test.go (RED).
2. Write baseline.go (GREEN).
3. Repeat for waste.go, subagent.go, recommend.go.
4. Run full coverage check, fill gaps.

## Exit criteria

- [ ] Pull latest main
- [ ] Create feature branch from main
- [ ] `go test ./analyze/... -cover` passes with ≥90% coverage on all files
- [ ] `go vet ./...` zero warnings
- [ ] `golangci-lint run` zero issues
- [ ] Self-review: follow behavioral guidelines in `AGENTS.md`
- [ ] Commit: `feat: add waste detection engine with statistical baselines and recommendations`
- [ ] Push to branch `pr4-analysis-engine`
- [ ] Open pull request
- [ ] Perform code review
- [ ] Merge to main
- [ ] Delete feature branch after merge
