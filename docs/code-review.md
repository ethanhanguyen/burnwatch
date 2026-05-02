# Code Review

<!--
  Static reference. Applies to every PR — no per-review fill-in needed.
  Severity: Critical (potential bug / broken invariant), Should (pattern/architecture violation), Could (style/nit).
  Every finding cites a concrete file:line.
-->

## Behavioral

- [ ] **Simplicity.** No speculative features, unnecessary abstractions, or "flexibility" not requested. If 200 lines could be 50, flag it.
- [ ] **Surgical.** Only expected files changed. No drive-by refactors, comment "improvements," or deleted dead code.
- [ ] **Explicit.** No hidden assumptions. If something is unclear, stop and ask — don't pick silently among interpretations.
- [ ] **Goal-driven.** Changes trace to a concrete success criterion. No code without a testable claim.

## Go Idioms

| Check | Severity |
|-------|----------|
| Early return on empty/zero input — no nested conditionals | Should |
| Error wrapping with `%w`, not `%v` or string concatenation | Should |
| Float comparison in tests uses `math.Abs(got-want) > delta` | Critical |
| No comments on exported functions (codebase convention) | Could |
| Short variable names in tight loops, descriptive otherwise | Could |
| No `interface{}`, `map[string]any`, or generics (codebase convention) | Should |
| Table-driven tests for parse/transform logic (not copy-pasted sub-tests) | Should |

## Defensive Code

| Check | Severity |
|-------|----------|
| Negative tokens clamped to 0 at entry point, not redundantly | Critical |
| Non-fatal parse errors go to error channel (skip entry, continue) | Critical |
| Error channels buffered (cap 10), events channel unbuffered | Critical |
| Channel consumer drains events and errors concurrently (goroutine + done channel) | Should |
| No panics — use `t.Fatalf` for precondition failures, `t.Errorf` for field checks | Should |

## Project-Specific

| Check | Source |
|-------|--------|
| New Source touches 3 files: `source/<name>.go`, `source/interface.go` `Discover()`, `README.md` | Hidden coupling |
| `CostForModel` signature unchanged — it's the single pricing entry point for all Sources | Hidden coupling |
| Pricing table is an ordered `[]struct{}` slice, not a map. New entries must not shadow existing model substrings. | Gotcha |
| Global baseline key `"*"` preserved — removing/renaming breaks H2, H4, and cross-project percentiles | Gotcha |
| N ≥ 6 sessions required for cost outlier detection to trip (max z = sqrt(N-1)) | Gotcha |
| Test in-memory SQLite schemas match actual harness DB schemas exactly | Gotcha |
| Unexported helper types declared at package level, not inside functions | Mistake |
| No config files introduced — env vars + CLI flags only | Architecture |
| Concurrency is single-goroutine per `Source.Events()`. No mutexes, goroutine pools, or race conditions. | Architecture |
| `TokenEvent` is least-common-denominator — harness-specific data is dropped, not added as fields | Architecture |
| Testdata paths use `filepath.Join("..", "testdata", ...)` — Go runs tests from package dir | Pattern |
| Output sort order is explicit: severity desc → reason asc → SessionID. Must stay deterministic. | Pattern |

## Without Fail

- [ ] `go vet ./...` passes
- [ ] `golangci-lint run` passes (zero warnings)
- [ ] `go build -o burnwatch .` succeeds
- [ ] `go test ./... -cover` passes with ≥90% coverage on new code
