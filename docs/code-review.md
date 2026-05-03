# Code Review

Static checklist applied to every PR. Run before merge.

---

## Phase 0: Automated

Single command. All must pass. Runs on `git push` via pre-push hook.

```
./scripts/review-check.sh
```

| Check | What it verifies |
|-------|-----------------|
| `go vet ./...` | No vet warnings |
| `golangci-lint run` | Zero lint warnings |
| `go build -o burnwatch .` | Clean compile |
| `go test -cover ./...` | Coverage >= 80% per package |
| No `interface{}` / `map[string]any` in diff | Codebase convention |
| No `panic()` in non-test Go files | Use `t.Fatalf` / `t.Errorf` |
| Testdata paths use `filepath.Join("..", "testdata", …)` | Go runs tests from package dir |
| No new config files (`.env`, `.yaml`, `.toml`, `.json`) | Env vars + CLI flags only |
| No `sync.Mutex` / `sync.RWMutex` in non-test diff | Single-goroutine architecture |

**Pre-push hook:** `ln -sf ../../scripts/pre-push .git/hooks/pre-push`

---

## Phase 1: Diff Inspection

Verify scope and coupling. ~2 min manual.

| Check | How to verify |
|-------|---------------|
| Only expected files changed | `git diff main...HEAD --name-only` — must match PR Files table |
| New Source touches exactly 3 files | `source/<name>.go` + `source/interface.go` (`Discover()`) + `README.md` |
| `CostForModel` signature unchanged | `git diff main...HEAD -- source/pricing.go` — verify signature intact |
| Global baseline key `"*"` preserved | `git diff main...HEAD -- analyze/baseline.go` — star key must not be renamed/removed |
| Unexported helpers at package level | Scan diff: helper types must be at file top-level, not inside functions |
| `TokenEvent` is least-common-denominator | No harness-specific fields added to `source/event.go` |
| No drive-by reformats | Whitespace-only changes in unrelated files get flagged |

---

## Phase 2: Code Patterns

Semantic correctness. ~5 min manual.

### Go Idioms

| Check | How to verify | Severity |
|-------|---------------|----------|
| Early return on empty/zero input | `git diff` — scan for nested `if val > 0 { … }` | Should |
| Error wrapping with `%w`, not `%v` | `git diff \| grep '%v'` — flag any in error-return paths | Should |
| Float comparison uses `math.Abs(delta)` | Search diff for `==` or `!=` with float types | Critical |
| No comments on exported functions | Scan diff for `// Foo does …` above `func Foo` (convention to omit) | Could |
| Short names in tight loops, descriptive otherwise | Subjective — flag single-char names in 20+ line scopes | Could |
| Table-driven tests for parse/transform logic | Verify `_test.go` diff uses `tests := []struct{…}` not copy-paste | Should |

### Defensive Code

| Check | How to verify | Severity |
|-------|---------------|----------|
| Negative tokens clamped to 0 at entry | Verify clamp happens once, not redundantly in each consumer | Critical |
| Non-fatal parse errors go to error channel | `errorCh <-` not `return err` for per-entry parse failures | Critical |
| Channel consumer drains events + errors concurrently | Verify goroutine + `done` channel pattern in `Source.Events()` | Should |
| Error channels buffered (cap 10), events channel unbuffered | Check `make(chan …)` calls in diff | Critical |
| No panics — use `t.Fatalf` for preconditions | Any `panic(` call in non-test code must be flagged | Should |

### Project Gotchas

| Check | Source | How to verify |
|-------|--------|---------------|
| Pricing table is ordered `[]struct{}`, not map | Gotcha | No `map[string]` in `source/pricing.go` |
| New pricing entries must not shadow existing model substrings | Gotcha | Review new model names don't prefix-match existing keys |
| N >= 6 sessions required for cost outlier to trip | Gotcha | Verify guard in `analyze/waste.go` checks `len(sessions) >= 6` |
| Test in-memory SQLite schemas match actual harness DB schemas | Gotcha | Cross-reference `CREATE TABLE` in tests vs harness source |
| Output sort: severity desc -> reason asc -> SessionID | Pattern | Check `sort.Slice` in `output/` matches contract |

---

## Phase 3: Behavioral Gates

Design-level sanity. Each gate has a pass condition. ~3 min.

| Gate | Pass condition | Evidence |
|------|---------------|----------|
| **Simplicity** | No speculative features, unnecessary abstractions, or "flexibility" not in PR spec. Diff under 200 LoC for non-trivial PR. | No unused exported types; every new function traced to a success criterion. |
| **Surgical** | All changed files listed in PR Files table. No unrelated touchpoints. | `git diff main...HEAD --name-only` matches Files table (excl. go.sum/go.mod). |
| **Explicit** | Every ambiguous decision has a PR comment, ADR, or code comment referencing the constraint. | No undocumented assumptions visible in diff — testable by a fresh reviewer. |
| **Goal-driven** | Every added function called from >=1 test asserting a success criterion from the PR. | `go test -run=TestXxx -v` shows assertions matching PR success criteria. |
