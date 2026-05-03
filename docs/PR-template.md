# PR<N>: <Category> — <Components>

## Objective

<!-- 1-2 sentences. What outcome does this PR achieve? No implementation details — just the result. -->

## Success Criteria

<!-- Each criterion must be verifiable — confirmable true/false by a test, log, or observable output. -->
- [ ] ...

## Dependencies

- **Must merge first:** <!-- e.g. PR1 -->
- **External dependencies:** <!-- e.g. `go get github.com/foo/bar` -->
- **Can be parallel with:** <!-- e.g. PR2, PR3 -->
- **Breaking changes / Migrations needed:** <!-- if any -->

---

## Pre-flight

- [ ] Pull latest main: `git fetch origin && git checkout main && git pull origin main`
- [ ] Create feature branch: `git checkout -b pr<N>-<description>`
- [ ] Verify build environment works on clean main

## Files

| File | Purpose | Notes |
|------|---------|-------|
| `path/to/file` | What it does |  |

---

## Implementation

<!--
  One subsection per component or file.
  Include: types, function signatures, algorithms, constraints, error handling.
  Use code blocks for schemas, interfaces, queries.
  Be specific — the agent should implement without guessing.
-->

### `<component-name>`

```
<type signatures, interfaces, SQL queries, JSON schemas>
```

**Constraints:**
- ...

**Error handling:**
- Case → action (skip / emit error / fail)

---

## Test Requirements

<!--
  Specific test cases, not just "write tests."
  Include: edge cases, table-driven patterns, coverage target.
-->

1. **`<file>_test.go`**:
   - Case → expected result
   - Edge case → expected behavior
2. Coverage target: >=90% on new code

---

## E2E Scenario Tests

<!-- Required for PRs that add/modify heuristics, output formats, or detection logic -->

Scenario tests verify each feature in isolation against crafted test data. Every new heuristic needs a scenario that triggers it and only it.

1. **Scenario file**: `testdata/scenarios/<name>.jsonl`
   - Claude-format JSONL (one JSON object per line, `type: "assistant"`)
   - ≥6 sessions for sigma-based heuristics (≥3 for churn-based), ≥2 for subagent
   - Exactly ONE session triggers the target heuristic; all others are within normal range
   - Session IDs follow convention: `ses_<scenario>_waste`, `ses_<scenario>_normal_NN`

2. **Scenario test**: `output/scenario_test.go`
   - `TestScenario_<Name>` loads scenario, runs pipeline, asserts:
     - Waste session IS flagged with expected reason
     - Normal sessions are NOT flagged
   - Use `findSignalByID()` and iterate for multiple signals per session
   - Update `testdata/labels/labels.jsonl` for new scenario sessions

3. **Golden file** updated only when output format changes (not when adding scenarios)

## Benchmarking

<!-- Required for PRs that add algorithms or change data paths -->

- [ ] `Benchmark<Feature>` added to package's `bench_test.go`
- [ ] Benchmark uses deterministic data (fixed seed `rand.NewSource(42)`)
- [ ] No regression >20% vs baseline. Run: `go test -bench=<Feature> -benchmem -count=3`

## Signal Quality

<!-- Required for PRs that add/modify heuristics -->

- [ ] Labels file updated for new scenario sessions (`testdata/labels/labels.jsonl`)
- [ ] `go test ./output -bench=SignalQuality` shows no regression in precision/recall/F1

---

## Approach

<!-- Step-by-step execution order. TDD recommended. -->

1. Write tests first (RED)
2. Implement minimal code (GREEN)
3. Refactor + verify coverage
4. Run full test suite

---

## Exit Criteria

- [ ] Pull latest main
- [ ] Create feature branch from main: `git checkout -b <branch>`
- [ ] Lint passes (zero warnings) — see `AGENTS.md` for project commands
- [ ] Build compiles cleanly — see `AGENTS.md` for project commands
- [ ] Tests pass with required coverage — see `AGENTS.md` for project commands
- [ ] Self-review: run through [docs/code-review.md](../code-review.md)
- [ ] Document learnings (gotchas, mistakes, patterns, hidden coupling) in `docs/learnings.md`
- [ ] Commit: `<type>: <description>`
- [ ] Push to branch `<branch>`
- [ ] Open pull request with description
- [ ] Perform code review
- [ ] Update `docs/plans/progress.md` to reflect merge
- [ ] Merge to main
- [ ] Delete feature branch after merge

---

## Notes

<!-- Gotchas, "do NOT do X", conventions to follow, reminders -->
