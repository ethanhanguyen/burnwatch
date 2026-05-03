# Progress Tracker

> Read this on session start to understand current state.

## Overall: 10/10 PRs complete (v1), 4/4 complete (v2), 3/6 complete (v2.5→v3)

```
v1    ████████████████████████████████████████ 10/10 merged
v2    ████████████████████████████████████████ 4/4 merged
v2.5  ████████████████████████████████████████ 2/2 (critical fixes)
v3    █████████░░░░░░░░░░░░░░░░░░░░░░░░░░░░░ 1/4 (features)

PR1  ██████████████████████ Foundation
PR2  ██████████████████████ OpenCode Source
PR3  ██████████████████████ Claude Code Source
PR4  ██████████████████████ Analysis Engine
PR5  ██████████████████████ CLI + Output + Wiring
PR6  ██████████████████████ Docs + CI + Release
PR7  ██████████████████████ Config File
PR8  ██████████████████████ Phase A — Display Fixes
PR9  ██████████████████████ Phase B — Noise Reduction
PR10 ██████████████████████ Phase C — Deeper Insights

PR11 ██████████████████████ Dynamic Pricing (OpenRouter)
PR12 ██████████████████████ Token Baselines
PR13 ██████████████████████ Token-Based Heuristics
PR14 ██████████████████████ Config-Wired Thresholds

PR15 ██████████████████████ Fix Pricing + Uncosted
PR16 ██████████████████████ Output Quality Fixes

PR17 ██████████████████████ Calibration Mode
PR18 ····················· Unsupervised Anomaly Detection
PR19 ····················· LLM Verification
PR20 ····················· ML Pipeline (experimental)
```

## PR Status

| PR | Branch | Status | Started | Merged | Notes |
|----|--------|--------|---------|--------|-------|
| PR1 | `pr1-foundation` | merged | 2026-05-02 | 2026-05-02 | TokenEvent, interface, pricing |
| PR2 | `pr2-opencode-source` | merged | 2026-05-02 | 2026-05-02 | SQLite reader |
| PR3 | `pr3-claude-source` | merged | 2026-05-02 | 2026-05-02 | JSONL reader + subagents |
| PR4 | `pr4-analysis-engine` | merged | 2026-05-02 | 2026-05-02 | Baselines + 5 heuristics + recommendations |
| PR5 | `pr5-cli-output` | merged | 2026-05-02 | 2026-05-02 | CLI + text/JSON + golden files + E2E |
| PR6 | `pr6-docs-ci` | merged | 2026-05-02 | 2026-05-02 | Docs + README + CHANGELOG + CI |
| PR7 | `pr7-config` | merged | 2026-05-02 | 2026-05-02 | Config file: TOML, thresholds, filters, toggles |
| PR8 | `pr8-display-fixes` | merged | 2026-05-02 | 2026-05-02 | A1 label fix, A2 churn grouping, A3 dates |
| PR9 | `pr9-noise-reduction` | merged | 2026-05-02 | 2026-05-02 | B1 min-cost, B2 dedup, B3 sigma, B4 cost fix |
| PR10 | `pr10-deeper-insights` | merged | 2026-05-02 | 2026-05-02 | C1 model/tokens, C2 signal toggles, C3 trends |
| PR11 | `pr11-dynamic-pricing` | merged | 2026-05-02 | 2026-05-02 | OpenRouter fetch, cache, fallback, fix wrong $ |
| PR12 | `pr12-token-baselines` | merged | 2026-05-02 | 2026-05-02 | InputMean, OutputMean, TER, token percentiles |
| PR13 | `pr13-token-heuristics` | merged | 2026-05-03 | 2026-05-03 | H6 input overconsumption, H7 output explosion, H8 token efficiency, H9 fragmentation index |
| PR14 | `pr14-config-wiring` | merged | 2026-05-03 | 2026-05-03 | Wire all thresholds to config, new CLI flags, complete PR7 work |
| PR15 | `pr15-pricing-fix` | **merged** | 2026-05-03 | 2026-05-03 | 1000x embedded bug, cache validation, uncosted fallback |
| PR16 | `pr15-pricing-fix` | **merged** | 2026-05-03 | 2026-05-03 | Fragment min-cost gating, savings dedup, --init |
| PR17 | `pr17-calibrate` | **merged** | 2026-05-03 | 2026-05-03 | --calibrate mode, distribution, suggestions |
| PR18 | `pr18-anomaly-detection` | **not started** | — | — | Isolation forest on session feature vectors |
| PR19 | `pr19-llm-verification` | **not started** | — | — | LLM review of top-N waste signals |
| PR20 | `pr20-ml-pipeline` | **not started** | — | — | Supervised logistic regression (experimental) |

## Blockers

**PR15 (critical):** Embedded pricing table uses $/MTok values but treated as $/1K — all costs inflated 1000x. $963K reported should be ~$963. All analysis and downstream PRs blocked until fixed.

## Dependency graph

```
Phase 0 (Critical Fix — sequential)
  PR15 ─── PR16
  pricing   output-quality
  fix       (noise + dedup + init)

Phase 1 (v2 Features — sequential)
  PR17 ─── PR18 ─── PR19
  calibrate anomaly  llm-verify

Phase 2 (Experimental)
  PR20
  supervised
```

## Validation gates

Each milestone requires a gate check before the next PR can start.

### Gate P15 (after PR15 merge)

- [x] **P15.1 — Cost accuracy:** Run on real data (`burnwatch -harness all -days 30`). Spot-check 3 Claude sessions against OpenRouter billing dashboard. Variance <5%.
- [x] **P15.2 — Cache health:** `burnwatch --refresh-pricing` writes `pricing.json` with >500 entries. `burnwatch --no-fetch-pricing` falls back to embedded table without 1000x inflation.
- [x] **P15.3 — Graceful degradation:** Simulate pricing fetch failure. Verify uncosted sessions show `$?`, no fallback price used.

### Gate P16 (after PR16 merge)

- [x] **P16.1 — Fragment noise:** Run on real data. Fragmentation signals <100 (was 1,900+). No sub-$0.50 session flagged by H9.
- [x] **P16.2 — Savings honesty:** "Potential savings" ≤ sum of all session costs. Same session flagged by 3 heuristics is counted once.
- [x] **P16.3 — Init:** `burnwatch --init` in tmpdir → `.burnwatch.toml` loads without errors.

### Gate P17 (after PR17 merge)

- [x] **P17.1 — Output compact:** `burnwatch --calibrate` prints <80 lines, readable on a terminal.
- [x] **P17.2 — Suggestions valid:** Copy suggested thresholds to `.burnwatch.toml`. Re-run `burnwatch`. Verify same metrics produce same signals (no breakage).

### Gate P18 (after PR18 merge)

- [ ] **P18.1 — Isolation correct:** Known-outlier test: 95 normal + 5 outliers → all 5 score >0.6.
- [ ] **P18.2 — No false positives:** Identical data → all scores ≈0.5 (no isolation possible, no false anomaly).
- [ ] **P18.3 — Performance:** 1000 sessions × 100 trees <200ms.

### Gate P19 (after PR19 merge)

- [ ] **P19.1 — Parse reliability:** Manual test with real API key. All verdicts parse correctly (WASTE/NOT_WASTE/UNKNOWN).
- [ ] **P19.2 — Cost estimate:** Estimate before confirm is accurate within 50%.

### Gate P20 (after PR20 merge)

- [ ] **P20.1 — F1 score:** Precision + recall on held-out labeled data >0.70.

## Next action

Start PR18: Unsupervised Anomaly Detection. See `docs/plans/PR18-prompt.md`.

## Execution log

| Date | PR | Action |
|------|----|--------|
| 2026-05-02 | — | Implementation plan and PR prompts written (PR1-6) |
| 2026-05-02 | PR1 | Foundation (no-op, already in place) |
| 2026-05-02 | PR2 | OpenCode source implemented, reviewed, merged |
| 2026-05-02 | PR3 | Claude Code source implemented, reviewed, merged |
| 2026-05-02 | PR4 | Analysis engine implemented, reviewed, merged |
| 2026-05-02 | PR5 | CLI + Output + Wiring implemented, reviewed, merged |
| 2026-05-02 | PR6 | Docs + CI + Release implemented, reviewed, merged |
| 2026-05-02 | PR7 | Config file implemented, reviewed, merged |
| 2026-05-02 | PR8 | Display fixes implemented, reviewed, merged |
| 2026-05-02 | PR9 | Noise reduction implemented, reviewed, merged |
| 2026-05-02 | PR10 | Deeper insights implemented, reviewed, merged |
| 2026-05-02 | — | Burnwatch assessment complete. Identified 3 root problems. |
| 2026-05-02 | — | V2 implementation plan and PR11–PR18 prompts written. |
| 2026-05-02 | PR11 | Dynamic pricing: OpenRouter API, 7-day cache, CostApproximate propagation, ≈ indicator |
| 2026-05-02 | PR12 | Token baselines: InputMean, InputStd, InputP50/P90, OutputMean, OutputStd, OutputP50/P90, TERP10, raw arrays |
| 2026-05-03 | PR13 | Token heuristics: H6 input overconsumption, H7 output explosion, H8 token efficiency, H9 fragmentation index (replaces H5) |
| 2026-05-03 | PR14 | Config-wired thresholds: all hardcoded constants moved to config, new CLI flags, signature changes, full test coverage |
| 2026-05-03 | — | **Post-PR14 review found 1000x pricing bug.** Embedded table uses $/MTok values but CostForModel treats as $/1K. All costs inflated. Cache corrupted (1 fake entry). OpenCode trusts DB costs instead of recalculating. Fallback price fabricates costs for unknown models. |
| 2026-05-03 | — | v2.5 plan drafted: PR15 (pricing fix), PR16 (output quality). Original PR15-18 renumbered to PR17-20. Validation gates added at each milestone. |
| 2026-05-03 | PR15 | Fix embedded pricing 1000x, remove fallback, add CostUnknown, gate cost heuristics |
| 2026-05-03 | PR16 | Fragment min-cost gating, savings dedup, --init flag, config.example.toml |
| 2026-05-03 | PR17 | Calibration mode: distribution analysis, threshold suggestions, text+JSON output |

## Quality snapshot

| Metric | Target | Current |
|--------|--------|---------|
| Test coverage | ≥90% (new code) | 84.1% overall |
| `go vet` | 0 warnings | 0 |
| `golangci-lint` | 0 issues | 0 |
| Binary builds | pass | pass |
| Golden files match | pass | pass (will break in PR15 — costs drop 1000x) |
