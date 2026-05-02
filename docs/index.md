# Docs Index

Start here. This file stays short on purpose.

## Current docs

- [quickstart.md](./quickstart.md) — install, first run, interpret output, pipe to jq
- [architecture.md](./architecture.md) — module boundaries, data flow, design decisions
- [contributing.md](./contributing.md) — local setup, testing, adding new harnesses

## Rules

- Treat this file as the docs entrypoint for new coding sessions.
- Keep top-level `docs/` limited to current docs only.
- Move superseded plans and specs into [`archive/`](./archive/).
- When a spec is no longer current, archive it instead of leaving it at the top level.

## Current specs

- [specs/burnwatch-v1.md](./specs/burnwatch-v1.md) — v1 specification: data model, 5 heuristics, output formats, CLI flags
- [specs/source-interface.md](./specs/source-interface.md) — `Source` interface contract, `TokenEvent` schema, how to add a harness

## Recent decisions

### Architecture (2026-05-02)
- [decisions/2026-05-02-source-abstraction.md](./decisions/2026-05-02-source-abstraction.md) — why TokenEvent + Source interface over per-harness code
- [decisions/2026-05-02-statistical-thresholds.md](./decisions/2026-05-02-statistical-thresholds.md) — why P95/P10/2σ over hardcoded constants

## Plans

- [plans/progress.md](./plans/progress.md) — **read this on session start** — PR status, blockers, next action
- [plans/implementation.md](./plans/implementation.md) — PR dependency graph, file map, quality gates
- [plans/validation.md](./plans/validation.md) — test pyramid, golden files, smoke tests, CI gate

## Archive

- [archive/](./archive/) — archived PR prompts, implementation plan
