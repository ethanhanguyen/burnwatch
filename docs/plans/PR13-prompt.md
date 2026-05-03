# PR13: Token-Based Heuristics — Input Overconsumption, Output Explosion, TER, Fragmentation Index

> **Workflow:** Follow `docs/PR-template.md`. Review `AGENTS.md` behavioral guidelines before implementing.

## Objective

Add four new waste detection heuristics that use absolute token counts (not cost or ratio) to catch waste patterns invisible to v1 heuristics. Also replace the current binary session churn (H5) with a weighted fragmentation index (H9).

## Success Criteria

- [ ] **H6 – Input overconsumption:** sessions where `inputSum > bl.InputMean + sigma*bl.InputStd` are flagged as HIGH
- [ ] **H7 – Output explosion:** sessions where `outputSum > bl.OutputMean + sigma*bl.OutputStd` are flagged as MEDIUM
- [ ] **H8 – Token efficiency:** sessions where `TER < bl.TERP10` are flagged as LOW
- [ ] **H9 – Fragmentation index:** grouped by project/day, when `N_sessions * (1 − mean_ratio) > threshold`, each session in the group is flagged as MEDIUM
- [ ] Each heuristic has a config toggle in `SignalToggles` and a configurable threshold
- [ ] H9 replaces H5 (session churn). H5 is removed from the codebase.
- [ ] Each heuristic follows the existing `check*()` pattern: takes sessionAgg + baseline → returns *WasteSignal or nil
- [ ] H6/H7/H8 use project-specific baselines (like H1). H9 also uses project-specific baselines.
- [ ] All new heuristics output model/token info in text format (follow C1 pattern from PR10)

## Dependencies

- **Must merge first:** PR12 (token baselines)
- **External dependencies:** None
- **Can be parallel with:** None (needs PR12)
- **Breaking changes / Migrations needed:** H5 removed. Golden files updated. `SignalToggles.SessionChurn` renamed to `FragmentationIndex`.

---

## Pre-flight

- [ ] Pull latest main: `git fetch origin && git checkout main && git pull origin main`
- [ ] Create feature branch: `git checkout -b pr13-token-heuristics`
- [ ] Verify build environment works on clean main

## Files

| File | Purpose | Notes |
|------|---------|-------|
| `analyze/waste.go` | Add H6–H9 check functions, remove H5, update SignalToggles, update DetectWaste | Modify |
| `analyze/waste_test.go` | Tests for new heuristics, edge cases | Modify |
| `output/text.go` | Display new signal types, remove H5 display code | Modify |
| `output/text_test.go` | Golden file updates | Modify |
| `output/json.go` | New reason values in JSON output | Modify |
| `cmd/root.go` | Replace `--no-churn` with `--no-fragmentation-index`, add `--no-input-overconsumption`, `--no-output-explosion`, `--no-token-efficiency` | Modify |
| `config/config.go` | Add toggle fields for new heuristics | Modify |
| `config/config_test.go` | Default values test | Modify |
| `testdata/expected_report.txt` | Regenerate | Update |
| `testdata/expected_report.json` | Regenerate | Update |

---

## Implementation

### H6 — Input Overconsumption (`checkInputOverconsumption`)

```
Trigger: a.inputSum > bl.InputMean + sigma*bl.InputStd
Requires: bl.SessionCount >= 2, bl.InputStd > 0
Severity: "high"
Reason:   "input_overconsumption"
Detail:   "%.1fK input tokens (μ=%.1fK, σ=%.1fK)"
Metric:   float64(a.inputSum) / 1000
Threshold: (bl.InputMean + sigma*bl.InputStd) / 1000
Recommendation: "Session consumed %dx more input context than project peers. Check for context bloat, repeated file reads, or tool-call loops."
Savings:    a.cost * (1 - bl.InputMean/float64(a.inputSum))  // proportional to excess
```

**Edge cases:**
- `bl.InputStd == 0` → skip (all sessions have same input, no outlier possible)
- `a.inputSum == 0` → skip (no input tokens at all)
- Display token counts in human-readable format: `<1K`, `12.3K`, `1.2M`, etc.

### H7 — Output Explosion (`checkOutputExplosion`)

```
Trigger: a.outputSum > bl.OutputMean + sigma*bl.OutputStd
Requires: bl.SessionCount >= 2, bl.OutputStd > 0
Severity: "medium"
Reason:   "output_explosion"
Detail:   "%.1fK output tokens (μ=%.1fK, σ=%.1fK)"
Metric:   float64(a.outputSum) / 1000
Threshold: (bl.OutputMean + sigma*bl.OutputStd) / 1000
Recommendation: "Session generated %dx more output than project peers. Check for runaway generation loops or repeated corrections."
Savings:    a.cost * (1 - bl.OutputMean/float64(a.outputSum))
```

**Why MEDIUM not HIGH:** High output is often legitimate (generating a large file). Input overconsumption is almost always waste. Output explosion needs human judgment.

**Edge cases:**
- Same guards as H6 (std=0, sum=0)
- Sessions with high output and high ratio (>1.0) — these are "productive" sessions (generating code). Consider skipping if ratio > 2.0 (configurable).

### H8 — Token Efficiency Ratio (`checkTokenEfficiency`)

```
Trigger: a.ter < bl.TERP10
Requires: bl.SessionCount >= 2, a.inputSum+a.cacheWrite > 0
Severity: "low"
Reason:   "low_token_efficiency"
Detail:   "TER = %.2f (P10 = %.2f)"
Metric:   a.ter
Threshold: bl.TERP10
Recommendation: "Session produced low useful output per token consumed. Consider consolidating or using a cheaper model."
Savings:    a.cost * 0.2  // rough estimate: 20% of cost could be saved with better prompting
```

**TER formula:** `(outputSum + cacheRead) / (inputSum + cacheWrite)`

**Why LOW not MEDIUM:** Low TER is often legitimate (exploring a codebase, reading docs). It's a signal to investigate, not an urgent fix.

### H9 — Fragmentation Index (`checkFragmentationIndex`)

Replaces H5. Instead of binary "all sessions below mean ratio" check, use weighted index.

```
Group:  by (project, day) — same as H5
Trigger: float64(len(sessions)) * (1 - meanRatio) > threshold
Requires: len(sessions) >= minSessions (default 3, configurable)
Severity: "medium"
Reason:   "fragmentation_index"
Detail:   "%d sessions below mean ratio (index=%.1f)" on the group, then flag each session
For each session in the group:
  Metric:   float64(len(sessions)) * (1 - meanRatio)  // group index
  Threshold: threshold (default 3.0, configurable)
Recommendation: "Consolidate fragmented sessions — fewer, longer sessions cache better."
Savings:    per-session: a.cost * 0.7  // fragmentation typically wastes ~70% due to lost cache
```

**Why weighted index is better than binary:**
- Binary: 3 sessions at mean-0.01 → flagged. Index: 3 * 0.01 = 0.03 → not flagged.
- Binary: 20 sessions at mean-0.5 → flagged. Index: 20 * 0.5 = 10.0 → strongly flagged.
- The index captures both "how many sessions" AND "how low quality" — more honest than the binary check.

**De-duplication:** Use `seen` map (same pattern as current H5) to avoid flagging the same session if it appears in multiple day groups.

### DetectWaste integration

```go
func DetectWaste(events []source.TokenEvent, baselines map[string]Baseline,
    costSigma float64, inputSigma float64, outputSigma float64, terPercentile float64,
    fragmentationThreshold int, fragmentationMinSessions int,
    toggles SignalToggles) []WasteSignal {

    // ... existing aggregation loop ...

    // Existing heuristics (unchanged)
    if toggles.CostOutlier        { signals = append(signals, checkCostOutlier(...)) }
    if toggles.LowSignal          { signals = append(signals, checkLowSignal(...)) }
    if toggles.SubagentOverhead   { signals = append(signals, checkSubagentOverhead(...)) }
    if toggles.CacheUnderutilized { signals = append(signals, checkCacheUnderutilized(...)) }

    // NEW heuristics
    if toggles.InputOverconsumption { signals = append(signals, checkInputOverconsumption(...)) }
    if toggles.OutputExplosion     { signals = append(signals, checkOutputExplosion(...)) }
    if toggles.TokenEfficiency     { signals = append(signals, checkTokenEfficiency(...)) }
    if toggles.FragmentationIndex  { signals = append(signals, checkFragmentationIndex(...)) }

    // sort + return
}
```

### SignalToggles update

```go
type SignalToggles struct {
    CostOutlier          bool
    LowSignal            bool
    SubagentOverhead     bool
    CacheUnderutilized   bool
    FragmentationIndex   bool   // renamed from SessionChurn
    InputOverconsumption bool   // NEW
    OutputExplosion      bool   // NEW
    TokenEfficiency      bool   // NEW
}
```

### H5 removal

Remove `checkSessionChurn()` function entirely. The name `SessionChurn` is removed. The functionality is replaced by `checkFragmentationIndex()`.

Renaming `SessionChurn` → `FragmentationIndex` in:
- `SignalToggles` struct
- `config.Signals` struct
- `cmd/root.go` flag: `--no-churn` → `--no-fragmentation-index`
- `config/config.go` TOML key: `session_churn` → `session_churn` (keep TOML key backward compatible, rename Go field)
- Actually: keep TOML key `session_churn` for backward compatibility but map to `FragmentationIndex` Go field

### Output format

Follow existing signal block format:

```
  HIGH ses_abc123 (project): $4.95 — 5.2x project baseline (μ = 12.3K input tokens)
    Model: deepseek/deepseek-v4-pro, 1.2M in / 45.3K out
    → Session consumed 5.2x more input context than project peers. Potential savings: $3.34

  MEDIUM ses_def456 (project): $1.23 — 3.8x project baseline (μ = 3.1K output tokens)
    Model: claude-sonnet-4-6, 12.1K in / 118.4K out
    → Check for runaway generation loops. Potential savings: $0.87

  LOW  ses_ghi789 (project): $0.45 — TER = 0.03 (P10 = 0.08)
    Model: moonshotai/kimi-k2.6, 850.1K in / 2.1K out
    → Low useful output per token consumed. Potential savings: $0.09

  MEDIUM ses_jkl012 (project): $2.10 — 12 sessions below mean ratio (index = 5.2)
    → Consolidate fragmented sessions. Potential savings: $1.47
```

---

## Test Requirements

1. **`analyze/waste_test.go` — New heuristic tests**:
   - H6: session with 500K input, project μ=100K, σ=50K, sigma=2.0 → flagged (500K > 100K+100K=200K)
   - H6: session with 150K input, same baseline → not flagged
   - H6: InputStd=0 → not flagged (no variance)
   - H6: n=1 → not flagged (need ≥2 sessions)
   - H6: inputSum=0 → not flagged
   - H7: session with 200K output, project μ=40K, σ=30K, sigma=2.0 → flagged (200K > 40K+60K=100K)
   - H7: outputSum=0 → not flagged
   - H8: TER=0.02, P10=0.05 → flagged
   - H8: TER=0.10, P10=0.05 → not flagged
   - H8: zero cache activity → TER computed correctly (outputSum/inputSum)
   - H9: 5 sessions, mean ratio=0.1, N=5, index=4.5, threshold=3.0 → flagged
   - H9: 2 sessions, mean ratio=0.1, index=1.8, minSessions=3 → not flagged (below min)
   - H9: 3 sessions, mean ratio=0.9, index=0.3, threshold=3.0 → not flagged (low fragmentation)
   - H9: same session in two different day groups → de-duplicated correctly

2. **`analyze/waste_test.go` — Existing tests**:
   - All existing tests pass after H5 removal
   - H5 references removed from test helpers

3. **`output/text_test.go`**:
   - New signal types displayed correctly in text format
   - Model/token info present on H6/H7/H8 signals
   - Golden file update

4. Coverage target: >=90% on new code

---

## Approach

1. Add `checkInputOverconsumption`, `checkOutputExplosion`, `checkTokenEfficiency`, `checkFragmentationIndex` functions to `waste.go`
2. Write tests for each (RED)
3. Update `SignalToggles` (GREEN)
4. Update `DetectWaste` signature and body (GREEN)
5. Update `cmd/root.go` with new flags (GREEN)
6. Update `output/text.go` for new signal display
7. Update `config/config.go` for new toggle defaults
8. Regenerate golden files
9. Full test suite + lint
10. REFACTOR: verify H5 is fully removed (grep for `SessionChurn`, `checkSessionChurn`)

---

## Exit Criteria

- [ ] Pull latest main
- [ ] Create feature branch from main: `git checkout -b pr13-token-heuristics`
- [ ] Lint passes (zero warnings) — `golangci-lint run`
- [ ] Build compiles cleanly — `go build -o burnwatch .`
- [ ] Tests pass with >=90% coverage on new code — `go test ./... -cover`
- [ ] Self-review: run through [docs/code-review.md](../code-review.md)
- [ ] Document learnings in `docs/learnings.md`
- [ ] Commit: `feat: token-based heuristics — input/output/TER/fragmentation index`
- [ ] Push to branch `pr13-token-heuristics`
- [ ] Open pull request
- [ ] Perform code review
- [ ] Update `docs/plans/progress.md` to reflect merge
- [ ] Merge to main
- [ ] Delete feature branch after merge

---

## Notes

- H6 uses `sigma` — same concept as H1 but on input tokens. The config key will be `input_overconsumption_sigma` (added in PR14). For this PR, pass it as a parameter to `DetectWaste`.
- Savings estimates for H6/H7 are `cost * excess_ratio` — a rough proportional estimate. For H8, use 20% of cost (arbitrary but reasonable). For H9, use 70% (cache loss from fragmentation).
- All token counts in output use `formatTokens()` helper (already exists in `output/text.go`).
- H9 signal lists ALL sessions in the group, each with the group's index as the metric. This follows the existing H5 pattern.
- TER is always computed (even without cache activity) as `outputSum / inputSum` fallback.
