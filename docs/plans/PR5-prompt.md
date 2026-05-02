# PR5: CLI + Output + Wiring + Integration Tests

> **Workflow:** Follow `docs/plans/PR-template.md`. Review `AGENTS.md` behavioral guidelines before implementing.

## Objective

Wire all components together: CLI entrypoint, output formatters (text + JSON), and end-to-end integration tests against real test data. This is where burnwatch becomes a usable binary.

## Files to create

| File | Purpose |
|------|---------|
| `main.go` | Entrypoint — parse flags, discover sources, run pipeline |
| `cmd/root.go` | CLI flag definitions and dispatch logic |
| `output/text.go` | Human-readable text report |
| `output/text_test.go` | Golden file tests for text output |
| `output/json.go` | Machine-readable JSON output |
| `output/json_test.go` | Golden file tests for JSON output |
| `testdata/expected_report.txt` | Golden file — expected text output |
| `testdata/expected_report.json` | Golden file — expected JSON output |

## Pre-flight

- [ ] Pull latest main: `git fetch origin && git checkout main && git pull origin main`
- [ ] Create feature branch: `git checkout -b pr5-cli-output`
- [ ] Verify build environment works on clean main

## Dependencies

PR1, PR2, PR3, PR4 must all be merged. This PR serializes after them.

## `cmd/root.go`

```go
var flags struct {
    DBPath    string
    Harness   string   // "all", "opencode", "claude-code"
    Project   string   // filter to specific project
    JSON      bool
    Days      int      // lookback window in days
    Verbose   bool
}
```

### CLI usage

```
burnwatch [flags]

Flags:
  --db <path>       OpenCode database path (default: ~/.local/share/opencode/opencode.db)
  --harness <name>  Filter to harness: all, opencode, claude-code (default: all)
  --project <name>  Filter to project
  --json            Output as JSON instead of text
  --days <n>        Lookback window in days (default: 1 for today, 7 for week, 30 for month)
  --verbose         Show all events, not just waste signals
```

### Dispatch flow

```
main() → parse flags → Discover() sources
       → collect events from all sources (or filtered by --harness)
       → filter by --project and --days
       → ComputeBaselines(events)
       → DetectWaste(events, baselines)
       → BuildSubagentTree(events)
       → GenerateRecommendations(signals, baselines)
       → if --json: output as JSON
       → else: output as text report
```

## `output/text.go`

Format:

```
$ burnwatch
OpenCode: 1610 sessions, 1089 subagent sessions | Claude Code: 200 sessions
Today: $1.34 (8 sessions) | This week: $18.72 (52 sessions)

Project lilysbeauty: $12.40 (median $1.24/session)
Project reka-travel: $6.32 (median $0.79/session)

Waste signals:
  HIGH ses_xyz Bright-Butterfly (lilysbeauty): $1.86 — 4.2x project median
    → Investigate session for unnecessary loops or re-prompts. Potential savings: $1.24
  MED  ses_abc Clever-Squid (reka-travel): 87% subagent overhead ($0.82 / $0.94)
    → Evaluate whether subagent delegation was necessary vs inline. Potential savings: $0.57
  LOW  Project lilysbeauty: Cache hit rate 12% (P10 = 25%)
    → Consider CLAUDE.md optimization for better caching. Potential savings: $2.48

Summary: 3 waste signals found. Potential savings: $4.29 / week
```

- Use `--verbose` to show ALL sessions, not just waste.
- Truncate long session slugs to 20 chars.
- Right-align costs.
- No colors in v1.

## `output/json.go`

```json
{
  "summary": {
    "opencode_sessions": 1610,
    "opencode_subagent_sessions": 1089,
    "claude_sessions": 200,
    "today_cost": 1.34,
    "today_sessions": 8,
    "week_cost": 18.72,
    "week_sessions": 52
  },
  "projects": [
    {
      "name": "lilysbeauty",
      "harness": "opencode",
      "session_count": 450,
      "total_cost": 12.40,
      "median_cost": 1.24
    }
  ],
  "waste_signals": [
    {
      "session_id": "ses_xyz",
      "project": "lilysbeauty",
      "severity": "high",
      "reason": "cost_outlier",
      "detail": "4.2x project median",
      "metric": 1.86,
      "threshold": 1.24
    }
  ],
  "subagent_trees": [
    {
      "session_id": "ses_abc",
      "total_cost": 0.94,
      "subagent_cost": 0.82,
      "overhead_pct": 87.2,
      "subagents": [
        {
          "session_id": "ses_child1",
          "agent_type": "explore",
          "cost": 0.45,
          "children": []
        }
      ]
    }
  ],
  "recommendations": [
    {
      "signal": "cost_outlier",
      "action": "Investigate session for unnecessary loops",
      "detail": "...",
      "savings_est": 1.24
    }
  ],
  "potential_savings": 4.29
}
```

## Integration tests

### Golden file test (`output/text_test.go` + `output/json_test.go`)

1. Load `testdata/opencode_sample.db` via OpenCode source.
2. Run full pipeline: events → baselines → waste → subagent → recommendations.
3. Format as text and JSON.
4. Compare against `testdata/expected_report.txt` and `testdata/expected_report.json`.
5. Fail if output differs.

### E2E smoke test

Create a test in `cmd/root_test.go`:

```go
func TestEndToEnd(t *testing.T) {
    // Invoke main logic with test DB
    events := collectTestEvents(t)
    baselines := ComputeBaselines(events)
    signals := DetectWaste(events, baselines)

    // Smoke: we get at least one signal from known-bad data
    if len(signals) == 0 {
        t.Fatal("expected waste signals from test data, got none")
    }

    // Verify no panic with --json flag
    // Verify --project filter works
    // Verify --harness filter works
    // Verify --days filter works
}
```

### Coverage

- Test `--json` output format.
- Test `--harness opencode` filtering.
- Test `--project` filtering.
- Test `--verbose` mode.
- Test `--days 7` date filtering.
- Test with zero events → graceful "No data found" message.
- Test with missing DB → error message, not panic.

**Coverage target**: ≥90% on `output/` files. Integration test for `cmd/`.

## Approach: TDD

1. Create `testdata/expected_report.txt` and `expected_report.json` based on the sample DB from PR2 + sample JSONL from PR3.
2. Write output tests against golden files (RED — output isn't generated yet).
3. Implement `output/text.go` and `output/json.go` (GREEN).
4. Write `cmd/root.go` + `main.go`.
5. Run full integration test against testdata.

## Exit criteria

- [ ] Pull latest main
- [ ] Create feature branch from main
- [ ] `go test ./... -cover` passes with ≥90% coverage
- [ ] `go vet ./...` zero warnings
- [ ] `golangci-lint run` zero issues
- [ ] `go build -o burnwatch .` produces a working binary
- [ ] `./burnwatch --db testdata/opencode_sample.db` runs and outputs text
- [ ] `./burnwatch --db testdata/opencode_sample.db --json` outputs valid JSON
- [ ] Golden file tests pass (text and JSON match expected)
- [ ] E2E smoke test passes (at least one waste signal detected)
- [ ] Self-review: follow behavioral guidelines in `AGENTS.md`
- [ ] Document learnings (gotchas, mistakes, patterns, hidden coupling) in `docs/learnings.md`
- [ ] Commit: `feat: wire CLI, text/JSON output, integration tests`
- [ ] Push to branch `pr5-cli-output`
- [ ] Open pull request
- [ ] Perform code review
- [ ] Update `docs/plans/progress.md` to reflect merge
- [ ] Merge to main
- [ ] Delete feature branch after merge

## Notes

- Golden files live in `testdata/` — regenerate with `go test -update` if output format changes.
- No colors in v1 text output. Keep formatting simple and testable.
- CLI flags use stdlib `flag` package — no external CLI framework.
