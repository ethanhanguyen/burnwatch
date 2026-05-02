# Progress Tracker

> Read this on session start to understand current state.

## Overall: 0/6 PRs complete

```
PR1 ░░░░░░░░░░░░░░░░░░░░ Foundation
PR2 ░░░░░░░░░░░░░░░░░░░░ OpenCode Source
PR3 ░░░░░░░░░░░░░░░░░░░░ Claude Code Source
PR4 ░░░░░░░░░░░░░░░░░░░░ Analysis Engine
PR5 ░░░░░░░░░░░░░░░░░░░░ CLI + Output + Wiring
PR6 ░░░░░░░░░░░░░░░░░░░░ Docs + CI + Release
```

## PR Status

| PR | Branch | Status | Started | Merged | Notes |
|----|--------|--------|---------|--------|-------|
| PR1 | `pr1-foundation` | pending | — | — | TokenEvent, interface, pricing |
| PR2 | `pr2-opencode-source` | pending | — | — | SQLite reader |
| PR3 | `pr3-claude-source` | pending | — | — | JSONL reader + subagents |
| PR4 | `pr4-analysis-engine` | pending | — | — | Baselines + 5 heuristics |
| PR5 | `pr5-cli-output` | pending | — | — | CLI + text/JSON + integration |
| PR6 | `pr6-docs-ci` | pending | — | — | Docs + README + CHANGELOG + CI |

## Blockers

*None yet.*

## Next action

Execute [`PR1-prompt.md`](./PR1-prompt.md) — Foundation.

## Execution log

| Date | PR | Action |
|------|----|--------|
| 2026-05-02 | — | Implementation plan and PR prompts written |

## Quality snapshot

| Metric | Target | Current |
|--------|--------|---------|
| Test coverage | ≥90% | — |
| `go vet` | 0 warnings | — |
| `golangci-lint` | 0 issues | — |
| Binary builds | pass | — |
| Golden files match | pass | — |

## Post-implementation

When all 6 PRs are merged:
1. Tag `v1.0.0`
2. Archive PR prompts and implementation plan → `docs/archive/`
3. Update `docs/index.md` to reflect current state (remove plans, add to archive)
4. Ship: `go install github.com/yourname/burnwatch@v1.0.0`
