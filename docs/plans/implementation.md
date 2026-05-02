# Burnwatch Implementation Plan

Single-binary Go tool that reads session data from AI agent harnesses (OpenCode, Claude Code) and produces a statistically-calibrated waste report.

## PR Dependency Graph

```
PR1: Foundation (TokenEvent + Interface + Pricing)
 │
 ├── PR2: OpenCode Source ─────────────┐
 │                                      │
 ├── PR3: Claude Code Source ───────────┤  ← all three parallel after PR1
 │                                      │
 └── PR4: Analysis Engine ──────────────┘
                                        │
                                 PR5: CLI + Output + Wiring + Integration Tests
                                        │
                                 PR6: Docs + README + CHANGELOG + CI + Archive
```

- PR2, PR3, PR4 can be built in parallel after PR1 merges.
- PR5 serializes — it wires the full pipeline and runs integration tests against real data.
- PR6 is the polish pass — docs, changelog, CI guardrails.

## PR Summary

| PR | Scope | ~LOC | Key deliverable |
|----|-------|------|-----------------|
| PR1 | Foundation | 120 | `TokenEvent` struct, `Source` interface, embedded pricing table |
| PR2 | OpenCode Source | 150 | SQLite reader → `TokenEvent` stream, tests with anonymized DB |
| PR3 | Claude Code Source | 180 | JSONL reader → `TokenEvent` stream, subagent file discovery, tests |
| PR4 | Analysis Engine | 300 | Baseline computation, 5 waste heuristics, subagent cost tree, recommendations |
| PR5 | CLI + Output | 250 | `main.go`, `cmd/`, `text.go`, `json.go`, golden file tests, e2e smoke test |
| PR6 | Docs + CI | 200 | All docs, README, CHANGELOG, golangci-lint config, archive historical files |

**Total ~1200 LOC**

## File Map

```
burnwatch/
├── main.go                       # PR5
├── go.mod                        # PR1 (init)
│
├── source/
│   ├── event.go                  # PR1: TokenEvent struct
│   ├── interface.go              # PR1: Source interface + Discover()
│   ├── opencode.go               # PR2: OpenCode SQLite reader
│   ├── opencode_test.go          # PR2
│   ├── claude.go                 # PR3: Claude Code JSONL reader
│   ├── claude_test.go            # PR3
│   └── pricing.go                # PR1: embedded pricing table
│
├── analyze/
│   ├── baseline.go               # PR4: μ, σ, percentiles
│   ├── baseline_test.go          # PR4
│   ├── waste.go                  # PR4: outlier detection
│   ├── waste_test.go             # PR4
│   ├── subagent.go               # PR4: subagent cost tree
│   ├── subagent_test.go          # PR4
│   ├── recommend.go              # PR4: waste → recommendation text
│   └── recommend_test.go         # PR4
│
├── output/
│   ├── text.go                   # PR5: human-readable report
│   ├── text_test.go              # PR5
│   ├── json.go                   # PR5: machine-readable output
│   └── json_test.go              # PR5
│
├── cmd/
│   └── root.go                   # PR5: CLI flags + dispatch
│
├── testdata/
│   ├── opencode_sample.db        # PR2: anonymized 10-session test DB
│   ├── claude_sample.jsonl       # PR3: anonymized test session
│   ├── expected_report.txt       # PR5: golden file
│   └── expected_report.json      # PR5: golden file
│
└── docs/
    ├── index.md                  # PR6
    ├── quickstart.md             # PR6
    ├── architecture.md           # PR6
    ├── contributing.md           # PR6
    ├── specs/
    │   ├── burnwatch-v1.md       # PR6
    │   └── source-interface.md   # PR6
    ├── decisions/
    │   ├── 2026-05-02-statistical-thresholds.md   # PR6
    │   └── 2026-05-02-source-abstraction.md       # PR6
    └── plans/
        ├── implementation.md     # This file (PR6 — archive after merged)
        ├── PR1-prompt.md         # PR6 (archive after executed)
        ├── PR2-prompt.md         # PR6
        ├── PR3-prompt.md         # PR6
        ├── PR4-prompt.md         # PR6
        ├── PR5-prompt.md         # PR6
        ├── PR6-prompt.md         # PR6
        └── validation.md         # PR6
```

## Execution Order

1. Execute PR1 → merge
2. Execute PR2, PR3, PR4 in parallel → merge each
3. Execute PR5 → merge
4. Execute PR6 → merge → tag `v1.0.0`

## Quality Gates

Every PR must pass:
- `go test ./... -cover -coverprofile=coverage.out` (≥90% coverage for new code)
- `go vet ./...` (zero warnings)
- `golangci-lint run` (zero issues)
- No uncommitted generated files
