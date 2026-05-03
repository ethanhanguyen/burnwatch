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

- `golangci-lint` v2.x config format requires `version: "2"` at the top of `.golangci.yml`. The v1 format (no version field, `linters-settings` section) is silently rejected with "unsupported version of the configuration." Always check `golangci-lint version` before writing config. `.golangci.yml:1`
- `go vet` flags `composite literal uses unkeyed fields` for structs with 5+ fields passed between packages. All cross-package struct literals must use keyed field initialization (`s := SignalToggles{CostOutlier: true, ...}`), not positional (`s := SignalToggles{true, true, ...}`). `analyze/waste.go:26-32`
- `pricing.go` uses a `[]struct{...}` slice, not `map[string]priceEntry`, because model matching is substring-based (`strings.Contains`) and requires ordered iteration. A map would hide ambiguous matches silently — the slice ensures first-match-wins is explicit and testable. `source/pricing.go:12-22`
- `modernc.org/sqlite` v1.50.0 was incompatible with Go 1.22. The latest pure-Go SQLite isn't always compatible — pin to a version that matches the project's Go toolchain. `go.mod:5`
- Claude Code subagent entries: both `SessionID` and `ParentSessionID` of the resulting `TokenEvent` are the parent session UUID. The subagent's identity is in the `agentId` field, not a unique session ID. `source/claude.go:196-201`
- Go runs tests from the package directory, not the project root. All test data paths must use `filepath.Join("..", "testdata", ...)`. `source/*_test.go`
- With N ≤ 5 sessions and a single cost outlier, the 2σ threshold (population stddev) can never trip because the max z-score for one outlier is `sqrt(N-1)`. For N=5, max z = 2.0 — exactly equals the threshold, never exceeds it. Use N ≥ 6 for cost outlier test data. `analyze/waste_test.go`
- Global baseline key `"*"` provides cross-project percentile thresholds for H2 (low signal) and H4 (cache underutilized). `splitKey` handles this sentinel by returning `"*"` as project and `""` as harness. `analyze/baseline.go:21,166-173`
- Map iteration order in Go is non-deterministic. `BuildSubagentTree` iterates over `sessions` map, causing different output per run. Golden file tests must either sort output or avoid map-iteration-dependent sections. `output/json.go:113-115`
- Unexported helper types must be declared at package level, not inside functions. Defining types inside functions is prohibited by the codebase convention and makes types invisible to other package-level functions. `output/text.go:12-29`

## Mistakes

- Initial `go.mod` declared `go 1.26.2` (nonexistent version). Never write a go.mod version without checking `go version` on the actual runtime.
- Unreachable `if cost < 0 { return 0 }` guard in `CostForModel`. Negative tokens are already clamped at input — the negative-cost branch was dead code. Check invariants once at the entry point, not redundantly. `source/pricing.go:26`
- Left an unused `_ any` parameter that survived self-review until golangci-lint caught it. Run `golangci-lint run` before committing, not after.
- Defined `sessionInfo` as a local type inside `BuildSubagentTree` but tried to use it in helper function signatures. Local types defined inside a function can't be referenced from other function signatures — must be package-level. `analyze/subagent.go:25-31`
- Infinite recursion in `BuildSubagentTree` — real OpenCode data contained cyclic parent-child relationships among subagent sessions, causing stack overflow. Fix: `buildChildNodesVisited` tracks visited session IDs with a `map[string]bool` passed through the call chain. `analyze/subagent.go:114-145`
- Golden file tests used `time.Now()` for today/week calculations, making output non-deterministic. Tests passed at generation time but failed on subsequent runs. Fix: extract to package-level `NowFunc = time.Now` variable, override in tests to a fixed reference time. `output/json.go:223, output/output_test.go:21`

- `BurntSushi/toml` unmarshals into an existing struct, only overriding fields present in the TOML file. The `Defaults()` pattern (return a struct, unmarshal on top) gives partial-override semantics without manual field merging. `config/config.go:70-75`

## Patterns

- Float comparisons in tests: use `math.Abs(got - want) > delta` with `const delta = 0.0001`. Never `==` for float equality. `source/pricing_test.go:9,86`
- Separate test function for defensive guards vs. main table-driven behavior. Keeps table-driven tests focused on normal cases. `source/pricing_test.go`
- Env var override for test path injection: every Source follows the `BURNWATCH_<HARNESS>_<PATH>` convention. See `defaultDBPath()` in `source/opencode.go:177` and `defaultProjectDir()` in `source/claude.go:37-43`.
- Error channels buffered (cap 10), events channels unbuffered — prevents the goroutine from blocking on error sends while the consumer drains events. `source/opencode.go:24-25`
- `projectNameToDisplay()` strips leading `-` and replaces `-` with `/` to convert directory paths to display names. `source/claude.go:220-223`
- Waste signal sorting: severity descending (high → medium → low), then reason alphabetically, then SessionID. Ensures deterministic output ordering before CLI rendering. `analyze/waste.go:122-130`
- Per-session metric aggregation: `map[string]*sessionAgg` groups events by SessionID, then ratios and cache rates are computed from summed token counts. Same pattern in `aggregateMetrics` and `DetectWaste`. `analyze/baseline.go:77`, `analyze/waste.go:50-87`
- Model tracking for WasteSignals: use "last model wins" — assign `a.model = e.Model` whenever `e.Model != ""` during the event loop. Most sessions use a single model; this is an approximation good enough for v1. `analyze/waste.go:92-94`
- Signal toggle pattern: define a `SignalToggles` struct in the analyze package, guard each `check*` call with the toggle, merge CLI `--no-*` flags into config toggles before passing to `DetectWaste`. Centralized gating, no scattered boolean checks. `analyze/waste.go:26-32,127-154`
- Weekly trend aggregation: group events by Monday boundary using `weekStartOf()`, compute per-week totals, compare first vs last week. Use Unicode arrows (↑ ↓ →) for direction. Controlled by `config.Output.ShowTrends`. `analyze/trend.go:32-101`
- HTTP clients that fetch from a fixed external URL should expose a package-level `var url = "..."` so tests can override it with a `httptest.Server` URL. The main function delegates to an unexported helper that takes the URL explicitly. `source/pricing_fetcher.go:42-49`
- Use a package-level `var NowFunc = time.Now` for testable time-dependent code. Override in tests to a fixed reference time so today/week/month calculations produce deterministic output. `output/json.go:223`, `output/output_test.go:21`
- Golden file tests with `-update` flag: use `flag.Bool("update", ...)` and `os.WriteFile` to regenerate expected output when format changes. Run `go test ./pkg/... -update` to update, then re-run without flag to verify. `output/output_test.go:15,51-57`

## Hidden Couplings

- `CostForModel` in `source/pricing.go:26` is the single pricing entry point. Every Source calls it. Changing its signature or the `priceEntry` struct breaks all Sources. When adding fields to `TokenEvent`, check if pricing needs them too.
- Adding a Source touches 3 files: (1) the new `source/<name>.go`, (2) `source/interface.go` `Discover()`, (3) `README.md` Supported Harnesses. `source/interface.go:10-20`
- Adding fields to `WasteSignal` requires updating: (1) struct definition, (2) every `check*` function that returns a `WasteSignal`, (3) `output/text.go` `writeSignalBlock` for display, (4) `output/json.go` `JSONWasteSignal` if fields should appear in JSON output. `analyze/waste.go:11-24`
- Changing `DetectWaste` signature requires updating all call sites: `cmd/root.go`, `output/text.go`, and every test file (`waste_test.go`, `output_test.go`, `cmd/root_test.go`). Post-PR14 cleanup: `allToggles` was removed — use `config.Defaults()` or a custom `config.Config` with Signals set. `analyze/waste_test.go`
- Test in-memory SQLite DB schemas must match actual harness DB schemas exactly. If a harness changes its schema, tests break silently by failing to create matching tables. `source/opencode_test.go:361-365`
- `Baseline.RatioMean` was added to support H5 (session churn) which needs per-project mean ratios. Not in the original PR4 spec but required — `CostStd` alone doesn't capture output/input behavior. `analyze/baseline.go:7`
- Adding new baseline fields (InputMean, OutputMean, TERP10, etc.) to the `Baseline` struct + `sessionMetrics` is additive — no downstream callers need changes until the fields are consumed by heuristics. The JSON tags (`json:"..."`) on the Baseline struct only matter when a Baseline is directly serialized; the JSONReport output struct doesn't include baselines. `analyze/baseline.go:10-29`
- All heuristics depend on `ComputeBaselines` producing a global baseline (key `"*"`). Removing or renaming this key breaks H2, H4, and cross-project percentile comparisons. `analyze/baseline.go:21,65`
- OpenRouter API returns per-token prices (e.g. `"prompt":"0.000003"`), but burnwatch uses per‑1K‑token prices internally. Convert by multiplying by 1000. Getting this wrong inflates/deflates costs by 3 orders of magnitude. `source/pricing_fetcher.go:80-82`
- The embedded pricing table (`source/pricing.go:12-22`) only covers 6 models. All other models fall back to claude-sonnet pricing, inflating costs 5x–17x for deepseek, kimi, minimax, qwen, gpt-5.4. Fix: fetch pricing from OpenRouter API dynamically (PR11). Changing `CostForModel` signature adds a return value — all callers must be updated.
- Adding a new field to `TokenEvent` (`source/event.go`) that is consumed downstream requires touching: (1) all source implementations that create TokenEvent (claude.go, opencode.go), (2) all test helpers that construct events (scenario_test.go, bench_test.go), (3) all aggregation structs (sessionAgg in waste.go), (4) all output structs (WasteSignal, JSONWasteSignal). Propagation of a single field touches 8+ files. `source/event.go:21`

- Changing `ComputeBaselines` or `DetectWaste` function signatures requires updating every call site: `cmd/root.go`, `output/text.go`, `output/bench_test.go`, `cmd/root_test.go`, `analyze/baseline_test.go`, `analyze/waste_test.go`, `output/scenario_test.go`, `output/output_test.go`. Use `replaceAll` with precise old/new strings for mechanical edits, but always review diff manually. `analyze/baseline.go:41`, `analyze/waste.go:56`

## Rules

These govern how this file is maintained. Violating them makes the file less useful over time.

1. **Category-first, not chronological.** Add/edit entries within their category. Never add a new date/PR section.
2. **Merge before adding.** Before adding a new entry, check if a similar one exists. Consolidate instead of duplicating.
3. **Staleness gate.** When adding to a category, review the oldest ~3 entries. If a gotcha/pattern hasn't been relevant for 5+ PRs, remove it.
4. **Cite concrete code.** Every entry must reference a specific file, function, or symptom. "In general" observations are invalid.
5. **Drop forensic detail.** Commit hashes, dates, and PR numbers belong in git history, not here.
