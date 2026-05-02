# PR1: Foundation — TokenEvent, Source interface, Pricing table

## Objective

Establish the core types and interfaces that all other PRs depend on. No runtime I/O yet — just the contracts and a pricing table.

## Files to create

| File | Purpose |
|------|---------|
| `go.mod` | Initialize Go module (`github.com/yourname/burnwatch`). Go 1.22+. |
| `source/event.go` | `TokenEvent` struct — the universal data model |
| `source/interface.go` | `Source` interface + `Discover()` function |
| `source/pricing.go` | Embedded pricing table, `CostForModel()` function |
| `source/pricing_test.go` | Test cost calculation for known models |

## `source/event.go`

```go
type TokenEvent struct {
    SessionID       string
    ParentSessionID string
    AgentType       string   // "build", "explore", "general", "" for top-level
    Model           string
    Provider        string
    Timestamp       time.Time
    InputTokens     int64
    OutputTokens    int64
    CacheRead       int64
    CacheWrite      int64
    ReasoningTokens int64
    CostUSD         float64  // computed by Source or by pricing.go
    Project         string
    Harness         string   // "opencode", "claude-code"
    IsSubagent      bool
}
```

**Constraints:**
- All exported fields (no getters — Go convention for pure data types).
- `Timestamp` uses `time.Time` (UTC).
- `CostUSD` may be zero if the source doesn't pre-compute it (Claude Code). The analysis layer fills it via `pricing.go`.
- `IsSubagent` is true when `ParentSessionID != ""`.

## `source/interface.go`

```go
type Source interface {
    Name() string
    Events() (<-chan TokenEvent, <-chan error)
}

func Discover() []Source
```

- `Name()` returns harness identifier (e.g. `"opencode"`, `"claude-code"`).
- `Events()` returns two channels: events and errors. The source streams all parsed events and closes both channels when done. Errors are non-fatal parse warnings (skip that entry, continue).
- `Discover()` auto-detects which harnesses are installed by checking well-known paths. Returns available `Source` instances.

## `source/pricing.go`

Embed a pricing table supporting both Anthropic and Google/Gemini models (the two providers seen in user data). Per-1K-token pricing:

| Model | Input/1K | Output/1K | Cache Read/1K | Cache Write/1K |
|-------|----------|-----------|---------------|-----------------|
| Claude Sonnet 4-5 | $3.00 | $15.00 | $0.30 | $3.75 |
| Claude Opus 4-5 | $15.00 | $75.00 | $1.50 | $18.75 |
| Claude Haiku 4-5 | $0.80 | $4.00 | $0.08 | $1.00 |
| Gemini 3 Pro | $1.25 | $5.00 | — | — |
| Gemini 2.5 Pro | $1.25 | $5.00 | — | — |
| Gemini 2.5 Flash | $0.15 | $0.60 | — | — |

Also add `fallback-pricing` with Sonnet-tier costs for unknown models.

Export `func CostForModel(model string, inputTokens, outputTokens, cacheRead, cacheWrite int64) float64`.

Match models by substring (e.g. `"claude-sonnet-4-5-20250929"` contains `"claude-sonnet-4-5"`).

## Test requirements

1. **`source/pricing_test.go`**: Table-driven tests for:
   - Known model → correct cost
   - Unknown model → falls back to fallback pricing
   - Zero tokens → zero cost
   - Each known model variant matches correctly
   - Edge case: very large token counts (no overflow)

2. **Compile-time only** for `event.go` and `interface.go` (no runtime behavior to test).

## Approach: TDD

1. Write `pricing_test.go` first (RED — won't compile yet).
2. Write type stubs in `pricing.go` (compile but tests fail).
3. Fill in pricing table + matching logic (GREEN).
4. Verify ≥90% coverage on `pricing.go`.
5. Write `event.go` and `interface.go` (no tests needed — pure types).

## Exit criteria

- [ ] `go test ./source/... -cover` passes with ≥90% coverage on `pricing.go`
- [ ] `go vet ./...` passes with zero warnings
- [ ] `go build ./...` compiles cleanly
- [ ] Commit message: `feat: add TokenEvent, Source interface, and pricing table`
- [ ] Push to branch `pr1-foundation`

## Notes

- No `main.go` yet — this PR is pure library code.
- Pricing table should be a `map[string]priceEntry` keyed by canonical model name. Match with `strings.Contains()`.
- Do NOT add any external dependencies (keep `go.mod` minimal).
