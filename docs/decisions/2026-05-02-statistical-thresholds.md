# ADR: Statistical Thresholds for Waste Detection

**Date**: 2026-05-02
**Status**: Accepted

## Context

Burnwatch needs to flag "wasteful" sessions — those that cost more or produce less output than normal. The naive approach is hardcoded thresholds (e.g., "flag sessions with >3 duplicate file reads", "flag sessions with output/input ratio <5%"). But these have two problems:

1. **One size doesn't fit all**: A heavy user's "normal" session costs $5. A light user's costs $0.50. A 3-file-read threshold means nothing to someone who reads 50 files per session.
2. **Thresholds are arbitrary**: Why 3 reads? Why 5% ratio? These numbers are made up, not derived from data.

## Decision

Use **statistical outlier detection on the user's own data**. Three mechanisms:

1. **Cost outliers**: Flag sessions where `cost > μ_project + 2σ`. This catches sessions that are anomalously expensive relative to the user's own behavior. Using 2σ gives ~95% confidence the session is a true outlier (assuming roughly normal distribution).

2. **Percentile-based thresholds**: For metrics without natural distributions (output/input ratios, cache hit rates), use percentiles. Flag sessions below P10 (the bottom 10%). This means "this session is in the worst 10% of all your sessions." The user decides if that's a real problem.

3. **Absolute thresholds for structural waste**: For patterns that are inherently wasteful regardless of user behavior, use absolute thresholds:
   - Subagent cost >50% of session total → flag
   - ≥3 sessions in the same project on the same day, all below the user's median ratio → flag

## Alternatives considered

### Hardcoded constants
- **Rejected**: Brittle across users. Would need per-user tuning, defeating the purpose of a zero-config tool.

### ML clustering (k-means, DBSCAN)
- **Rejected for v1**: Over-engineered for a 1200-line tool. Requires labeled training data to validate cluster quality. Adds complexity without clear benefit over simple statistics.

### LLM-powered analysis (like Agent Optimization)
- **Deferred to v2**: Claude Haiku can provide qualitative insights ("this session looks like it was stuck in a loop"), but it costs tokens itself. Statistical signals should prove the data pipeline works first.

## Consequences

### Positive
- Thresholds self-calibrate to each user. No configuration needed.
- Natural feedback loop: as the user improves their agent usage, baselines shift, and old patterns stop flagging.
- Transparent: users can see "your median is X, this session is Y" — no black box.

### Negative
- Requires sufficient data: ≥10 sessions per project for meaningful baselines. Below that, falls back to global baselines (across all projects). First-time users with <10 total sessions get a "Not enough data — collect more sessions" message instead of unreliable signals.
- Assumes roughly normal distribution for costs. Heavy-tailed distribution (few very expensive sessions) can inflate σ and suppress legitimate outliers. Mitigation: also compute median absolute deviation (MAD) as a robustness check in v1.1.
- Re-computed every run. If the user runs burnwatch 10x on the same day, thresholds are identical (no drift). Acceptable for a batch tool.

## When to revisit

- If user feedback says "it flags sessions I know are fine" → tighten threshold to 3σ or add a whitelist feature.
- If user feedback says "it misses sessions I know are wasteful" → relax to 1.5σ or add more heuristic types.
- When we have 100+ users with anonymized data, compare P95/P10/2σ against ML clustering and LLM analysis for accuracy.
