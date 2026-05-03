# Test Data

Data used by the test suite. Each subdirectory and file has a specific purpose.

## Directory structure

```
testdata/
├── README.md                    # this file
├── opencode_sample.db           # SQLite DB with actual OpenCode session data
├── claude_sample.jsonl          # 7-line JSONL of a Claude Code session
├── claude_subagents/            # subagent JSONL for the Claude session above
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
│   └── multi_signal.jsonl       # 6 sessions, 1 triggers multiple heuristics
├── labels/                      # labeled sessions for signal quality benchmarking
│   ├── labels.jsonl             # 54 labeled sessions (14 waste, 40 clean)
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

Each scenario JSONL file is a crafted Claude Code session log. The files follow the same JSONL format as `claude_sample.jsonl` — one JSON object per line, with `type: "assistant"` entries containing `model` and `usage` fields.

Scenario file naming convention:
- `<heuristic_name>.jsonl` — exercises one heuristic
- `all_clean.jsonl` — no waste expected (negative test)
- `multi_signal.jsonl` — multiple heuristics fire (overlap test)

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
