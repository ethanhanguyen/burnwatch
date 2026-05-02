# Repo Learnings

<!--
  Curated knowledge reference. Read on session start to avoid past mistakes.
  4 categories — use only those that apply. Every bullet cites a concrete file/function/symptom.
  "In general" entries are invalid. See Rules section for maintenance protocol.
-->

## Overview

This file is a curated knowledge reference, not a chronological PR log. Its purpose is to prevent AI agents and human contributors from repeating known mistakes. Structured by category to stay compact and searchable regardless of how many PRs have passed.

## Categories

| Category | Prompt |
|----------|--------|
| **Gotcha** | Something not obvious from reading the code. Surprising behavior, hidden constraints. |
| **Mistake** | What went wrong + root cause + how to prevent next time. |
| **Pattern** | An idiom or convention worth repeating. |
| **Hidden coupling** | Touching X means you must also update Y. |

---

## Gotchas

- `pricing.go` uses a `[]struct{...}` slice, not `map[string]priceEntry`, because model matching is substring-based (`strings.Contains`) and requires ordered iteration. A map would hide ambiguous matches silently — the slice ensures first-match-wins is explicit and testable. `source/pricing.go:12-22`
- `modernc.org/sqlite` v1.50.0 was incompatible with Go 1.22. The latest pure-Go SQLite isn't always compatible — pin to a version that matches the project's Go toolchain. `go.mod:5`
- Claude Code subagent entries: both `SessionID` and `ParentSessionID` of the resulting `TokenEvent` are the parent session UUID. The subagent's identity is in the `agentId` field, not a unique session ID. `source/claude.go:196-201`
- Go runs tests from the package directory, not the project root. All test data paths must use `filepath.Join("..", "testdata", ...)`. `source/*_test.go`
- With N ≤ 5 sessions and a single cost outlier, the 2σ threshold (population stddev) can never trip because the max z-score for one outlier is `sqrt(N-1)`. For N=5, max z = 2.0 — exactly equals the threshold, never exceeds it. Use N ≥ 6 for cost outlier test data. `analyze/waste_test.go`
- Global baseline key `"*"` provides cross-project percentile thresholds for H2 (low signal) and H4 (cache underutilized). `splitKey` handles this sentinel by returning `"*"` as project and `""` as harness. `analyze/baseline.go:21,166-173`

## Mistakes

- Initial `go.mod` declared `go 1.26.2` (nonexistent version). Never write a go.mod version without checking `go version` on the actual runtime.
- Unreachable `if cost < 0 { return 0 }` guard in `CostForModel`. Negative tokens are already clamped at input — the negative-cost branch was dead code. Check invariants once at the entry point, not redundantly. `source/pricing.go:26`
- Left an unused `_ any` parameter that survived self-review until golangci-lint caught it. Run `golangci-lint run` before committing, not after.
- Defined `sessionInfo` as a local type inside `BuildSubagentTree` but tried to use it in helper function signatures. Local types defined inside a function can't be referenced from other function signatures — must be package-level. `analyze/subagent.go:25-31`

## Patterns

- Float comparisons in tests: use `math.Abs(got - want) > delta` with `const delta = 0.0001`. Never `==` for float equality. `source/pricing_test.go:9,86`
- Separate test function for defensive guards vs. main table-driven behavior. Keeps table-driven tests focused on normal cases. `source/pricing_test.go`
- Env var override for test path injection: every Source follows the `BURNWATCH_<HARNESS>_<PATH>` convention. See `defaultDBPath()` in `source/opencode.go:177` and `defaultProjectDir()` in `source/claude.go:37-43`.
- Error channels buffered (cap 10), events channels unbuffered — prevents the goroutine from blocking on error sends while the consumer drains events. `source/opencode.go:24-25`
- `projectNameToDisplay()` strips leading `-` and replaces `-` with `/` to convert directory paths to display names. `source/claude.go:220-223`
- Waste signal sorting: severity descending (high → medium → low), then reason alphabetically, then SessionID. Ensures deterministic output ordering before CLI rendering. `analyze/waste.go:122-130`
- Per-session metric aggregation: `map[string]*sessionAgg` groups events by SessionID, then ratios and cache rates are computed from summed token counts. Same pattern in `aggregateMetrics` and `DetectWaste`. `analyze/baseline.go:77`, `analyze/waste.go:50-87`

## Hidden Couplings

- `CostForModel` in `source/pricing.go:26` is the single pricing entry point. Every Source calls it. Changing its signature or the `priceEntry` struct breaks all Sources. When adding fields to `TokenEvent`, check if pricing needs them too.
- Adding a Source touches 3 files: (1) the new `source/<name>.go`, (2) `source/interface.go` `Discover()`, (3) `README.md` Supported Harnesses. `source/interface.go:10-20`
- Test in-memory SQLite DB schemas must match actual harness DB schemas exactly. If a harness changes its schema, tests break silently by failing to create matching tables. `source/opencode_test.go:361-365`
- `Baseline.RatioMean` was added to support H5 (session churn) which needs per-project mean ratios. Not in the original PR4 spec but required — `CostStd` alone doesn't capture output/input behavior. `analyze/baseline.go:7`
- All heuristics depend on `ComputeBaselines` producing a global baseline (key `"*"`). Removing or renaming this key breaks H2, H4, and cross-project percentile comparisons. `analyze/baseline.go:21,65`

## Rules

These govern how this file is maintained. Violating them makes the file less useful over time.

1. **Category-first, not chronological.** Add/edit entries within their category. Never add a new date/PR section.
2. **Merge before adding.** Before adding a new entry, check if a similar one exists. Consolidate instead of duplicating.
3. **Staleness gate.** When adding to a category, review the oldest ~3 entries. If a gotcha/pattern hasn't been relevant for 5+ PRs, remove it.
4. **Cite concrete code.** Every entry must reference a specific file, function, or symptom. "In general" observations are invalid.
5. **Drop forensic detail.** Commit hashes, dates, and PR numbers belong in git history, not here.
