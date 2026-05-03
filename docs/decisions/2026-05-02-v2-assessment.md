# ADR: Burnwatch v1 Assessment and v2 Direction

## Status

Accepted (2026-05-02)

## Context

After completing PR1–PR10, burnwatch was run on real session data (908 main sessions, 1,255 subagent sessions, ~$1.06M tracked spend). The output revealed three structural problems that undermine the tool's core value proposition.

## Findings

### Problem 1: Dollar amounts are wrong

The embedded pricing table in `source/pricing.go` covers only 6 models (claude-sonnet-4-5, claude-opus-4-5, claude-haiku-4-5, gemini-3-pro, gemini-2.5-pro, gemini-2.5-flash). All other models (deepseek, kimi, minimax, qwen, gpt-5.4 — the majority of user sessions) fall back to claude-sonnet-4-5 pricing ($3/$15 per MTok).

Actual pricing vs. current fallback:
- deepseek/deepseek-v4-pro: $0.44/$0.87 → fallback $3/$15 (**6.8x–17x inflation**)
- qwen/qwen3.6-plus: $0.33/$1.95 → fallback $3/$15 (**9.1x–7.7x**)
- minimax/minimax-m2.7: $0.30/$1.20 → fallback $3/$15 (**10x–12.5x**)

This makes the "potential savings: $824,948.98" headline meaningless.

### Problem 2: Token data is collected but unused

`TokenEvent` has separate `InputTokens`/`OutputTokens` fields since PR1. `sessionAgg` sums them independently. But every heuristic (H1–H5) operates on cost or ratio — never on absolute input or output token counts. This is squandering data that is already cleanly plumbed through every layer of the architecture.

### Problem 3: Heuristic thresholds are hardcoded

Only `cost_outlier_sigma` is configurable. `LowSignalPercentile` exists in config but isn't wired. Subagent overhead (50%), churn min sessions (3), P10 values — all unchanging constants. A user with 3,000 sessions gets the same thresholds as one with 30.

## Decision

Launch a v2 development cycle with three phases:

**Phase A — Foundation (PR11–PR14):**
- PR11: Replace hardcoded pricing with OpenRouter API (free, no auth, covers all models)
- PR12: Add token baselines (input/output mean, std, percentiles)
- PR13: Four new token-based heuristics (H6 input overconsumption, H7 output explosion, H8 token efficiency, H9 fragmentation index)
- PR14: Complete config wiring for all thresholds

**Phase B — Calibration + Advanced (PR15–PR17):**
- PR15: `--calibrate` mode showing full distribution and suggested thresholds
- PR16: Isolation Forest for multi-dimensional anomaly detection
- PR17: Optional LLM verification of top-N waste signals

**Phase C — ML (PR18):**
- PR18: Supervised logistic regression pipeline (experimental, disabled by default)

## Consequences

### Positive
- Dollar amounts will be accurate for all models (OpenRouter pricing is community-maintained)
- Token-based heuristics will catch waste invisible to cost-based detection
- Users can calibrate thresholds to their own data distribution
- Zero new external dependencies (all pure Go, stdlib only)

### Negative
- PR11 breaks backward compatibility on dollar amounts (old golden files must be regenerated)
- PR13 removes H5 (session churn), replaced by H9 (fragmentation index) — config migration needed
- OpenRouter API dependency for pricing freshness (graceful degradation: cached → embedded fallback)

### Risks
- OpenRouter could change their API format. Mitigation: cache JSON with 7-day TTL, embedded table as ultimate fallback.
- Token-based heuristics (H6–H9) are novel — no prior art. May need tuning based on user feedback.
- LLM verification (PR17) costs money per run — opt-in only, requires explicit `--llm-key` flag.
