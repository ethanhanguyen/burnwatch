# PR14: Config-Wired Thresholds — Complete Config → Heuristic Wiring

> **Workflow:** Follow `docs/PR-template.md`. Review `AGENTS.md` behavioral guidelines before implementing.

## Objective

Wire all remaining hardcoded heuristic thresholds to the config system. Replace every hardcoded constant in waste detection with a config value loaded from `.burnwatch.toml`. Complete the work PR7 started but only partially finished.

## Success Criteria

- [ ] `thresholds.low_signal_percentile` wired to `Baseline.RatioP10` computation (was hardcoded `10`)
- [ ] `thresholds.cache_percentile` wired to `Baseline.CacheP10` computation (was hardcoded `10`)
- [ ] `thresholds.subagent_overhead_pct` wired to `checkSubagentOverhead()` threshold (was hardcoded `50`)
- [ ] `thresholds.churn_min_sessions` wired (was hardcoded `3`)
- [ ] `thresholds.churn_threshold` wired (was hardcoded `2`)
- [ ] `thresholds.input_overconsumption_sigma` added and wired (default `2.0`)
- [ ] `thresholds.output_explosion_sigma` added and wired (default `2.0`)
- [ ] `thresholds.token_efficiency_percentile` added and wired (default `10.0`)
- [ ] `thresholds.fragmentation_index_threshold` added and wired (default `3.0`)
- [ ] `signals.input_overconsumption`, `signals.output_explosion`, `signals.token_efficiency`, `signals.fragmentation_index` added and wired
- [ ] `signals.session_churn` TOML key kept for backward compatibility, maps to `FragmentationIndex`
- [ ] CLI flags override config values (existing priority: flag > config > defaults)
- [ ] `Validate()` checks all new fields (sigma > 0, percentile in (0,100), etc.)
- [ ] `--print-config` outputs all new fields

## Dependencies

- **Must merge first:** PR13 (token heuristics — need the heuristics defined before wiring their configs)
- **External dependencies:** None
- **Can be parallel with:** PR15 (different files, no shared state)
- **Breaking changes / Migrations needed:** Config file schema extended. Old configs still valid (new fields get defaults).

---

## Pre-flight

- [ ] Pull latest main: `git fetch origin && git checkout main && git pull origin main`
- [ ] Create feature branch: `git checkout -b pr14-config-wiring`
- [ ] Verify build environment works on clean main

## Files

| File | Purpose | Notes |
|------|---------|-------|
| `config/config.go` | Extend Thresholds + Signals structs, update Validate, Defaults | Modify |
| `config/config_test.go` | Test new defaults, validation, TOML parsing | Modify |
| `analyze/waste.go` | Remove hardcoded constants, accept Config as parameter | Modify |
| `analyze/baseline.go` | Accept percentile values from config instead of hardcoded 10 | Modify |
| `analyze/waste_test.go` | Update test helpers to pass config | Modify |
| `analyze/baseline_test.go` | Pass config to ComputeBaselines | Modify |
| `cmd/root.go` | Wire config values into DetectWaste and ComputeBaselines, add new CLI flags | Modify |
| `testdata/expected_report.txt` | No changes (defaults unchanged) | Verify |

---

## Implementation

### `config/config.go` — Extended structs

```go
type Config struct {
    Thresholds Thresholds
    Signals    Signals
    Filters    Filters
    Output     Output
}

type Thresholds struct {
    // Existing
    CostOutlierSigma    float64 `toml:"cost_outlier_sigma"`

    // New — ratio/cache percentiles (configurable, not hardcoded)
    LowSignalPercentile float64 `toml:"low_signal_percentile"`
    CachePercentile     float64 `toml:"cache_percentile"`

    // New — subagent
    SubagentOverheadPct float64 `toml:"subagent_overhead_pct"`

    // New — churn → fragmentation
    ChurnMinSessions         int     `toml:"churn_min_sessions"`
    ChurnThreshold           float64 `toml:"churn_threshold"`
    FragmentationIndexThreshold float64 `toml:"fragmentation_index_threshold"`

    // New — token heuristics (from PR13)
    InputOverconsumptionSigma float64 `toml:"input_overconsumption_sigma"`
    OutputExplosionSigma      float64 `toml:"output_explosion_sigma"`
    TokenEfficiencyPercentile float64 `toml:"token_efficiency_percentile"`
}

type Signals struct {
    // Existing
    CostOutlier        bool `toml:"cost_outlier"`
    LowSignal          bool `toml:"low_signal"`
    SubagentOverhead   bool `toml:"subagent_overhead"`
    CacheUnderutilized bool `toml:"cache_underutilized"`
    SessionChurn       bool `toml:"session_churn"`        // backward compat → maps to FragmentationIndex

    // New — token heuristics
    InputOverconsumption bool `toml:"input_overconsumption"`
    OutputExplosion      bool `toml:"output_explosion"`
    TokenEfficiency      bool `toml:"token_efficiency"`
    FragmentationIndex   bool `toml:"fragmentation_index"`
}
```

**Backward compatibility:** `session_churn` TOML key maps to `FragmentationIndex` Go field. If the user has `session_churn = false` in their config, the fragmentation index heuristic is disabled.

```go
// In Defaults() or Load(), after unmarshaling:
// If session_churn is explicitly set in TOML, it overrides fragmentation_index
// (handled by BurntSushi/toml partial override — both fields default to true)
```

Actually, simpler approach: keep `signals.fragmentation_index` as the canonical field. Add `session_churn` as an alias that maps to the same field:

```toml
# These are equivalent:
session_churn = false
fragmentation_index = false
```

The simplest way: add both TOML keys. BurntSushi/toml will overwrite the struct field for whichever key appears last. If both appear, last one wins. Document this.

Or: remove `session_churn` entirely and update config with `fragmentation_index`. This is a breaking change but v2 is the right time.

**Decision:** Remove `session_churn`, replace with `fragmentation_index`. This is PR13+PR14 together — v2 migration. Document in release notes.

### `config/config.go` — Updated Defaults

```go
func Defaults() Config {
    return Config{
        Thresholds: Thresholds{
            CostOutlierSigma:            2.0,
            LowSignalPercentile:         10.0,
            CachePercentile:             10.0,
            SubagentOverheadPct:         50.0,
            ChurnMinSessions:            3,
            ChurnThreshold:              2.0,
            FragmentationIndexThreshold: 3.0,
            InputOverconsumptionSigma:   2.0,
            OutputExplosionSigma:        2.0,
            TokenEfficiencyPercentile:   10.0,
        },
        Signals: Signals{
            CostOutlier:          true,
            LowSignal:            true,
            SubagentOverhead:     true,
            CacheUnderutilized:   true,
            FragmentationIndex:   true,
            InputOverconsumption: true,
            OutputExplosion:      true,
            TokenEfficiency:      true,
        },
        // ... Filters, Output unchanged ...
    }
}
```

### `config/config.go` — Extended Validate

```go
func Validate(cfg Config) error {
    // ... existing checks ...
    if cfg.Thresholds.CachePercentile <= 0 || cfg.Thresholds.CachePercentile >= 100 {
        return fmt.Errorf("cache_percentile must be in (0, 100), got %f", cfg.Thresholds.CachePercentile)
    }
    if cfg.Thresholds.SubagentOverheadPct <= 0 || cfg.Thresholds.SubagentOverheadPct >= 100 {
        return fmt.Errorf("subagent_overhead_pct must be in (0, 100), got %f", cfg.Thresholds.SubagentOverheadPct)
    }
    if cfg.Thresholds.ChurnMinSessions < 1 {
        return fmt.Errorf("churn_min_sessions must be >= 1, got %d", cfg.Thresholds.ChurnMinSessions)
    }
    if cfg.Thresholds.FragmentationIndexThreshold <= 0 {
        return fmt.Errorf("fragmentation_index_threshold must be > 0, got %f", cfg.Thresholds.FragmentationIndexThreshold)
    }
    if cfg.Thresholds.InputOverconsumptionSigma <= 0 {
        return fmt.Errorf("input_overconsumption_sigma must be > 0, got %f", cfg.Thresholds.InputOverconsumptionSigma)
    }
    if cfg.Thresholds.OutputExplosionSigma <= 0 {
        return fmt.Errorf("output_explosion_sigma must be > 0, got %f", cfg.Thresholds.OutputExplosionSigma)
    }
    if cfg.Thresholds.TokenEfficiencyPercentile <= 0 || cfg.Thresholds.TokenEfficiencyPercentile >= 100 {
        return fmt.Errorf("token_efficiency_percentile must be in (0, 100), got %f", cfg.Thresholds.TokenEfficiencyPercentile)
    }
    return nil
}
```

### `analyze/baseline.go` — Wire percentiles

```go
func ComputeBaselines(events []source.TokenEvent, cfg config.Config) map[string]Baseline {
    // ... existing grouping ...

    for key, group := range groups {
        // ... aggregate ...
        b := buildBaseline(key, metrics, cfg)
        result[key] = b
    }

    global := buildBaseline(globalKey, allSessionMetrics, cfg)
    result[globalKey] = global
    return result
}

func buildBaseline(key string, metrics []sessionMetrics, cfg config.Config) Baseline {
    // ... existing cost, ratio computation ...

    // Use configurable percentile instead of hardcoded 10
    b.RatioP10 = percentile(b.Ratios, cfg.Thresholds.LowSignalPercentile)
    b.RatioP50 = percentile(b.Ratios, 50)
    b.RatioP90 = percentile(b.Ratios, 90)
    b.CacheP10 = percentile(b.CacheRates, cfg.Thresholds.CachePercentile)
    b.CacheP50 = percentile(b.CacheRates, 50)

    // Token baselines (from PR12)
    // ...
    b.TERP10 = percentile(b.TERs, cfg.Thresholds.TokenEfficiencyPercentile)

    return b
}
```

### `analyze/waste.go` — Wire thresholds

```go
func DetectWaste(events []source.TokenEvent, baselines map[string]Baseline, cfg config.Config) []WasteSignal {
    // ... existing ...

    // Replace hardcoded constants with config values
    for _, a := range agg {
        if toggles.CostOutlier {
            bl := findBaseline(a, baselines)
            if s := checkCostOutlier(a, bl, cfg.Thresholds.CostOutlierSigma); s != nil {
                signals = append(signals, *s)
            }
        }
        if toggles.InputOverconsumption {
            bl := findBaseline(a, baselines)
            if s := checkInputOverconsumption(a, bl, cfg.Thresholds.InputOverconsumptionSigma); s != nil {
                signals = append(signals, *s)
            }
        }
        // ... etc for all heuristics ...
    }

    // Subagent overhead: use cfg.Thresholds.SubagentOverheadPct
    // Fragmentation index: use cfg.Thresholds.FragmentationIndexThreshold + ChurnMinSessions
}
```

### `cmd/root.go` — Add new CLI flags

```go
// New flags (add to existing flags struct):
flag.Float64Var(&flags.InputOverconsumptionSigma, "input-sigma", 0,
    "Sigma for input overconsumption detection (0 = use config)")
flag.Float64Var(&flags.OutputExplosionSigma, "output-sigma", 0,
    "Sigma for output explosion detection (0 = use config)")
flag.Float64Var(&flags.TokenEfficiencyPercentile, "ter-percentile", 0,
    "Percentile for token efficiency threshold (0 = use config)")
flag.Float64Var(&flags.FragmentationThreshold, "fragmentation-threshold", 0,
    "Threshold for fragmentation index (0 = use config)")
flag.Float64Var(&flags.SubagentOverheadPct, "subagent-overhead", 0,
    "Subagent overhead percentage threshold (0 = use config)")

flag.BoolVar(&flags.NoInputOverconsumption, "no-input-overconsumption", false,
    "Disable input overconsumption detection")
flag.BoolVar(&flags.NoOutputExplosion, "no-output-explosion", false,
    "Disable output explosion detection")
flag.BoolVar(&flags.NoTokenEfficiency, "no-token-efficiency", false,
    "Disable token efficiency detection")
flag.BoolVar(&flags.NoFragmentationIndex, "no-fragmentation-index", false,
    "Disable fragmentation index detection")

// Merge: CLI flag > config > defaults
if flags.InputOverconsumptionSigma > 0 {
    cfg.Thresholds.InputOverconsumptionSigma = flags.InputOverconsumptionSigma
}
// ... same pattern for all numeric thresholds ...
```

---

## Test Requirements

1. **`config/config_test.go`**:
   - Defaults() returns sensible values for all new fields
   - TOML parse: all new fields read correctly
   - TOML parse: partial config (only some fields) → others use defaults
   - Validate: all new fields with valid values pass
   - Validate: each invalid field produces correct error message
   - Validate: percentile 0, 100, 101 — catch boundary cases
   - Validate: sigma=0 → error
   - Validate: churn_min_sessions=0 → error

2. **`analyze/waste_test.go`**:
   - Pass config with custom sigma → threshold changes
   - Pass config with different percentile → P10 changes
   - Update all existing test helpers to pass valid config

3. **`analyze/baseline_test.go`**:
   - Pass config with custom percentile → P10 computed correctly
   - Default percentile=10 → same as before (backward compatible)

4. Coverage target: >=90% on new code

---

## Approach

1. Extend config structs (Thresholds, Signals)
2. Update Defaults() and Validate()
3. Write config tests (RED → GREEN)
4. Update buildBaseline to accept config, wire percentiles
5. Update DetectWaste to accept config, wire all thresholds
6. Update all callers (cmd/root.go, tests)
7. Add new CLI flags
8. Full test suite + lint

---

## Exit Criteria

- [ ] Pull latest main
- [ ] Create feature branch from main: `git checkout -b pr14-config-wiring`
- [ ] Lint passes (zero warnings) — `golangci-lint run`
- [ ] Build compiles cleanly — `go build -o burnwatch .`
- [ ] Tests pass with >=90% coverage on new code — `go test ./... -cover`
- [ ] Self-review: run through [docs/code-review.md](../code-review.md)
- [ ] Document learnings in `docs/learnings.md`
- [ ] Commit: `feat: complete config wiring for all heuristic thresholds`
- [ ] Push to branch `pr14-config-wiring`
- [ ] Open pull request
- [ ] Dispatch CodeReviewer subagent against the PR diff
- [ ] Update `docs/plans/progress.md` to reflect merge
- [ ] Merge to main
- [ ] Delete feature branch after merge

---

## Notes

- `session_churn` TOML key is removed. Users must update their config to use `fragmentation_index`. Document in release notes.
- `ComputeBaselines` currently takes no config. Adding `cfg config.Config` parameter changes its signature — update ALL callers (cmd/root.go, all test files).
- `DetectWaste` currently takes `costSigma float64, toggles SignalToggles`. Change to `cfg config.Config` — extract sigma/toggles inside the function. This reduces parameter count as we add more thresholds.
- The `LowSignalPercentile` field already exists in config but isn't wired. This PR wires it.
- No golden file changes needed (defaults produce same output as before).
