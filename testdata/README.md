# Test Data

Data used by the test suite. Each subdirectory and file has a specific purpose.

## Directory structure

```
testdata/
├── README.md                    # this file
├── opencode_sample.db           # SQLite DB with actual OpenCode session data
├── opencode_part_migration.sql  # Migration script: adds part table for v3 tool calls
├── claude_projects/             # Claude Code session fixtures (JSONL + subagents)
│   └── sample-project/          # Contains tool_use content blocks (v3 format)
├── expected_report.txt          # golden file: full text report output
├── expected_report.json         # golden file: full JSON report output
├── scenarios/                   # crafted scenarios for E2E heuristic testing
│   ├── cost_outlier.jsonl       # 6 sessions, 1 at 80x baseline cost
│   ├── input_overconsumption.jsonl # 6 sessions, 1 at 500x input mean
│   ├── output_explosion.jsonl   # 6 sessions, 1 at 500x output mean
│   ├── low_token_efficiency.jsonl # 6 sessions, 1 at TER < P10
│   ├── fragmentation.jsonl      # 12 sessions over 2 days, 1 churned day
│   ├── subagent_overhead.jsonl  # 2 sessions + subagent tree
│   ├── cache_underutilized.jsonl # 6 sessions, 1 with write-only cache
│   ├── all_clean.jsonl          # 10 identical sessions, zero waste expected
│   ├── multi_signal.jsonl       # 6 sessions, 1 triggers multiple heuristics
│   ├── tool_loop.jsonl          # 3 sessions, 1 with Read/Edit loop (H10, v3)
│   ├── tool_loop_edge.jsonl      # 3 sessions, 1 at threshold=5, 1 no-repeat, 1 normal (H10 edge)
│   ├── file_reread.jsonl        # 3 sessions, 1 with uncached re-reads (H11, v3)
│   ├── file_reread_mixed.jsonl   # 3 sessions: 1 waste, 1 below-threshold, 1 cached (H11 edges)
│   ├── subagent_overlap.jsonl   # 4 sessions, parent+subagent 50% overlap (H12, v3)
│   ├── subagent_overlap_multi.jsonl # 5 sessions: 2 subagents, one overlaps 75%, other 0% (H12 edges)
│   ├── session_restart.jsonl    # 4 sessions, A→B 80% overlap flagged (H13, v3)
│   └── session_restart_chain.jsonl # 4 sessions: A→B→C chain, only B flagged (H13 edges)
├── labels/                      # labeled sessions for signal quality benchmarking
│   ├── labels.jsonl             # labeled sessions
│   └── README.md                # labeling guide
└── benchmarks/                  # baseline performance data for benchstat
```

## Key files

### Golden files

`expected_report.txt` and `expected_report.json` are the source of truth for output format. They are regenerated with:

```bash
go test ./output -run "TestGolden" -update
```

The golden files are compared against actual output in CI. Any change to output format requires regenerating these files.

### Scenarios

Each scenario JSONL file is a crafted Claude Code session log. The files follow the same JSONL format as the Claude fixture — one JSON object per line, with `type: "assistant"` entries containing `model`, `usage`, and optionally `content` with `tool_use` blocks.

**v1/v2 scenarios** (H1–H9): Basic format with `model` + `usage` only. No content blocks.

**v3 scenarios** (H10–H13): Extended format with `message.content` containing `tool_use` blocks. Required for behavioral heuristics that need tool call sequences and file operation data.

Scenario file naming convention:
- `<heuristic_name>.jsonl` — exercises one heuristic
- `*_edge.jsonl` — boundary cases (threshold, below-threshold, chains)
- `*_mixed.jsonl` — mixed waste/normal scenarios within one file
- `all_clean.jsonl` — no waste expected (negative test)
- `multi_signal.jsonl` — multiple heuristics fire (overlap test)

Scenario file format: Token Event Format (TEF). See [`docs/specs/scenario-tef-format.md`](../docs/specs/scenario-tef-format.md) for the full spec and harness-agnostic design.

Session ID convention within scenarios:
- `ses_<scenario>_waste` — the session that SHOULD trigger the heuristic
- `ses_<scenario>_normal_NN` — sessions that should NOT be flagged

## Adding a scenario

1. Create `testdata/scenarios/<name>.jsonl` with ≥6 sessions
2. Add a `TestScenario_<Name>` function in `output/scenario_test.go`
3. Update `testdata/labels/labels.jsonl` with verdicts for new sessions
4. Run: `go test ./output -run "TestScenario" -v`

## Golden file policy

- Golden files track the combined text/JSON output of ALL testdata (OpenCode DB + Claude JSONL)
- Regenerate when output format changes: `go test ./output -run "TestGolden" -update`
- Commit both `expected_report.txt` and `expected_report.json` when regenerated
- Scenario tests do NOT depend on golden files — they assert specific signal properties
