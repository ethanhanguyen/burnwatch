# PR11: Dynamic Pricing — OpenRouter API + Cache + Fallback

> **Workflow:** Follow `docs/PR-template.md`. Review `AGENTS.md` behavioral guidelines before implementing.

## Objective

Replace the hardcoded 6-model pricing table with dynamic pricing fetched from OpenRouter's free models API. Fixes dollar amounts for deepseek, kimi, minimax, qwen, gpt-5.4 — all of which currently use incorrect sonnet-tier fallback pricing (inflated 5x–17x).

## Success Criteria

- [ ] `burnwatch` runs with no network (uses cached pricing, or embedded fallback)
- [ ] First run fetches `https://openrouter.ai/api/v1/models` and caches to `~/.cache/burnwatch/pricing.json`
- [ ] Subsequent runs use cache (7-day TTL). Expired cache triggers background re-fetch with embedded fallback used immediately
- [ ] `--refresh-pricing` forces re-fetch and overwrites cache immediately
- [ ] Model lookup: exact match → substring match → cache entry → embedded table → sonnet fallback
- [ ] CostOutlier output shows `≈` prefix when fallback pricing was used (not exact/cached match)
- [ ] Pricing for these models is correct vs OpenRouter (spot-check: deepseek, kimi, minimax, qwen, claude-sonnet)
- [ ] `CostForModel()` signature unchanged — all callers compile without modification
- [ ] No new external dependencies (use `net/http` + `encoding/json` from stdlib)

## Pre-requisite: E2E Scenario Test Harness

Before implementing pricing changes, add the E2E scenario test infrastructure (already scaffolded). This section is a prerequisite — the scenario harness must pass before any PR11 pricing work begins.

### Files

| File | Purpose | Notes |
|------|---------|-------|
| `output/scenario_test.go` | Scenario test harness: `loadScenarioJSONL`, `findSignalByID`, `runPipeline`, per-scenario tests | Exists (scaffolded), verify passes |
| `output/bench_test.go` | Benchmark functions for pricing, pipeline, signal quality | Exists (scaffolded), verify passes |
| `testdata/scenarios/cost_outlier.jsonl` | Cost outlier scenario (6 sessions, 1 at 80x baseline) | Exists |
| `testdata/scenarios/all_clean.jsonl` | All-clean scenario (10 sessions, zero waste) | Exists |
| `testdata/labels/labels.jsonl` | Labeled sessions for signal quality benchmarking | Exists |

### Verification

```bash
go test ./output -run "TestScenario" -v -count=1   # all 9 scenarios pass
go test ./output -bench=. -benchmem -count=1       # all benchmarks run without panic
go test ./... -cover -count=1                      # 171 tests pass, ≥87% coverage
```

If any scenario test fails, diagnose with `-v` flag before proceeding to pricing implementation.

## Dependencies

- **Must merge first:** None (all PR1–PR10 merged). Pre-requisite scenario harness verified.
- **External dependencies:** None (stdlib only)
- **Can be parallel with:** None
- **Breaking changes / Migrations needed:** Golden files — all dollar amounts change for non-Anthropic/non-Google models

---

## Pre-flight

- [ ] Pull latest main: `git fetch origin && git checkout main && git pull origin main`
- [ ] Create feature branch: `git checkout -b pr11-dynamic-pricing`
- [ ] Verify build environment works on clean main

## Files

| File | Purpose | Notes |
|------|---------|-------|
| `source/pricing.go` | Replace hardcoded table with fetched pricing. Add `InitPricing()`, `RefreshPricing()` | Modify |
| `source/pricing_fetcher.go` | OpenRouter API client, JSON parser, cache reader/writer | New file |
| `source/pricing_test.go` | Test lookup chain, fallback behavior, substring matching | Modify |
| `source/pricing_fetcher_test.go` | Test fetch, cache, TTL, refresh | New file |
| `output/text.go` | Display `≈` prefix when `CostApproximate` is true on WasteSignal | Modify |
| `output/text_test.go` | Golden file updates for `≈` indicator | Modify |
| `cmd/root.go` | Add `--refresh-pricing` flag, call `InitPricing()` on startup | Modify |
| `testdata/expected_report.txt` | Regenerate (dollar amounts change) | Update |
| `testdata/expected_report.json` | Regenerate | Update |

---

## Implementation

### `source/pricing_fetcher.go` — OpenRouter client + cache

```
Type: CacheEntry
Fields:
  FetchedAt  time.Time     `json:"fetched_at"`
  TTL        time.Duration `json:"ttl"`
  Entries    []PricingEntry

Type: PricingEntry
Fields:
  Key        string  `json:"key"`         // normalized model ID for matching
  Input      float64 `json:"input"`       // $ per 1K tokens (same unit as existing)
  Output     float64 `json:"output"`      // $ per 1K tokens
  CacheRead  float64 `json:"cache_read"`  // $ per 1K tokens

Type: Fetcher struct{} — no state, pure functions

Functions:
  FetchPricing(client *http.Client) ([]PricingEntry, error)
    → GET https://openrouter.ai/api/v1/models
    → Parse JSON: for each entry in data[], extract:
        key = normalizeModelID(entry.id)
        input = entry.pricing.prompt * 1000   // convert per-token to per-1K
        output = entry.pricing.completion * 1000
        cacheRead = entry.pricing.input_cache_read * 1000 (or 0 if absent)
    → Return []PricingEntry

  normalizeModelID(openRouterID string) string
    → Strip provider prefix: "anthropic/claude-sonnet-4.6" → "claude-sonnet-4-6"
    → Normalize dots to hyphens: "4.6" → "4-6"
    → Lowercase

  CachePath() string
    → os.UserCacheDir() + "/burnwatch/pricing.json"

  LoadCache(path string) (*CacheEntry, error)

  SaveCache(path string, cache *CacheEntry) error

  IsFresh(cache *CacheEntry) bool
    → time.Since(cache.FetchedAt) < cache.TTL
```

**Constraints:**
- HTTP client has 10s timeout, 2 retries with backoff (stdlib `net/http` + simple loop)
- Cache file is user-readable JSON (not binary) — users can inspect
- Failed fetch returns error but does NOT block startup — fall through to embedded table
- Fetch is synchronous on first run (no goroutines — keep architecture simple)

**Error handling:**
- HTTP error → log to stderr, use embedded table, continue
- JSON parse error → invalidate cache, use embedded table, continue
- Cache write error → log to stderr, continue (non-fatal)
- Empty API response → use embedded table

### `source/pricing.go` — Replace lookup logic

```
New types to add:
  var fetchedPricing []PricingEntry   // populated by InitPricing
  var pricingInitialized bool         // guard

New function: InitPricing(client *http.Client) error
  → Load cache
  → If cache missing or stale: FetchPricing, SaveCache
  → Populate fetchedPricing with merged list (cache + embedded fallbacks)
  → Set pricingInitialized = true

New function: RefreshPricing(client *http.Client) error
  → FetchPricing, SaveCache, overwrite fetchedPricing unconditionally
  → Set pricingInitialized = true

Modify CostForModel:
  → Add return value: (cost float64, approximate bool)
  → Lookup order:
    1. exact match in fetchedPricing (strings.EqualFold)
    2. substring match in fetchedPricing (strings.Contains)
    3. substring match in embedded pricing (existing slice)
    4. fallback — return approximate=true
  → All per-token prices are now per-1K tokens (same unit as before)
  → Pseudocode:
    for _, e := range fetchedPricing {
        if match(e.Key, model) {
            return computeCost(e.Input, e.Output, e.CacheRead, tokens...), false
        }
    }
    for _, e := range pricing {
        if strings.Contains(model, e.key) {
            return computeCost(e.p.input, e.p.output, e.p.cacheRead, e.p.cacheWrite, tokens...), false
        }
    }
    return computeCost(fallback.input, fallback.output, fallback.cacheRead, fallback.cacheWrite, tokens...), true
```

**match() rules:**
1. `strings.EqualFold(modelLower, entryKeyLower)` — exact match
2. `strings.Contains(modelLower, entryKeyLower)` — substring match
3. Multiple substring matches → pick longest key (most specific)

**Backward compatibility:** All callers of `CostForModel` must update to accept the new `approximate` return value. Add to `TokenEvent`:
```
CostApproximate bool  // true when fallback pricing was used
```

### `output/text.go` — `≈` indicator

```
In writeSignalBlock(), when s.CostApproximate:
  → Prefix cost display with "≈":  "≈ $4.95" instead of "$4.95"
  → This silently tells users the dollar amount may be off
```

### `cmd/root.go` — New flags

```
--refresh-pricing  bool   "Force re-fetch pricing from OpenRouter"
--no-fetch-pricing  bool   "Skip network fetch, use embedded pricing only"

On startup:
  if !flags.NoFetchPricing {
      if flags.RefreshPricing {
          source.RefreshPricing(httpClient)
      } else {
          source.InitPricing(httpClient)
      }
  }
```

New: add `CostApproximate bool` to `analyze.WasteSignal`:
```go
type WasteSignal struct {
    // ...existing...
    CostApproximate bool  // from TokenEvent, propagated through sessionAgg
}
```

In `sessionAgg`, track `costApproximate` (OR of all events in the session).

---

## Test Requirements

1. **`source/pricing_fetcher_test.go`**:
   - Fetch from live OpenRouter API (integration test, skip in `-short` mode)
   - Returns non-empty pricing for known models (deepseek, claude-sonnet)
   - Cache save/load round-trip preserves all fields
   - TTL check: fresh (1h ago, 24h TTL), stale (48h ago, 24h TTL)
   - Empty cache file → returns nil error, empty entries
   - HTTP error → returns error, doesn't panic

2. **`source/pricing_test.go`**:
   - Lookup: exact match in fetched pricing (add test fetcher that injects mock entries)
   - Lookup: substring match ("claude-sonnet-4-6" matches "claude-sonnet-4-5")
   - Lookup: substring match — multiple candidates, picks longest key
   - Lookup: fallback when no match → approximate=true
   - Lookup: empty fetchedPricing → falls back to embedded table
   - DeepSeek actual pricing: $0.435/MTok input, $0.87/MTok output → verify against known OpenRouter values
   - All existing pricing tests continue to pass (adjust expected values for `approximate` return)

3. **`output/text_test.go`**:
   - CostApproximate=true → "≈ $4.95" in output
   - CostApproximate=false → "$4.95" (no prefix)
   - Golden file updates for changed dollar amounts and `≈` prefixes

4. Coverage target: >=90% on new code

---

## Approach

1. Write `pricing_fetcher.go` with tests (lighter, independent of existing code)
2. Write cache logic with round-trip test
3. Modify `pricing.go`: add `InitPricing`, modify `CostForModel` signature
4. Propagate `CostApproximate` through `TokenEvent` → `sessionAgg` → `WasteSignal`
5. Update `output/text.go` for `≈` display
6. Update `cmd/root.go` for new flags
7. Regenerate golden files
8. Full test suite + lint

---

## Exit Criteria

- [ ] Pull latest main
- [ ] Create feature branch from main: `git checkout -b pr11-dynamic-pricing`
- [ ] Lint passes (zero warnings) — `golangci-lint run`
- [ ] Build compiles cleanly — `go build -o burnwatch .`
- [ ] Tests pass with >=90% coverage on new code — `go test ./... -cover`
- [ ] Self-review: run through [docs/code-review.md](../code-review.md)
- [ ] Document learnings in `docs/learnings.md`
- [ ] Commit: `feat: dynamic pricing from OpenRouter API with cache`
- [ ] Push to branch `pr11-dynamic-pricing`
- [ ] Open pull request
- [ ] Perform code review
- [ ] Update `docs/plans/progress.md` to reflect merge
- [ ] Merge to main
- [ ] Delete feature branch after merge

---

## Notes

- OpenRouter API response is one long JSON line (~430KB). Parse with `json.NewDecoder` streaming, not `json.Unmarshal`.
- The existing pricing table must remain as fallback — don't delete the `pricing` slice.
- OpenRouter pricing is per-token (e.g. `"prompt":"0.000003"`). Convert to per-1K tokens by multiplying by 1000 to match the existing unit.
- Models in JSONL data have provider prefixes (e.g. `vercel/deepseek/deepseek-v4-pro`). Strip prefixes via `splitKey` pattern: everything after last `/`.
- Do NOT add new dependencies to `go.mod`. Use stdlib `net/http`, `encoding/json`, `os`, `time`.
- `CostApproximate` must be added to `analyze.WasteSignal` (hidden coupling: `waste.go`, `text.go`, `json.go`).
