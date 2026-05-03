# PR16: Output Quality — Fragment Noise, Savings Dedup, Config Init

> **Workflow:** Follow `docs/PR-template.md`. Review `AGENTS.md` behavioral guidelines before implementing.

## Objective

Suppress pointless fragmentation signals for near-free sessions, make "Potential savings" numbers honest by deduplicating across heuristics, and ship a default config file so new users don't start with broken defaults.

## Success Criteria

- [ ] Sessions below `fragmentation_min_cost` ($0.50 default) are skipped by H9 fragmentation index
- [ ] "Potential savings" per session capped at session cost (can't save more than you spent)
- [ ] Summary line deduplicates savings across heuristics for the same session (take max, not sum)
- [ ] `burnwatch --init` writes `.burnwatch.toml` with defaults and comments
- [ ] `--init` refuses to overwrite existing config (safety check)
- [ ] Config shipped as `config.example.toml` in repo root
- [ ] Fragmentation signals drop from 1,900+ to <100 on real data (no sub-$0.50 noise)
- [ ] All existing tests pass; new tests for min-cost, dedup, init

## Dependencies

- **Must merge first:** PR15 (pricing fix — needs correct costs for min-cost gating)
- **External dependencies:** None
- **Can be parallel with:** None (sequential after PR15)
- **Breaking changes / Migrations needed:** New config fields. Summary line format changes.

---

## Pre-flight

- [ ] Pull latest main: `git fetch origin && git checkout main && git pull origin main`
- [ ] Create feature branch: `git checkout -b pr16-output-quality`
- [ ] Verify build environment works on clean main

## Files

| File | Purpose | Notes |
|------|---------|-------|
| `config/config.go` | Add `fragmentation_min_cost` threshold | Modify |
| `config/config_test.go` | Test new defaults | Modify |
| `analyze/waste.go` | Skip sub-threshold sessions in H9 | Modify |
| `output/text.go` | Dedup savings in summary line | Modify |
| `output/json.go` | Add `unique_savings` to summary | Modify |
| `output/text_test.go` | Test deduped summary | Modify |
| `cmd/root.go` | Add `--init` flag | Modify |
| `config.example.toml` | Default config with comments | New |
| `analyze/waste_test.go` | Test fragmentation_min_cost gating | Modify |
| `output/scenario_test.go` | Fragment noise scenario | Modify |

---

## Implementation

### 1. Fragmentation noise suppression (`config/config.go`, `analyze/waste.go`)

Add to `Thresholds`:
```go
FragmentationMinCost float64 `toml:"fragmentation_min_cost"` // default: 0.50
```

In `checkFragmentationIndex`, add gating at the start of the per-session loop:
```go
for _, s := range sessions {
    if s.cost < cfg.Thresholds.FragmentationMinCost {
        continue  // skip sessions below cost threshold
    }
    // ... existing code ...
}
```

**Rationale:** Sessions costing $0.03 with fragmentation index 94 generate noise, not signal. The heuristic is about wasted money — if the session cost is below the noise floor, fragmentation isn't material.

### 2. Savings deduplication (`output/text.go`)

Current (buggy): sums every heuristic's claimed savings independently. Same session flagged by H1, H6, H7, H8 appears 4x.

Fix: in summary calculation, deduplicate by SessionID — take max savings among all signals for that session.

```go
func dedupedSavings(signals []WasteSignal) float64 {
    bySession := make(map[string]float64)
    for _, s := range signals {
        saving := s.SessionCost - (s.SessionCost * savingsRatio(s))
        if saving > bySession[s.SessionID] {
            bySession[s.SessionID] = saving
        }
    }
    var total float64
    for _, s := range bySession {
        total += min(s, ...) // capped at session cost
    }
    return total
}
```

Also cap per-session savings at the session's total cost:
```go
potentialSaving := min(saving, s.SessionCost)
```

### 3. Config init (`cmd/root.go`, `config.example.toml`)

`--init` flag:
```go
flag.BoolVar(&flags.Init, "init", false, "Write default .burnwatch.toml and exit")

// In Execute():
if flags.Init {
    if _, err := os.Stat(".burnwatch.toml"); err == nil {
        fmt.Fprintln(os.Stderr, "Error: .burnwatch.toml already exists. Delete it first to regenerate.")
        os.Exit(1)
    }
    // Write embedded default config with comments
    config.WriteDefault(os.Stdout) // or to file
    return
}
```

`config.example.toml` shipped in repo root — matches what `--init` writes.

---

## Test Requirements

1. **`analyze/waste_test.go`**:
   - Session with cost $0.03, fragmentation index 50, min_cost=0.50 → NOT flagged
   - Session with cost $2.50, fragmentation index 50, min_cost=0.50 → flagged
   - Session with cost exactly $0.50, fragmentation index 50 → flagged (>=, not >)

2. **`output/text_test.go`**:
   - Same session flagged by H1+H6+H7+H8 → savings counted once, not 4x
   - Session cost $10, all savings sum to $30 → summary shows $10 max
   - Dedup across sessions: 2 sessions × $5 each → total $10 savings shown

3. **`cmd/root_test.go`**:
   - `--init` writes `.burnwatch.toml` in tmpdir
   - `--init` refuses to overwrite existing `.burnwatch.toml`
   - Generated config can be loaded by `config.Load()` without errors

4. **`output/scenario_test.go`**:
   - Fragment scenario with sub-threshold cost: `testdata/scenarios/fragment_below_min.jsonl`

5. Coverage target: >=90% on new code

---

## E2E Scenario Tests

1. **Scenario file**: `testdata/scenarios/fragment_below_min.jsonl`
   - 6 Claude-format sessions, all same project, same day, all costing $0.10 each
   - Fragmentation index = 6 * (1 - 0.5) = 3.0 (above default threshold 1.5)
   - With `fragmentation_min_cost = 0.50`, these should NOT trigger H9
   - With `fragmentation_min_cost = 0.05`, these SHOULD trigger H9

2. **Scenario test**: `output/scenario_test.go`
   - `TestScenario_FragmentBelowMin`: runs pipeline with min_cost=0.50, asserts no fragmentation signals
   - Same scenario with min_cost=0.05, asserts fragmentation signals appear

---

## Benchmarking

Not required (no new algorithms or data paths).

---

## Signal Quality

- [ ] Labels file updated for fragment_below_min scenario
- [ ] Verify real-data fragmentation signals drop from 1,900+ to under 100 with min_cost=0.50

---

## Approach

1. Add `fragmentation_min_cost` to config (step 1)
2. Gate H9 on session cost (step 1)
3. Write tests (RED → GREEN) for fragmentation gating
4. Write dedup logic in output layer (step 2)
5. Write tests for savings dedup
6. Implement `--init` flag (step 3)
7. Write `config.example.toml`
8. Run full test suite + lint

---

## Exit Criteria

- [ ] Pull latest main
- [ ] Create feature branch from main: `git checkout -b pr16-output-quality`
- [ ] Lint passes (zero warnings) — `golangci-lint run`
- [ ] Build compiles cleanly — `go build -o burnwatch .`
- [ ] Tests pass with >=90% coverage on new code — `go test ./... -cover`
- [ ] Self-review: run through [docs/code-review.md](../code-review.md)
- [ ] **Validation gate P16.1**: Run on real data, verify fragmentation signals under 100 (was 1,900+)
- [ ] **Validation gate P16.2**: Verify "Potential savings" summary ≤ sum of all session costs
- [ ] **Validation gate P16.3**: `--init` creates valid `.burnwatch.toml` that survives `config.Load()`
- [ ] Document learnings in `docs/learnings.md`
- [ ] Commit: `fix: fragmentation min-cost gating, savings dedup, --init flag`
- [ ] Push to branch `pr16-output-quality`
- [ ] Open pull request with description
- [ ] Dispatch CodeReviewer subagent against the PR diff
- [ ] Update `docs/plans/progress.md` to reflect merge
- [ ] Merge to main
- [ ] Delete feature branch after merge

---

## Notes

- `fragmentation_min_cost = 0.50` is a sane default — drops 95%+ of OpenCode noise while keeping all Claude sessions (median $1,600+). Users can lower it to $0 if they want full visibility.
- Dedup logic lives in `output/text.go` (display concern), not in `analyze/waste.go` (analysis concern). The heuristics remain independent; only the summary deduplicates.
- `--init` writes to the current directory by default. Future: support `--init --global` for `~/.config/burnwatch/config.toml`.
