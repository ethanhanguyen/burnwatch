# Progress Tracker

> Read this on session start to understand current state.

## Overall: 5/6 PRs complete (PR6 in progress)

```
PR1 ██████████████████████ Foundation
PR2 ██████████████████████ OpenCode Source
PR3 ██████████████████████ Claude Code Source
PR4 ██████████████████████ Analysis Engine
PR5 ██████████████████████ CLI + Output + Wiring
PR6 ███████████████████░░ Docs + CI + Release
```

## PR Status

| PR | Branch | Status | Started | Merged | Notes |
|----|--------|--------|---------|--------|-------|
| PR1 | `pr1-foundation` | merged | 2026-05-02 | 2026-05-02 | TokenEvent, interface, pricing |
| PR2 | `pr2-opencode-source` | merged | 2026-05-02 | 2026-05-02 | SQLite reader |
| PR3 | `pr3-claude-source` | merged | 2026-05-02 | 2026-05-02 | JSONL reader + subagents |
| PR4 | `pr4-analysis-engine` | merged | 2026-05-02 | 2026-05-02 | Baselines + 5 heuristics + recommendations |
| PR5 | `pr5-cli-output` | **merged** | 2026-05-02 | 2026-05-02 | CLI + text/JSON + golden files + E2E |
| PR6 | `pr6-docs-ci` | in progress | 2026-05-02 | — | Docs + README + CHANGELOG + CI |

## Blockers

*None yet.*

## Next action

Execute [`PR6-prompt.md`](./PR6-prompt.md) — Docs + CI + Release.

## Execution log

| Date | PR | Action |
|------|----|--------|
| 2026-05-02 | — | Implementation plan and PR prompts written |
| 2026-05-02 | PR2 | OpenCode source implemented, reviewed, merged |
| 2026-05-02 | PR3 | Claude Code source implemented, reviewed, merged |
| 2026-05-02 | PR4 | Analysis engine implemented, reviewed, merged |
| 2026-05-02 | PR5 | CLI + Output + Wiring implemented, reviewed, merged |

## Quality snapshot

| Metric | Target | Current |
|--------|--------|---------|
| Test coverage | ≥90% | 93.1% |
| `go vet` | 0 warnings | 0 |
| `golangci-lint` | 0 issues | 0 |
| Binary builds | pass | pass |
| Golden files match | pass | pass |

## Post-implementation

When all 6 PRs are merged:
1. Tag `v1.0.0`
2. Archive PR prompts and implementation plan → `docs/archive/`
3. Update `docs/index.md` to reflect current state (remove plans, add to archive)
4. Ship: `go install github.com/yourname/burnwatch@v1.0.0`
