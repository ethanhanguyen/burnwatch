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
