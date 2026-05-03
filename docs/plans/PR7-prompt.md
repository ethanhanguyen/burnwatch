# PR7: Config File — Customizable Thresholds, Filters, Output

> **Workflow:** Follow `docs/plans/PR-template.md`. Review `AGENTS.md` behavioral guidelines before implementing.

## Objective

Add a TOML config file that lets users customize waste detection thresholds, toggle signal types, set cost filters, and control output formatting — the foundation PRs 8-10 build on.

## Success Criteria

- [ ] `burnwatch` runs with no config file (zero-config still works, all defaults sensible)
- [ ] Config loaded from `--config <path>`, otherwise `./.burnwatch.toml`, otherwise `~/.config/burnwatch/config.toml`
- [ ] CLI flags override config values (flag priority > config file > defaults)
- [ ] `--print-config` flag outputs the effective config and exits
- [ ] Invalid config file produces clear error message, not panic

## Dependencies

- **Must merge first:** None (basis for PR8-PR10)
- **External dependencies:** `github.com/BurntSushi/toml` (add to go.mod)
- **Can be parallel with:** None
- **Breaking changes / Migrations needed:** None

---

## Pre-flight

- [ ] Pull latest main: `git fetch origin && git checkout main && git pull origin main`
- [ ] Create feature branch: `git checkout -b pr7-config`
- [ ] Verify build environment works on clean main

## Files

| File | Purpose | Notes |
|------|---------|-------|
| `config/config.go` | Config struct, Load(), Defaults(), Validate() | New package |
| `config/config_test.go` | Parse test, default test, override test, validate test | Table-driven |
| `cmd/root.go` | Add `--config` and `--print-config` flags, load config, wire into pipeline | Modify existing |

---

## Implementation

### `config/config.go`

```go
package config

type Config struct {
    Thresholds Thresholds
    Signals    Signals
    Filters    Filters
    Output     Output
}

type Thresholds struct {
    CostOutlierSigma    float64 `toml:"cost_outlier_sigma"`
    LowSignalPercentile float64 `toml:"low_signal_percentile"`
}

type Signals struct {
    CostOutlier        bool `toml:"cost_outlier"`
    LowSignal          bool `toml:"low_signal"`
    SubagentOverhead   bool `toml:"subagent_overhead"`
    CacheUnderutilized bool `toml:"cache_underutilized"`
    SessionChurn       bool `toml:"session_churn"`
}

type Filters struct {
    MinCost     float64 `toml:"min_cost"`
    Deduplicate bool    `toml:"deduplicate"`
}

type Output struct {
    GroupChurn  bool `toml:"group_churn"`
    ShowTrends  bool `toml:"show_trends"`
}

func Defaults() Config {
    return Config{
        Thresholds: Thresholds{
            CostOutlierSigma:    2.0,
            LowSignalPercentile: 10.0,
        },
        Signals: Signals{
            CostOutlier:        true,
            LowSignal:          true,
            SubagentOverhead:   true,
            CacheUnderutilized: true,
            SessionChurn:       true,
        },
        Filters: Filters{
            MinCost:     0,
            Deduplicate: false,
        },
        Output: Output{
            GroupChurn: false,
            ShowTrends: false,
        },
    }
}

func Load(path string) (Config, error) { /* ... */ }
func Validate(cfg Config) error       { /* ... */ }
```

**Load() algorithm:**
1. If `path != ""`, parse that file only (error if missing/invalid).
2. Else try `./.burnwatch.toml`, then `~/.config/burnwatch/config.toml`.
3. If no file found, return `Defaults()`.
4. Parse TOML, unmarshal into Config.
5. Return merged result (TOML values override defaults).

**Validate() checks:**
- `CostOutlierSigma > 0`
- `LowSignalPercentile` in range (0, 100)
- `MinCost >= 0`

**Constraints:**
- No global mutable state. Config is a value passed through the pipeline.
- `Load()` returns `Config, error` — caller decides to exit or use defaults on error.
- `BurntSushi/toml` is the only new dependency.

**Error handling:**
- File not found (when no `--config` flag) → return Defaults(), no error.
- File not found (with `--config` flag) → error.
- Parse error (invalid TOML) → error with line info from library.
- Validation error → error with field name.

### `cmd/root.go` modifications

Add flags:
```go
var flags struct {
    // ... existing ...
    ConfigPath  string
    PrintConfig bool
}

flag.StringVar(&flags.ConfigPath, "config", "", "Config file path (default: ./.burnwatch.toml, ~/.config/burnwatch/config.toml)")
flag.BoolVar(&flags.PrintConfig, "print-config", false, "Print effective config and exit")
```

Pipeline integration:
```go
cfg := config.Defaults()
if loaded, err := config.Load(flags.ConfigPath); err != nil {
    fmt.Fprintf(os.Stderr, "Config error: %v\n", err)
    os.Exit(1)
} else {
    cfg = loaded
}

if flags.PrintConfig {
    // marshal and print cfg as TOML, then exit
}

// Override with CLI flags where applicable
// (flags always take priority over config)
```

**CLI override rules:**
- `--min-cost N` (future) overrides `config.filters.min_cost`
- No existing flags conflict with config fields in this PR
- PR9 adds `--min-cost` as a CLI flag override

---

## Test Requirements

1. **`config/config_test.go`**:
   - `TestDefaults` — verify all default values are sensible
   - `TestLoad_NoFile` — returns Defaults(), no error
   - `TestLoad_FileNotFound` (with explicit path) — returns error
   - `TestLoad_ValidTOML` — write temp file, load, verify values
   - `TestLoad_InvalidTOML` — returns parse error
   - `TestLoad_PartialOverride` — TOML sets only `thresholds.cost_outlier_sigma`, rest stay default
   - `TestValidate_NegativeSigma` — returns error
   - `TestValidate_BadPercentile` — returns error for 0, 100, -1, 101
   - `TestLoad_SearchOrder` — verify `--config` flag takes priority

2. Coverage target: >=90% on `config/`

---

## Approach

1. Add dependency: `go get github.com/BurntSushi/toml`
2. Write `config/config_test.go` (RED)
3. Implement `config/config.go` (GREEN)
4. Add `--config` and `--print-config` to `cmd/root.go`
5. Test CLI: `go run . --print-config`
6. Run full test suite

---

## Exit Criteria

- [ ] Pull latest main
- [ ] Create feature branch from main: `git checkout -b pr7-config`
- [ ] Lint passes (zero warnings) — `go vet ./... && golangci-lint run`
- [ ] Build compiles cleanly — `go build -o burnwatch .`
- [ ] Tests pass with >=90% coverage — `go test ./... -cover`
- [ ] `./burnwatch` works with no config file
- [ ] `./burnwatch --print-config` shows defaults
- [ ] `./burnwatch --config /nonexistent` exits with clear error
- [ ] Self-review: run through [docs/code-review.md](../code-review.md)
- [ ] Document learnings in `docs/learnings.md`
- [ ] Commit: `feat: add TOML config file for threshold, filter, and output customization`
- [ ] Push to branch `pr7-config`
- [ ] Open pull request with description
- [ ] Perform code review
- [ ] Update `docs/plans/progress.md` to reflect merge
- [ ] Merge to main
- [ ] Delete feature branch after merge

## Notes

- TOML chosen over JSON for readability. `BurntSushi/toml` is the standard Go TOML library.
- Config is a value type, not a global — passed explicitly to functions that need it.
- Config loading is at the `cmd` level, not in library packages. Analysis functions accept parameters, not a Config struct directly.
- `ShowTrends` and `GroupChurn` fields are forward-declared for PR8/PR10. They default to false and do nothing until those PRs.
