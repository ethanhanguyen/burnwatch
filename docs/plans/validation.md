# Validation Plan

## Test Pyramid

```
        ┌──────────────┐
        │  E2E smoke   │  ← 1 test: full pipeline against testdata, verify output
        ├──────────────┤
        │  Golden      │  ← 2 tests: text output matches .txt golden, JSON matches .json golden
        ├──────────────┤
        │ Integration  │  ← 4 tests: each source against its testdata, analysis against constructed events
        ├──────────────┤
        │  Unit        │  ← 20+ tests: per-function table-driven tests, edge cases, error handling
        └──────────────┘
```

## Unit Tests

### `source/` package

| Test file | What it tests | Key cases |
|-----------|--------------|-----------|
| `pricing_test.go` | `CostForModel()` | Known models, unknown fallback, zero tokens, large numbers |
| `opencode_test.go` | JSON blob parsing, DB query integration | Valid entry, missing tokens, corrupt JSON, zero assistant messages |
| `claude_test.go` | JSONL parsing, subagent discovery, cost computation | Valid entry, non-assistant types, missing usage, malformed line, empty file |

### `analyze/` package

| Test file | What it tests | Key cases |
|-----------|--------------|-----------|
| `baseline_test.go` | `ComputeBaselines()` | 2 projects × 3 sessions, single session, all identical, empty input, zero input |
| `waste_test.go` | `DetectWaste()` | Each heuristic fires independently, no false positives, empty input |
| `subagent_test.go` | `BuildSubagentTree()` | Parent+2 children, no subagents, multi-level nesting, orphan subagent |
| `recommend_test.go` | `GenerateRecommendations()` | Each signal type → correct text, multiple signals, empty signals |

### `output/` package

| Test file | What it tests | Key cases |
|-----------|--------------|-----------|
| `text_test.go` | Text formatter | Golden file match, empty events, zero cost, negative savings |
| `json_test.go` | JSON formatter | Golden file match, valid JSON schema, empty events |

## Integration Tests

### `source/opencode_test.go`
Connect to `testdata/opencode_sample.db`. Stream events. Verify:
- Event count matches expected.
- First and last events have all required fields populated.
- Subagent events have `IsSubagent == true`, `ParentSessionID != ""`.
- Cost > 0 for all events.

### `source/claude_test.go`
Parse `testdata/claude_sample.jsonl` and `testdata/claude_subagents/`. Verify:
- Event count matches expected.
- Model, tokens, cost correctness.
- Subagent cost computed via pricing, not pre-stored.
- Project name derived from directory name.

### `analyze/*_test.go`
Construct `TokenEvent` slices simulating real scenarios. Feed through pipeline. Verify:
- Known-wasteful event sets trigger the correct signals.
- Known-clean event sets trigger no signals.
- Edge cases (single event, zero tokens, negative tokens) don't panic.

## Golden File Tests

### Setup
1. Run the full pipeline against `testdata/opencode_sample.db`.
2. Capture text output → save as `testdata/expected_report.txt`.
3. Capture JSON output → save as `testdata/expected_report.json`.
4. Commit both to repo.

### Test
```go
func TestGoldenText(t *testing.T) {
    events := collectTestEvents(t)
    // ... full pipeline ...
    output := FormatText(events, baselines, signals, trees, recs)
    
    expected, _ := os.ReadFile("testdata/expected_report.txt")
    if output != string(expected) {
        t.Errorf("golden file mismatch. Run with -update flag to regenerate.")
    }
}
```

## E2E Smoke Test

```go
func TestEndToEnd(t *testing.T) {
    // 1. Discover sources (at least one should exist in test env)
    sources := Discover()
    if len(sources) == 0 {
        t.Skip("no harnesses available")
    }

    // 2. Collect events from all sources
    var allEvents []TokenEvent
    for _, src := range sources {
        events, errs := src.Events()
        for e := range events {
            allEvents = append(allEvents, e)
        }
        // errors are non-fatal
        for range errs {}
    }

    // 3. Run full pipeline
    baselines := ComputeBaselines(allEvents)
    signals := DetectWaste(allEvents, baselines)
    trees := BuildSubagentTree(allEvents)
    recs := GenerateRecommendations(signals, baselines)

    // 4. Smoke: at least one signal from known test data
    if len(signals) == 0 {
        t.Fatal("expected waste signals from test data")
    }

    // 5. Verify no partial data (all projects have baselines)
    if len(baselines) == 0 {
        t.Fatal("expected baselines from test data")
    }
}
```

## Flag Review (Manual)

After the tool runs against real data:
1. Select 10 flagged sessions at random.
2. Manually inspect each session:
   - Was the session genuinely wasteful? (agent loops, unnecessary reads, excessive subagent delegation)
   - Or was it a legitimate expensive task? (complex multi-file refactor, deep research)
3. Compute precision: `genuine_waste / total_flagged`.
4. Target: ≥70% precision. If <50%, recalibrate thresholds.

## CI Gate

```yaml
name: CI
on: [push, pull_request]
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with: { go-version: '1.22' }
      - run: go test ./... -cover -coverprofile=coverage.out
      - run: go tool cover -func=coverage.out | tail -1  # total coverage
      - run: go vet ./...
      - uses: golangci/golangci-lint-action@v6
        with: { version: latest }
      - run: go build -o burnwatch .
      - run: ./burnwatch --help  # binary is functional
```

## Coverage Targets

| Package | Target |
|---------|--------|
| `source/pricing.go` | ≥90% |
| `source/opencode.go` | ≥90% |
| `source/claude.go` | ≥90% |
| `analyze/baseline.go` | ≥90% |
| `analyze/waste.go` | ≥90% |
| `analyze/subagent.go` | ≥90% |
| `analyze/recommend.go` | ≥90% |
| `output/text.go` | ≥90% |
| `output/json.go` | ≥90% |
| **Total** | **≥90%** |
