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
- [specs/scenario-tef-format.md](./specs/scenario-tef-format.md) — Token Event Format (TEF): harness-agnostic scenario JSONL spec

## Deferred PRs

| PR | Description | Reason |
|----|-------------|--------|
| PR18 | Isolation Forest anomaly detection | Multivariate statistical anomaly = same conflation problem. May return as pre-filter. |
| PR19 | LLM verification | More valuable with behavioral signals. Reschedule post-v3. |
| PR20 | Supervised ML pipeline | Behavioral rules are interpretable; explicit > black-box for waste detection. |

## Recent decisions

### Architecture (2026-05-02)
- [decisions/2026-05-02-source-abstraction.md](./decisions/2026-05-02-source-abstraction.md) — why TokenEvent + Source interface over per-harness code
- [decisions/2026-05-02-statistical-thresholds.md](./decisions/2026-05-02-statistical-thresholds.md) — why P95/P10/2σ over hardcoded constants

### V2 direction (2026-05-02)
- [decisions/2026-05-02-v2-assessment.md](./decisions/2026-05-02-v2-assessment.md) — assessment findings: wrong pricing, unused token data, hardcoded thresholds
- V2 plan: see [plans/v2-implementation-plan.md](./plans/v2-implementation-plan.md)

### V3 direction (2026-05-03)
- [decisions/2026-05-03-event-level-waste-detection.md](./decisions/2026-05-03-event-level-waste-detection.md) — **read this** — pivot from cost anomaly detection to event-level behavioral waste detection; why, what changes, what stays

## Progress

- [plans/progress.md](./plans/progress.md) — **read this on session start** — PR status, dependency graph, quality snapshot

## Learnings

- [learnings.md](./learnings.md) — **read this on session start** — accumulated repo knowledge, past mistakes, gotchas

## Archive

- [archive/](./archive/) — archived PR prompts (PR1–PR10), implementation plan, validation strategy
