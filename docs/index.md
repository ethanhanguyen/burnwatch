# Docs Index

Start here. This file stays short on purpose.

## Current docs

- [quickstart.md](./quickstart.md) — install, first run, interpret output, pipe to jq
- [architecture.md](./architecture.md) — module boundaries, data flow, design decisions
- [contributing.md](./contributing.md) — local setup, testing, adding new harnesses
- [code-review.md](./code-review.md) — 3-phase checklist: automated, diff inspection, code patterns, behavioral gates

## Rules

- Treat this file as the docs entrypoint for new coding sessions.
- Keep top-level `docs/` limited to current docs only.
- Move superseded plans and specs into [`archive/`](./archive/).
- When a spec is no longer current, archive it instead of leaving it at the top level.

## Current specs

- [specs/burnwatch-v1.md](./specs/burnwatch-v1.md) — v1 specification: data model, 5 heuristics, output formats, CLI flags
- [specs/source-interface.md](./specs/source-interface.md) — `Source` interface contract, `TokenEvent` schema, how to add a harness

## Implementation plans

- [plans/v2-implementation-plan.md](./plans/v2-implementation-plan.md) — v2 roadmap: pricing, token heuristics, calibration, anomaly detection, ML

## Active PR prompts (v2.5→v3)

| PR | Prompt | Description |
|----|--------|-------------|
| ~~PR11~~ | [plans/PR11-prompt.md](./plans/PR11-prompt.md) | ~~Dynamic Pricing — OpenRouter API + cache~~ merged |
| ~~PR12~~ | [plans/PR12-prompt.md](./plans/PR12-prompt.md) | ~~Token Baselines — input/output mean, std, percentiles~~ merged |
| ~~PR13~~ | [plans/PR13-prompt.md](./plans/PR13-prompt.md) | ~~Token-Based Heuristics — H6–H9~~ merged |
| ~~PR14~~ | [plans/PR14-prompt.md](./plans/PR14-prompt.md) | ~~Config-Wired Thresholds~~ merged |
| **PR15** | [plans/PR15-prompt.md](./plans/PR15-prompt.md) | **Fix Pricing + Uncosted — 1000x bug, cache validation, fallback removal** |
| **PR16** | [plans/PR16-prompt.md](./plans/PR16-prompt.md) | **Output Quality — fragment noise, savings dedup, config init** |
| ~~PR17~~ | [plans/PR17-prompt.md](./plans/PR17-prompt.md) | ~~Calibration Mode (was PR15)~~ merged |
| PR18 | [plans/PR18-prompt.md](./plans/PR18-prompt.md) | Unsupervised Anomaly Detection (was PR16) |
| PR19 | [plans/PR19-prompt.md](./plans/PR19-prompt.md) | LLM Verification (was PR17) |
| PR20 | [plans/PR20-prompt.md](./plans/PR20-prompt.md) | ML Pipeline — experimental (was PR18) |

## Recent decisions

### Architecture (2026-05-02)
- [decisions/2026-05-02-source-abstraction.md](./decisions/2026-05-02-source-abstraction.md) — why TokenEvent + Source interface over per-harness code
- [decisions/2026-05-02-statistical-thresholds.md](./decisions/2026-05-02-statistical-thresholds.md) — why P95/P10/2σ over hardcoded constants

### V2 direction (2026-05-02)
- [decisions/2026-05-02-v2-assessment.md](./decisions/2026-05-02-v2-assessment.md) — assessment findings: wrong pricing, unused token data, hardcoded thresholds
- V2 plan: see [plans/v2-implementation-plan.md](./plans/v2-implementation-plan.md)

## Progress

- [plans/progress.md](./plans/progress.md) — **read this on session start** — PR status, dependency graph, quality snapshot

## Learnings

- [learnings.md](./learnings.md) — **read this on session start** — accumulated repo knowledge, past mistakes, gotchas

## Archive

- [archive/](./archive/) — archived PR prompts (PR1–PR10), implementation plan, validation strategy
