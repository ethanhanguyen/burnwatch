# Progress Tracker

> Read this on session start to understand current state.

## Overall: 10/10 PRs complete (v1), 4/4 complete (v2), 3/3 complete (v2.5), 1/4 complete (v3)

```
v1    ████████████████████████████████████████ 10/10 merged
v2    ████████████████████████████████████████ 4/4 merged
v2.5  ████████████████████████████████████████ 3/3 merged
v3    ███████████░░░░░░░░░░░░░░░░░░░░░░░░░░░░░ 1/4

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

N1   ██████████████████████ Data Model Expansion
N2   ····················· Loop + Re-read (H10,H11)
N3   ····················· Subagent Overlap + Restart (H12,H13)
N4   ····················· Polish + Calibration

PR18 ····················· Unsupervised Anomaly Detection (DEFERRED)
PR19 ····················· LLM Verification (DEFERRED)
PR20 ····················· ML Pipeline (DEFERRED)
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
| PR18 | `pr18-anomaly-detection` | **deferred** | — | — | See ADR 2026-05-03 |
| PR19 | `pr19-llm-verification` | **deferred** | — | — | Reschedule post-v3 |
| PR20 | `pr20-ml-pipeline` | **deferred** | — | — | Replaced by behavioral detection |
| N1 | `n1-data-model-expansion` | **merged** | 2026-05-03 | 2026-05-03 | ToolCall + FileOp in TokenEvent |
| N2 | `n2-loop-reread` | **planned** | — | — | H10, H11 — loop + file re-read detection |
| N3 | `n3-overlap-restart` | **planned** | — | — | H12, H13 — subagent overlap + session restart |
| N4 | `n4-polish` | **planned** | — | — | Performance, calibration, path normalization |

## Blockers

**PR15 (critical):** Embedded pricing table uses $/MTok values but treated as $/1K — all costs inflated 1000x. $963K reported should be ~$963. All analysis and downstream PRs blocked until fixed.

## Dependency graph

```
v3 (Event-Level Waste Detection — 4 PRs)
  N1 ─── N2
  │       │
  │       └── N4
  │
  └────── N3 ─── N4

Phase 2 (Deferred — post v3)
  N4 ─── PR19 (LLM verification)

Phase 3 (Deferred — unless behavioral analysis proves expensive)
  PR18 (Isolation Forest)
  PR20 (Supervised ML)
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

### Gate P18 (after PR18 merge) — DEFERRED

- [ ] **P18.1 — Isolation correct:** Not applicable. Behavioral detection replaces anomaly detection.

### Gate P19 (after PR19 merge) — DEFERRED (post-v3)

- [ ] **P19.1 — Parse reliability:** Not applicable until behavioral signals ship and accumulate labels.

## Quality snapshot

| Metric | Target | Current |
|--------|--------|---------|
| Test coverage | ≥90% (new code) | 84.1% overall |
| `go vet` | 0 warnings | 0 |
| `golangci-lint` | 0 issues | 0 |
| Binary builds | pass | pass |
| Golden files match | pass | pass |
