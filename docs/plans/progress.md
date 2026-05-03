# Progress Tracker

> Read this on session start to understand current state.

## Overall: 10/10 PRs complete (v1), 1/8 PRs complete (v2)

```
v1 ████████████████████████████████████████ 10/10 merged
v2 ████░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░ 1/8

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
PR12 ····················· Token Baselines
PR13 ····················· Token-Based Heuristics
PR14 ····················· Config-Wired Thresholds
PR15 ····················· Calibration Mode
PR16 ····················· Unsupervised Anomaly Detection
PR17 ····················· LLM Verification
PR18 ····················· ML Pipeline (experimental)
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
| PR12 | `pr12-token-baselines` | **not started** | — | — | InputMean, OutputMean, TER, token percentiles |
| PR13 | `pr13-token-heuristics` | **not started** | — | — | H6–H9: input, output, TER, fragmentation index |
| PR14 | `pr14-config-wiring` | **not started** | — | — | Wire all thresholds to config, complete PR7 work |
| PR15 | `pr15-calibrate` | **not started** | — | — | --calibrate mode, distribution, suggestions |
| PR16 | `pr16-anomaly-detection` | **not started** | — | — | Isolation forest on session feature vectors |
| PR17 | `pr17-llm-verification` | **not started** | — | — | LLM review of top-N waste signals |
| PR18 | `pr18-ml-pipeline` | **not started** | — | — | Supervised logistic regression (experimental) |

## Blockers

*None.*

## Dependency graph (v2)

```
Phase A (Foundation)
  PR11 ─── PR12 ─── PR13 ─── PR14
  pricing   token    token    config
  fetch     baselines heuristics wiring

Phase B (Calibration + Advanced)
  PR15 ─── PR16 ─── PR17
  calibrate anomaly  LLM verify
  (|| PR14)

Phase C (ML — experimental)
  PR18
  supervised
```

## Next action

Start PR12: Token Baselines. Adds input/output token baselines (mean, std, percentiles) as prerequisite for token-based heuristics. See `docs/plans/PR12-prompt.md`.

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

## Quality snapshot

| Metric | Target | Current |
|--------|--------|---------|
| Test coverage | ≥90% (new code) | 87.4% overall, >90% on new code |
| `go vet` | 0 warnings | 0 |
| `golangci-lint` | 0 issues | 0 |
| Binary builds | pass | pass |
| Golden files match | pass | pass |

## Post-implementation (v1)

All v1 PRs merged. Tasks:
1. [x] Tag `v1.0.0`
2. [x] Archive PR prompts and implementation plan → `docs/archive/`
3. [x] Update `docs/index.md` to reflect current state
4. [ ] Ship: `go install github.com/ethanhanguyen/burnwatch@v1.0.0`

## V2 tasks

- [x] PR11: Fix pricing (OpenRouter) — highest impact, fixes wrong dollar amounts
- [ ] PR12: Token baselines — prerequisite for token heuristics
- [ ] PR13: Token heuristics — H6–H9
- [ ] PR14: Config wiring — complete PR7's unfinished work
- [ ] PR15: Calibration mode
- [ ] PR16: Anomaly detection
- [ ] PR17: LLM verification
- [ ] PR18: ML pipeline
- [ ] Tag `v2.0.0`
