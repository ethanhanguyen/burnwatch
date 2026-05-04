# Burnwatch v2.5 Implementation Plan

> **Status:** Phase 0 (critical fixes) not started | **Date:** 2026-05-03 | **Dependency:** PR1–PR14 (all merged)

## Motivation

The burnwatch v1 assessment revealed three root problems. PR11-14 addressed #2 and #3 partially. The 2026-05-03 post-PR14 review discovered new critical bugs:

1. **Cost calculations are wrong (1000x bug).** The embedded pricing table (`source/pricing.go`) stores $/MTok values but `CostForModel()` divides tokens by 1000 (treating as $/1K). All Claude sessions inflated 1000x. Real cost ~$963, burnwatch reported $963K. *Partially fixed by PR11 (OpenRouter fetch), fully fixed by PR15.*

2. **Pricing cache corrupted.** `~/Library/Caches/burnwatch/pricing.json` has 1 fake entry (`model-1`) instead of 500+ OpenRouter models. All non-Claude/Gemini models fall back to embedded table → 1000x inflation. *Fixed by PR15 (cache validation).*

3. **OpenCode trusts database costs.** OpenCode source uses `td.Cost` from the message JSON instead of recalculating via `CostForModel()`. If OpenCode's own pricing is wrong, burnwatch propagates wrong costs. *Fixed by PR15 (unified cost path).*

4. **Fallback price fabricates authority.** Unknown models get claude-sonnet pricing as a guess. A fabricated dollar sign on a guess is worse than silence. *Fixed by PR15 (uncosted fallback).*

5. **Fragmentation noise.** H9 fires on 1,900+ sub-$0.05 sessions (OpenCode's many-API-call per task pattern). *Fixed by PR16 (min-cost gating).*

6. **Savings double-counting.** Same session flagged by H1, H6, H7, H8 — each claiming independent savings. Potential savings sum overshoots actual waste by ~3x. *Fixed by PR16 (dedup).*

7. **Token data is collected but unused by heuristics.** `TokenEvent` has separate `InputTokens`/`OutputTokens` since PR1. `sessionAgg` sums them independently. But every heuristic operates on cost or ratio — never on absolute input or output token counts. *Fixed by PR13.*

8. **Heuristic thresholds are hardcoded.** *Fixed by PR14.*

## Phases

```
Phase A (Foundation) — merged
  PR11 ─── PR12 ─── PR13 ─── PR14
  pricing   token    token    config
  fetch     baselines heuristics wiring

Phase 0 (Critical Fix — sequential)
  PR15 ─── PR16
  pricing   output-quality
  fix       (noise + dedup + init)

Phase 1 (v2 Features — sequential)
  PR17 ─── PR18 ─── PR19
  calibrate anomaly  LLM verify
  (was PR15) (was PR16) (was PR17)

Phase 2 (ML — experimental)
  PR20
  supervised
  (was PR18)
```

### Dependency graph

```
PR11 (pricing)     ── no deps, merged
PR12 (baselines)   ── after PR11, merged
PR13 (heuristics)  ── after PR12, merged
PR14 (config)      ── after PR13, merged

PR15 (pricing fix) ── after PR14 (needs config for uncosted toggle)
                   ── blocks ALL analysis (1000x inflated costs)

PR16 (output)      ── after PR15 (needs corrected cost data)

PR17 (calibrate)   ── after PR16 (was PR15; needs config init, clean output)
PR18 (anomaly)     ── after PR14+PR17 (was PR16; needs config-wired thresholds)
PR19 (LLM)         ── after PR18 (was PR17; uses anomaly scores)
PR20 (ML)          ── after PR19 (was PR18; needs labeled data)
```

### Parallel opportunities

None in Phase 0–1. All PRs are sequential. PR15 blocks all analysis. PR16 needs PR15 costs. PR17-20 depend on clean output from PR16.

---

## PR Overview

### PR11: Dynamic Pricing (OpenRouter)

**What:** Fetch model pricing from OpenRouter API on first run, cache to `~/.cache/burnwatch/pricing.json` (7-day TTL), use per-token pricing instead of hardcoded table. Add `--refresh-pricing` flag. Display `≈` indicator when fallback pricing is used.

**Why:** Fixes dollar amounts for deepseek, kimi, minimax, gpt-5.4, qwen. Makes cost-based heuristics trustworthy. Enables meaningful total-savings headline.

**Files affected:** `source/pricing.go`, `source/pricing_fetcher.go` (new), `output/text.go` (≈ indicator), `cmd/root.go` (new flags)

### PR12: Token Baselines

**What:** Extend `Baseline` struct with `InputMean`, `InputStd`, `InputP50`, `InputP90`, `OutputMean`, `OutputStd`, `OutputP50`, `OutputP90`, `TERP10` (token efficiency ratio). Compute in `buildBaseline`. No heuristic changes — just making the data available.

**Why:** Prerequisite for all token-based heuristics. Without these, PR13 has nowhere to get thresholds.

**Files affected:** `analyze/baseline.go`, `analyze/baseline_test.go`

### PR13: Token-Based Heuristics

**What:** Four new heuristics using token baselines from PR12:

- **H6 Input overconsumption:** `inputSum > bl.InputMean + sigma*bl.InputStd` — catches sessions reading 5x+ more context than project peers. HIGH severity.
- **H7 Output explosion:** `outputSum > bl.OutputMean + sigma*bl.OutputStd` — catches runaway generation loops. MEDIUM severity.
- **H8 Token efficiency ratio:** `(outputSum + cacheRead) / (inputSum + cacheWrite) < bl.TERP10` — low "useful work per token." LOW severity.
- **H9 Fragmentation index:** `N_day * (1 − mean_day_ratio)` groups sessions per project/day, flags > threshold. Replaces current binary H5 churn. MEDIUM severity.

Each heuristic gets a config toggle in `SignalToggles`, a configurable sigma/percentile, and follows the existing `check*()` pattern in `waste.go`.

**Why:** H6–H7 catch waste that cost-based H1 misses (cheap model + massive context = no $ flag but real waste). H8 is a better signal than H2 (ratio alone can't distinguish "reading docs" from "spinning in loops"). H9 replaces H5's binary all-below-mean check with a weighted index that surfaces genuinely problematic days.

**Files affected:** `analyze/waste.go`, `analyze/waste_test.go`, `output/text.go`, `output/text_test.go`, `output/json.go`, `testdata/expected_report.txt`

### PR14: Config-Wired Thresholds

**What:** Replace remaining hardcoded constants with configurable values loaded from `.burnwatch.toml`:

| Was hardcoded | Config key | Default |
|---|---|---|
| `LowSignalPercentile = 10.0` (not wired) | `thresholds.low_signal_percentile` | `10.0` |
| `Cache P10 = 10` | `thresholds.cache_percentile` | `10.0` |
| `Subagent overhead > 50%` | `thresholds.subagent_overhead_pct` | `50.0` |
| `Churn min sessions = 3` | `thresholds.churn_min_sessions` | `3` |
| `Churn threshold = 2` | `thresholds.churn_threshold` | `2` |
| `Deduplicate = false` | Already configurable | — |

Add entries for PR13 heuristics:
| Config key | Default |
|---|---|
| `thresholds.input_overconsumption_sigma` | `2.0` |
| `thresholds.output_explosion_sigma` | `2.0` |
| `thresholds.token_efficiency_percentile` | `10.0` |
| `thresholds.fragmentation_index_threshold` | `3.0` |
| `signals.input_overconsumption` | `true` |
| `signals.output_explosion` | `true` |
| `signals.token_efficiency` | `true` |
| `signals.fragmentation_index` | `true` |

**Why:** Users with 3,000 sessions have different distributions than users with 30. Let them tune. Also completes the config wiring that PR7 started but left partial.

**Files affected:** `config/config.go`, `config/config_test.go`, `analyze/waste.go`, `analyze/baseline.go`, `cmd/root.go`

### PR15: Fix Pricing + Uncosted Fallback

**What:** Fix 3 pricing bugs + add uncosted handling:

- **A. 1000x embedded pricing bug:** Divide all 6 embedded `priceEntry` values by 1000. Was `{3.00, 15.00, ...}` (accidental $/MTok), should be `{0.003, 0.015, ...}` ($/1K, matching fetched pricing format).
- **B. Cache validation:** `LoadCache` treats <50 entries as stale. Re-fetch from OpenRouter. Fixes the single fake `"model-1"` entry corruption.
- **C. OpenCode cost source:** Replace `CostUSD: td.Cost` with `CostForModel()` call — same calculation path as Claude source.
- **D. Uncosted fallback:** Remove `fallback` variable. Unknown models get `CostUnknown=true`. Cost-based heuristics (H1/H3/H9) skip. Token heuristics (H2/H4/H6/H7/H8) still fire.
- **E. Uncosted display:** `$?` and `[no pricing data]` in text output. `cost_unknown` field in JSON.

**Why:** The 1000x bug makes all dollar-based analysis meaningless. Cache corruption silently inflates non-Claude model costs. Fabricating prices for unknown models is worse than silence. Uncork the blocks for all downstream PRs.

**Files affected:** `source/pricing.go`, `source/pricing_fetcher.go`, `source/opencode.go`, `source/event.go`, `source/claude.go`, `analyze/waste.go`, `output/text.go`, `output/json.go`, all `*_test.go` (golden file updates)

### PR16: Output Quality Fixes

**What:** Three output-quality improvements:

- **Fragmentation noise suppression:** Config key `thresholds.fragmentation_min_cost = 0.50`. Sessions below this cost are skipped by H9. Fixes 1,900+ noise signals from sub-$0.05 OpenCode sessions.
- **Savings deduplication:** Same session flagged by multiple heuristics is counted once in the "Potential savings" summary. Savings capped at session cost.
- **Config init:** `burnwatch --init` writes default `.burnwatch.toml`. `config.example.toml` shipped in repo. `--init` refuses to overwrite existing config.

**Why:** Makes output usable. Without min-cost gating, OpenCode users get flooded by fragmentation noise. Without dedup, the "Potential savings" number is misleading. Without `--init`, new users have no config file.

**Files affected:** `config/config.go`, `analyze/waste.go`, `output/text.go`, `output/json.go`, `cmd/root.go`, `config.example.toml` (new)

### PR17: Calibration Mode (was PR15)

**What:** `--calibrate` flag that prints the full statistical distribution of every metric and suggests threshold values:

```
$ burnwatch --calibrate
Your data: 908 main sessions, 1255 subagent sessions across 10 projects
Baseline period: 2026-04-10 to 2026-05-02

Session costs ($):
  Mean=$1172.37  σ=$5231.44
  P50=$0.47  P75=$1.84  P90=$8.21  P95=$27.50  P99=$2844.49

Input tokens:
  Mean=187K  σ=752K
  P50=12K  P75=45K  P90=187K  P95=520K  P99=2.9M

Output tokens:
  Mean=42K  σ=156K
  P50=3.2K  P75=12K  P90=41K  P95=92K  P99=306K

Output/input ratio:
  Mean=0.52  P10=0.02  P50=0.31  P90=1.82

Cache hit rate (%):
  P10=52.1  P50=74.3  P75=87.6

Token efficiency ratio:
  P10=0.08  P50=0.47  P90=1.91

Subagent overhead (%):
  Mean=64.2  P50=72.1  P75=84.3  P90=92.5

Suggested thresholds (for .burnwatch.toml):
  [thresholds]
  cost_outlier_sigma = 2.5          # flags top 1.2% by cost
  input_overconsumption_sigma = 2.5 # flags top 1.2% by input tokens
  output_explosion_sigma = 2.5      # flags top 1.2% by output tokens
  subagent_overhead_pct = 75        # current 50% catches 68% of sessions
  low_signal_percentile = 5.0       # stricter: only bottom 5% of ratios
  token_efficiency_percentile = 5.0 # bottom 5% TER
```

Also supports `--calibrate --json` for machine-readable output.

**Why:** Makes the data model of "statistically calibrated" actually calibrated. The user should see their own distributions and decide thresholds informed by data, not guess.

**Files affected:** `analyze/calibrate.go` (new), `analyze/calibrate_test.go` (new), `output/calibrate_text.go` (new), `output/calibrate_json.go` (new), `cmd/root.go` (new flags)

### PR18: Unsupervised Anomaly Detection (was PR16)

**What:** Complement the threshold-based heuristics with an Isolation Forest that flags multi-dimensional outliers.

**Feature vector per session:**
```go
[inputSum, outputSum, ratio, cacheRate, cost, subagentOverheadPct, sessionCountToday, modelPriceTier, isWeekend, inputPerEvent]
```

**Algorithm:** Isolation Forest — build 100 trees, each tree isolates points with random splits on random features. Anomaly score = average path length. Sessions that are easy to isolate (require few splits) are anomalous.

**Integration:** Anomaly scores supplement existing heuristics. Sessions with anomaly score > threshold AND not already flagged by any heuristic get a new signal (severity: MEDIUM, reason: "anomaly"). Session IDs already flagged get an `anomaly_score` field added to their WasteSignal but no duplicate.

**Isolation Forest in Go:** Implement from scratch (no external ML library needed — the algorithm is ~150 lines). Reasons:
- No Go-native ML libraries mature enough for isolation forest
- The algorithm is simple: random feature, random split, count path length
- Avoids dependency bloat

**Why:** Catches combinations that single-threshold heuristics miss. A session with "medium input, low ratio, weekend, expensive model" may not trip any single heuristic but is anomalous in the joint distribution.

**Files affected:** `analyze/anomaly.go` (new), `analyze/anomaly_test.go` (new), `analyze/waste.go` (integration), `config/config.go` (toggle + threshold), `cmd/root.go` (flag)

### PR19: LLM Verification (was PR17)

**What:** For the top-20 highest-cost waste signals, optionally call an LLM to verify whether the session represents genuine waste and diagnose the root cause.

**Prompt construction per signal:**
```
This AI agent session consumed {inputTokens} input tokens and produced
{outputTokens} output tokens. The model was {model}. Cost: ${cost}.
The session used {subagentCount} subagents ({overheadPct}% overhead).
Project: {project}. Day: {date}. {sessionCountToday} sessions that day.

Is this likely wasteful agent behavior? If so, what specific pattern
(loop, context bloat, over-delegation, model mismatch, false positive)?
Reply: WASTE|NOT_WASTE|<reason>
```

**API:** Uses OpenRouter API (same pricing source) with a cheap model (haiku, $0.80/$4.00). No additional API key — just an optional `--llm-verify` flag that also requires `--llm-key <key>`.

**Cost estimate:** ~$0.02 per verification. 20 signals × $0.02 = $0.40 per full run. Optional, opt-in.

**Output:** Adds verification result to WasteSignal. In text output: `[LLM: WASTE — context bloat from repeated file reads]`. In JSON: `"llm_verification": {"verdict": "waste", "reason": "..."}`.

**Rate limiting and safety:**
- Max 20 verifications per run (configurable, default 20, max 50)
- Requires explicit `--llm-verify --llm-key <key>` (never reads key from env/files)
- Prints estimated cost before firing, requires `--llm-confirm` flag

**Why:** Reduces false positives on the most impactful signals. $0.40 to verify top signals is worth it. Builds training labels for PR20.

**Files affected:** `analyze/llm_verify.go` (new), `analyze/llm_verify_test.go` (new), `analyze/waste.go` (integration), `cmd/root.go` (flags), `config/config.go` (llm section)

### PR20: ML Pipeline — Experimental (was PR18)

**What:** Supervised machine learning pipeline that learns waste detection from user-labeled sessions.

**Label collection:**
```bash
burnwatch --label ses_abc123 waste      # mark session as waste
burnwatch --label ses_def456 not_waste  # mark session as not waste
burnwatch --labels                      # list all labeled sessions
```

Labels stored in `~/.cache/burnwatch/labels.jsonl`:
```json
{"session_id":"ses_abc123","label":"waste","reason":"loop","timestamp":"2026-05-02T..."}
```

**Feature extraction** (same vector as PR18):
```
[inputSum, outputSum, ratio, cacheRate, cost, subagentOverheadPct, 
 sessionCountToday, modelPriceTier, isWeekend, inputPerEvent]
```

**Classifier:** Logistic regression (pure Go, no external dependency). Simple, interpretable, gives `P(waste)` probability. Trained on labeled data.

**Integration:** When ≥20 labels exist, the classifier produces `P(waste)` scores. Sessions with `P(waste) > 0.7` and not already flagged get a new signal.

**Training trigger:** Manual via `burnwatch --train`. Retrains from scratch on all labels. Prints precision/recall/F1.

**Why:** The heuristics are a starting point. Over time, the user's own judgment (what they label as waste) should drive detection. This closes the feedback loop.

**Files affected:** `analyze/ml_label.go` (new), `analyze/ml_classifier.go` (new), `analyze/ml_test.go` (new), `cmd/root.go` (new commands), `config/config.go` (toggle)

---

## Config Schema (final)

```toml
[thresholds]
cost_outlier_sigma = 2.0
low_signal_percentile = 10.0
cache_percentile = 10.0
subagent_overhead_pct = 50.0
churn_min_sessions = 3
churn_threshold = 2
input_overconsumption_sigma = 2.0
output_explosion_sigma = 2.0
token_efficiency_percentile = 10.0
fragmentation_index_threshold = 3.0
anomaly_threshold = 0.6

[signals]
cost_outlier = true
low_signal = true
subagent_overhead = true
cache_underutilized = true
session_churn = true
input_overconsumption = true
output_explosion = true
token_efficiency = true
fragmentation_index = true
anomaly = false

[pricing]
cache_ttl_days = 7
openrouter_url = "https://openrouter.ai/api/v1/models"

[llm]
model = "anthropic/claude-haiku-4.5"
max_verifications = 20
api_url = "https://openrouter.ai/api/v1/chat/completions"

[ml]
enabled = false
min_labels = 20

[filters]
min_cost = 0
deduplicate = false

[output]
group_churn = false
show_trends = false
```

---

## Exit Criteria (per PR)

Each PR follows `docs/PR-template.md`:
- [ ] Lint: `golangci-lint run` — zero warnings
- [ ] Build: `go build -o burnwatch .` — clean
- [ ] Unit test: `go test ./... -cover` — ≥90% on new code
- [ ] Scenario tests: `go test ./output -run "TestScenario" -v` — all pass
- [ ] Benchmarks: `go test ./output -bench=. -benchmem` — no regression >20%
- [ ] Signal quality: `go test ./output -bench=SignalQuality` — no F1 regression
- [ ] Review: `docs/code-review.md` — all checks pass
- [ ] Docs: learnings captured in `docs/learnings.md`
- [ ] Commit: conventional commit (`feat:`, `fix:`)

## Testing Infrastructure

### E2E scenario tests (`output/scenario_test.go`)

Each scenario is a crafted Claude-format JSONL file in `testdata/scenarios/`. The test harness:
1. Loads a scenario JSONL
2. Parses it into TokenEvents
3. Runs the full pipeline (ComputeBaselines → DetectWaste)
4. Asserts specific sessions are/aren't flagged with expected reasons

| Scenario | Sessions | Target heuristic | Status |
|----------|----------|------------------|--------|
| `cost_outlier.jsonl` | 6 | H1 (cost outlier) | **Implemented** |
| `input_overconsumption.jsonl` | 6 | H6 (PR13) | **Implemented** |
| `output_explosion.jsonl` | 6 | H7 (PR13) | **Implemented** |
| `low_token_efficiency.jsonl` | 6 | H8 (PR13) | **Implemented** |
| `fragmentation.jsonl` | 12 | H9 (PR13) | **Implemented** |
| `subagent_overhead.jsonl` | 2 | H3 (subagent overhead) | **Implemented** |
| `cache_underutilized.jsonl` | 6 | H4 (cache underutilized) | **Implemented** |
| `all_clean.jsonl` | 10 | No waste expected | **Implemented** |
| `multi_signal.jsonl` | 6 | Multi-heuristic | **Implemented** |

### Benchmarks (`output/bench_test.go`)

| Benchmark | Measures |
|-----------|----------|
| `BenchmarkPipeline_1K` | Full pipeline on 1K synthetic sessions |
| `BenchmarkPipeline_10K` | Full pipeline on 10K synthetic sessions |
| `BenchmarkBaselineComputation` | Baseline stats on 5K sessions |
| `BenchmarkWasteDetection` | Waste detection on 5K sessions |
| `BenchmarkPricingLookup` | CostForModel across 6 models |
| `BenchmarkBuildSubagentTree` | Subagent tree building on 100 parents |
| `BenchmarkSignalQuality` | Precision/recall/F1 on labeled data |

### Labels (`testdata/labels/labels.jsonl`)

54 sessions labeled: 14 waste, 40 clean. Bootstrapped from scenario data. Grows via manual CLI labels (PR20) and LLM verification (PR19).

## Total Scope

| Metric | Count |
|--------|-------|
| PRs | 10 (PR11–PR20) |
| New test files | 2 (`output/scenario_test.go`, `output/bench_test.go`) |
| Scenario files | 9 (`testdata/scenarios/*.jsonl`) |
| Labels | 54 (`testdata/labels/labels.jsonl`) |
| New packages | 2 (`analyze/anomaly.go`, `analyze/ml_*.go`) |
| New CLI flags | ~12 |
| New config fields | ~15 |
| External dependencies | 0 (all pure Go, no new deps) |
| LLM costs (PR19) | $0.40/run (optional, opt-in) |
| Estimated lines of code | ~2,500–3,000 net new (incl. tests + scenarios) |
