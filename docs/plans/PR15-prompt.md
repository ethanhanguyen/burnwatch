# PR15: Fix Pricing + Uncosted — 1000x Bug, Cache Corruption, Fallback Removal

> **Workflow:** Follow `docs/PR-template.md`. Review `AGENTS.md` behavioral guidelines before implementing.

## Objective

Fix three pricing bugs that inflate costs 1000x and fabricate costs for unknown models. After this PR, costs match actual billing and uncosted sessions are flagged transparently instead of priced with a guess.

## Success Criteria

- [ ] 1000 input tokens at embedded sonnet pricing = $0.003 (not $3.00)
- [ ] Pricing cache with <50 entries treated as stale → re-fetched from OpenRouter
- [ ] OpenCode source calculates cost via `CostForModel()` (same path as Claude source)
- [ ] No price fallback — unknown models get `CostUnknown` instead of guessed price
- [ ] Uncosted sessions displayed with `$?` and token-count heuristics still fire (H2, H4, H6, H7, H8)
- [ ] Cost-based heuristics (H1, H3, H9) skip uncosted sessions
- [ ] `--refresh-pricing` works and writes valid cache with >500 entries
- [ ] All existing tests pass; golden files updated for corrected costs
- [ ] Regression test: 1K input tokens at sonnet = $0.003

## Dependencies

- **Must merge first:** PR14 (config fields for uncosted toggle)
- **External dependencies:** None
- **Can be parallel with:** None (blocks PR16 and all subsequent analysis)
- **Breaking changes / Migrations needed:** All golden files (costs drop 1000x). `WasteSignal` gains `CostUnknown` field. `TokenEvent` gains `CostUnknown` field.

---

## Pre-flight

- [ ] Pull latest main: `git fetch origin && git checkout main && git pull origin main`
- [ ] Create feature branch: `git checkout -b pr15-pricing-fix`
- [ ] Verify build environment works on clean main

## Files

| File | Purpose | Notes |
|------|---------|-------|
| `source/pricing.go` | Fix embedded price units, remove fallback, return approximate on lookup fail | Modify |
| `source/pricing_fetcher.go` | Cache validation: <50 entries → stale | Modify |
| `source/opencode.go` | Replace `td.Cost` with `CostForModel()` call | Modify |
| `source/event.go` | Add `CostUnknown bool` to `TokenEvent` | Modify |
| `source/claude.go` | Propagate `CostUnknown` from `CostForModel` | Modify |
| `analyze/waste.go` | Add `costUnknown` to `sessionAgg`, skip H1/H3/H9 for uncosted | Modify |
| `output/text.go` | Display `$?` and `[no pricing data]` for uncosted | Modify |
| `output/json.go` | Add `cost_unknown` to JSON output | Modify |
| `source/pricing_test.go` | Regression: 1000 tokens at sonnet = $0.003 | Modify |
| `source/pricing_fetcher_test.go` | Cache validation: <50 entries stale test | Modify |
| `analyze/waste_test.go` | Uncosted sessions: H6 fires, H1 skips | Modify |
| `output/scenario_test.go` | Uncosted session scenario | Modify |
| All `*_test.go` | Golden files update (costs drop 1000x) | Modify |

---

## Implementation

### A. Fix embedded pricing units (`source/pricing.go:22-28`)

The embedded table stores $/MTok values but `CostForModel()` divides tokens by 1000 (treating as $/1K). Divide all 6 entries by 1000 to match fetched pricing format.

```
BEFORE:
  {"claude-sonnet-4-5", priceEntry{3.00,  15.00,  0.30,  3.75}},
  {"claude-opus-4-5",   priceEntry{15.00, 75.00,  1.50,  18.75}},
  {"claude-haiku-4-5",  priceEntry{0.80,  4.00,   0.08,  1.00}},
  {"gemini-3-pro",      priceEntry{1.25,  5.00,   0,     0}},
  {"gemini-2.5-pro",    priceEntry{1.25,  5.00,   0,     0}},
  {"gemini-2.5-flash",  priceEntry{0.15,  0.60,   0,     0}},
  fallback {3.00, 15.00, 0.30, 3.75}
AFTER:
  {"claude-sonnet-4-5", priceEntry{0.003,  0.015,  0.0003,  0.00375}},
  {"claude-opus-4-5",   priceEntry{0.015,  0.075,  0.0015,  0.01875}},
  {"claude-haiku-4-5",  priceEntry{0.0008, 0.004,  0.00008, 0.001}},
  {"gemini-3-pro",      priceEntry{0.00125,0.005,  0,       0}},
  {"gemini-2.5-pro",    priceEntry{0.00125,0.005,  0,       0}},
  {"gemini-2.5-flash",  priceEntry{0.00015,0.0006, 0,       0}},
```

### B. Fix pricing cache (`source/pricing_fetcher.go:139-142`)

Add cache validation in `LoadCache()`:

```
if len(cache.Entries) < 50 {
    return nil, fmt.Errorf("cache has too few entries (%d), treating as stale", len(cache.Entries))
}
```

### C. Fix OpenCode cost source (`source/opencode.go:170`)

Replace:
```go
CostUSD: td.Cost,
```

With:
```go
CostUSD: ... // calculated from tokens via CostForModel, same as claude.go
```

Full replacement in `tokenDataToEvent`:
```go
cost, approx := CostForModel(
    td.ModelID,
    inputTokens,
    outputTokens,
    cacheRead,
    cacheWrite,
)

return TokenEvent{
    // ... existing fields ...
    CostUSD:         cost,
    CostUnknown:     model == "" || cost == 0, // fallback case, approximate handled separately
    CostApproximate: approx,
}
```

### D. Remove fallback + add uncosted path (`source/pricing.go:30,126`)

Remove `var fallback = priceEntry{3.00, 15.00, 0.30, 3.75}`.

Change `CostForModel` to return a third value:
```go
func CostForModel(model string, inputTokens, outputTokens, cacheRead, cacheWrite int64) (float64, bool, bool)
// returns: cost, approximate (from fetched pricing), costUnknown (no pricing found)
```

In `lookupPrice`, when no match found, return `priceEntry{}, true, true`.

All callers in `claude.go` and `opencode.go` must handle the third return value.

### E. Add `CostUnknown` field (`source/event.go`)

```go
type TokenEvent struct {
    // ... existing ...
    CostUnknown bool
}
```

### F. Uncosted handling in waste analysis (`analyze/waste.go`)

In `sessionAgg`:
```go
type sessionAgg struct {
    // ... existing ...
    costUnknown bool
}
```

Aggregation:
```go
if e.CostUnknown {
    a.costUnknown = true
}
```

In heuristics: H1, H3, H9 check `a.costUnknown` first and return nil. H2, H4, H6, H7, H8 run normally.

### G. Output display (`output/text.go`, `output/json.go`)

Uncosted sessions:
```
HIGH ses_abc123 (project): unknown cost — 361K input tokens (μ=79K, σ=78K)
    Model: deepseek-v4-pro [no pricing data]
    → Reduce input context bloat.
```

JSON: `"cost_unknown": true`, `"session_cost": 0`.

---

## Test Requirements

1. **`source/pricing_test.go`**:
   - 1000 input + 0 output at embedded sonnet = $0.003 (was $3.00)
   - 500 input + 200 output at embedded opus = $0.0075 + $0.015 = $0.0225
   - Unknown model returns `costUnknown=true`, cost=0, not fallback price
   - CostForModel signature changed: verify 3 return values

2. **`source/pricing_fetcher_test.go`**:
   - Cache with 49 entries → stale (LoadCache returns error)
   - Cache with 50 entries → valid
   - Empty cache file → stale

3. **`analyze/waste_test.go`**:
   - Uncosted session: H6 fires (input overconsumption)
   - Uncosted session: H1 does NOT fire (cost outlier)
   - Uncosted session: H3 does NOT fire (subagent)
   - Uncosted session: H9 does NOT fire (fragmentation)

4. **`output/scenario_test.go`**:
   - Scenario: one session with no pricing data, flagged for input overconsumption but not cost outlier
   - `testdata/scenarios/uncosted_input.jsonl`

5. Golden files: all `*_test.go` golden files updated for 1000x cost reduction

6. Coverage target: >=90% on new code

---

## E2E Scenario Tests

1. **Scenario file**: `testdata/scenarios/uncosted_input.jsonl`
   - 6 Claude-format sessions, all using `deepseek-v4-pro` (no embedded pricing, no fetched pricing match)
   - 5 sessions: token counts within normal range
   - 1 session: `ses_uncosted_waste` — 300K input tokens (well above project mean of 50K)
   - Expected: session flagged for H6 (input_overconsumption), NOT H1 (cost_outlier)
   - Model field left empty or set to a model not in pricing

2. **Scenario test**: `output/scenario_test.go`
   - `TestScenario_UncostedInput`: loads scenario, mocks pricing lookup failure
   - Asserts `ses_uncosted_waste` flagged with reason `input_overconsumption`
   - Asserts no cost_outlier signals for this session
   - Asserts `cost_unknown` field is true in JSON output

3. Labels file: `testdata/labels/labels.jsonl` — mark `ses_uncosted_waste` as WASTE (input bloat)

---

## Benchmarking

Not required (no new algorithms or data paths).

---

## Signal Quality

Not required (no new or modified heuristics).

---

## Approach

1. Fix embedded pricing units (A) — simplest, highest impact
2. Fix cache validation (B) — prevents future corruption
3. Change `CostForModel` signature (D) — 3 return values
4. Update Claude source callers (claude.go) for new signature
5. Fix OpenCode cost source (C) — same path as Claude
6. Add `CostUnknown` to `TokenEvent` (E)
7. Add uncosted handling in waste.go (F) — sessionAgg + heuristic skips
8. Add output display changes (G)
9. Write tests (RED → GREEN) for every change
10. Update golden files (costs drop 1000x)
11. Run full test suite + lint

---

## Exit Criteria

- [ ] Pull latest main
- [ ] Create feature branch from main: `git checkout -b pr15-pricing-fix`
- [ ] Lint passes (zero warnings) — `golangci-lint run`
- [ ] Build compiles cleanly — `go build -o burnwatch .`
- [ ] Tests pass with >=90% coverage on new code — `go test ./... -cover`
- [ ] Self-review: run through [docs/code-review.md](../code-review.md)
- [ ] **Validation gate P15.1**: Run on real data, verify costs match actual billing
- [ ] **Validation gate P15.2**: `--refresh-pricing` writes cache with >500 entries
- [ ] **Validation gate P15.3**: No crash when pricing fetch fails (simulate with invalid URL)
- [ ] Document learnings in `docs/learnings.md`
- [ ] Commit: `fix: embedded pricing 1000x bug, cache validation, uncosted fallback`
- [ ] Push to branch `pr15-pricing-fix`
- [ ] Open pull request with description
- [ ] Dispatch CodeReviewer subagent against the PR diff
- [ ] Update `docs/plans/progress.md` to reflect merge
- [ ] Merge to main
- [ ] Delete feature branch after merge

---

## Notes

- After this fix, the pricing cache file at `~/Library/Caches/burnwatch/pricing.json` with the single `model-1` entry must be deleted or re-fetched. The stale-cache check (`len < 50`) handles this automatically.
- OpenRouter API returns prices per token (e.g. `"0.000003"`). `parsePricingFloat` × 1000 converts to $/1K format. The fixed embedded table now uses the same $/1K format.
- `costUnknown` vs `costApproximate`: `Approximate` means fetched pricing used (≈ marker). `Unknown` means no pricing found at all ($? marker). They're orthogonal.
- Cache read/write aggregation must also use `CostForModel` for OpenCode (currently uses `Sum of e.CostUSD`). After fix C, all costs come from the same function.
