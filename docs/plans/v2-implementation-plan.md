# Burnwatch v2 Implementation Plan

> **Status:** Planning | **Date:** 2026-05-02 | **Dependency:** PR1–PR10 (all merged)

## Motivation

The burnwatch v1 assessment revealed three root problems:

1. **Cost calculations are wrong.** The embedded pricing table (`source/pricing.go`) covers only 6 models. All others (deepseek, kimi, minimax, gpt-5.4, qwen — the majority of user sessions) fall back to claude-sonnet-4-5 pricing ($3/$15 per MTok). DeepSeek is actually ~$0.44/$0.87 — costs are inflated **7x–17x**. This makes dollar-based heuristics and the potential-savings headline meaningless.

2. **Token data is collected but unused by heuristics.** `TokenEvent` has separate `InputTokens`/`OutputTokens` since PR1. `sessionAgg` sums them independently. But every heuristic operates on cost or ratio — never on absolute input or output token counts. This is squandering data that is already cleanly plumbed through every layer.

3. **Heuristic thresholds are hardcoded.** Only `cost_outlier_sigma` is configurable. `LowSignalPercentile` exists in config but isn't wired. Subagent overhead (50%), churn min sessions (3), P10 percentiles — all unchanging constants. A project with 3,000 sessions gets the same thresholds as one with 30.

## Phases

```
Phase A (Foundation)
  PR11 ─── PR12 ─── PR13 ─── PR14
  pricing   token    token    config
  fetch     baselines heuristics wiring

Phase B (Calibration + Advanced)
  PR15 ─── PR16 ─── PR17
  calibrate anomaly  LLM verify

Phase C (ML — experimental, post-v2)
  PR18
  supervised
```

### Dependency graph

```
PR11 (pricing)     ── no deps, can start immediately
PR12 (baselines)   ── after PR11 (uses new cost data for cross-validation)
PR13 (heuristics)  ── after PR12 (needs token baselines)
PR14 (config)      ── after PR13 (needs heuristic toggles defined)

PR15 (calibrate)   ── after PR13 (prints distributions for all heuristics)
                    ── parallel with PR14

PR16 (anomaly)     ── after PR14+PR15 (needs config-wired thresholds, calibration context)
PR17 (LLM)         ── after PR16 (uses anomaly scores to pick top-N for review)

PR18 (ML)          ── after PR17 (needs labeled data from LLM verification)
```

### Parallel opportunities

| Group | PRs | Rationale |
|-------|-----|-----------|
| G1 | PR14, PR15 | Different files, no shared state |
| G2 | PR16, PR17 | Different packages, PR17 depends on PR16 output format |

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

### PR15: Calibration Mode

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

### PR16: Unsupervised Anomaly Detection

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

### PR17: LLM Verification

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

**Why:** Reduces false positives on the most impactful signals. $0.40 to verify $800K in potential savings is worth it. Builds training labels for PR18.

**Files affected:** `analyze/llm_verify.go` (new), `analyze/llm_verify_test.go` (new), `analyze/waste.go` (integration), `cmd/root.go` (flags), `config/config.go` (llm section)

### PR18: ML Pipeline (Experimental)

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

**Feature extraction** (same vector as PR16):
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

54 sessions labeled: 14 waste, 40 clean. Bootstrapped from scenario data. Grows via manual CLI labels (PR18) and LLM verification (PR17).

## Total Scope

| Metric | Count |
|--------|-------|
| PRs | 8 (PR11–PR18) |
| New test files | 2 (`output/scenario_test.go`, `output/bench_test.go`) |
| Scenario files | 9 (`testdata/scenarios/*.jsonl`) |
| Labels | 54 (`testdata/labels/labels.jsonl`) |
| New packages | 2 (`analyze/anomaly.go`, `analyze/ml_*.go`) |
| New CLI flags | ~12 |
| New config fields | ~15 |
| External dependencies | 0 (all pure Go, no new deps) |
| LLM costs (PR17) | $0.40/run (optional, opt-in) |
| Estimated lines of code | ~2,500–3,000 net new (incl. tests + scenarios) |
