# Repo Learnings

<!--
  Accumulated knowledge from PR sessions. Read on session start to avoid past mistakes.
  4 categories — use only those that apply. Every bullet cites a concrete file/function/symptom.
  "In general" entries are invalid.
-->

## Categories

| Category | Prompt |
|----------|--------|
| **Gotcha** | Something not obvious from reading the code. Surprising behavior, hidden constraints. |
| **Mistake** | What went wrong + root cause + how to prevent next time. |
| **Pattern** | An idiom or convention worth repeating. |
| **Hidden coupling** | Touching X means you must also update Y. |

---

## 2026-05-02 — PR1: Foundation (TokenEvent, Source interface, Pricing)

### Gotcha
- `pricing.go` uses a `[]struct{key string; p priceEntry}` slice, not a `map[string]priceEntry`. Reason: model matching is substring-based (`strings.Contains`), which requires ordered iteration. A map would hide ambiguous matches silently — the slice ensures first-match-wins is explicit and testable. `source/pricing.go:12-22`

### Mistake
- Initial `go.mod` declared `go 1.26.2` (nonexistent version). Root cause: not checking `go version` on the actual runtime. Should match Go 1.22 exactly. Fixed in `04692cc`.
- Added unreachable `if cost < 0 { return 0 }` guard in `CostForModel`. Negative tokens are already clamped at input — the negative-cost branch was dead code. Fixed in `04692cc`. Lesson: check invariants once at the entry point, not redundantly.

### Pattern
- Float comparisons in tests use `math.Abs(got-want) > delta` with `const delta = 0.0001`. Never use `==` for float equality. `source/pricing_test.go:9,86`
- Separate test function (`TestCostForModelNonNegative`) for the defensive negative-input guard — keeps the main table-driven test focused on normal behavior.

### Hidden coupling
- `CostForModel` in `source/pricing.go:26` is the single pricing entry point. Every Source (OpenCode, Claude Code) calls it. Changing its signature or the `priceEntry` struct breaks all sources. When adding new fields to `TokenEvent`, check if pricing needs them too.

## 2026-05-02 — PR2: OpenCode Source

### Gotcha
- `modernc.org/sqlite` v1.50.0 was incompatible with Go 1.22 (required newer stdlib). Had to downgrade to v1.29.0. The latest pure-Go SQLite isn't always compatible — pin to a version that matches the project's Go toolchain. `go.mod:5` (fixed in `2060479`).

### Mistake
- Left an unused `_ any` parameter on `tokenDataToEvent()` — a leftover from an earlier design draft. Survived self-review until golangci-lint caught it. Fixed in `2060479`. Lesson: run `golangci-lint run` before committing, not after.

### Pattern
- `defaultDBPath()` in `source/opencode.go:177` checks `BURNWATCH_OPENCODE_DB` env var first, then falls back to the well-known path. This allows tests to inject custom DB paths without source modification. Every new Source should follow this pattern (env var override).
- Error channel in `Events()` is buffered (cap 10), events channel is unbuffered. Prevents the goroutine from blocking on error sends while the consumer drains events. `source/opencode.go:24-25`

### Hidden coupling
- `Discover()` in `source/interface.go:10-19` directly constructs `&OpenCodeSource{}`. Every new harness adds a compile-time dependency to `interface.go`. When adding a Source, you must update: (1) the Source file, (2) `Discover()`, (3) `README.md` Supported Harnesses.
- Test creates in-memory SQLite DBs with `sql.Open("sqlite", path)` + `CREATE TABLE` statements. Schema must match OpenCode's actual DB schema exactly. If OpenCode changes its schema, tests break silently by failing to create matching tables. `source/opencode_test.go:361-365`

## 2026-05-02 — PR3: Claude Code Source

### Gotcha
- Claude Code subagent entries store the parent's `sessionId` in their own line. Both the `SessionID` and `ParentSessionID` of the resulting `TokenEvent` end up being the parent session UUID — the subagent is identifier is the `agentId` field, not a unique session. `source/claude.go:196-201`
- Test data files referenced from `source/claude_test.go` need `filepath.Join("..", "testdata", ...)` — Go runs tests from the package directory, not the project root. Same pattern as opencode_test.go. `source/claude_test.go:30,38`

### Mistake
- Initial test used relative path `testdata/claude_sample.jsonl` without `..` prefix. Tests in `go test ./source/...` run with working directory set to `source/`, not project root. Fixed by matching the `filepath.Join("..", "testdata", ...)` pattern from opencode_test.go. `source/claude_test.go:30`

### Pattern
- `defaultProjectDir()` uses `BURNWATCH_CLAUDE_PROJECTS` env var for test overrides, following the same pattern as `defaultDBPath()` from the OpenCode source. `source/claude.go:37-43`
- `projectNameToDisplay()` strips leading `-` and replaces `-` with `/` to convert `-Users-hoang-project` → `Users/hoang/project`. `source/claude.go:220-223`

### Hidden coupling
- Adding a Source touches exactly 3 files: (1) the new `source/<name>.go`, (2) `source/interface.go` `Discover()`, (3) `README.md` Supported Harnesses. `source/interface.go:10-20`
