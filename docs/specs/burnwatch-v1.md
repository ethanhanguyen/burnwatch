# Burnwatch v1 Specification

## Overview

Burnwatch reads local session data from AI agent harnesses (OpenCode, Claude Code) and produces a waste detection report using statistical outlier analysis. No cloud, no API keys, no telemetry.

## Supported harnesses

| Harness | Data source | Format | Cost source |
|---------|------------|--------|-------------|
| OpenCode | `~/.local/share/opencode/opencode.db` | SQLite | Pre-computed in `data.cost` |
| Claude Code | `~/.claude/projects/-<path>/<uuid>.jsonl` | JSONL | Computed via embedded pricing table |

## Data model

### TokenEvent

```go
type TokenEvent struct {
    SessionID       string
    ParentSessionID string    // non-empty if subagent
    AgentType       string    // "build", "explore", "general", ""
    Model           string    // e.g. "google/gemini-3-pro-preview"
    Provider        string    // e.g. "vercel", "anthropic"
    Timestamp       time.Time
    InputTokens     int64
    OutputTokens    int64
    CacheRead       int64
    CacheWrite      int64
    ReasoningTokens int64
    CostUSD         float64
    Project         string
    Harness         string    // "opencode", "claude-code"
    IsSubagent      bool
}
```

## Waste detection heuristics

All thresholds are computed from the user's own data. No hardcoded constants.

| # | Name | Condition | Severity | Rationale |
|---|------|-----------|----------|-----------|
| H1 | Cost outlier | `session_cost > μ_project + 2σ` | HIGH | Unusually expensive session — likely agent loops or wrong model |
| H2 | Low signal | `output/input_ratio < P10_all` | MEDIUM | Agent is reading context but not producing meaningful output |
| H3 | Subagent overhead | `subagent_cost > 50%` of total session cost | MEDIUM | Too much delegation — subagent context initialization costs compound |
| H4 | Cache underutilized | `cache_hit_rate < P10_all` | LOW | Prompt caching isn't working — consider instruction optimization |
| H5 | Session churn | ≥3 sessions same project/day, all below μ_ratio | MEDIUM | Fragmented sessions lose cached context on every restart |

### Baseline computation

For each `project:harness` pair:
1. Aggregate events into per-session metrics: total cost, total input/output tokens, cache hit rate.
2. Compute cost mean (µ) and standard deviation (σ).
3. Compute percentiles (P10, P50, P90) for output/input ratios and cache hit rates.
4. Require ≥10 sessions per project for meaningful baselines. Below that, use global baselines.

## Output formats

### Text

```
$ burnwatch
OpenCode: N sessions, M subagent sessions | Claude Code: P sessions
Today: $X (N sessions) | This week: $Y (M sessions)

Project A: $total (median $med/session)
Project B: $total (median $med/session)

Trends:  (when --show-trends)
  Cost:    $A/wk → $B/wk (↓ C%)
  Sessions: N/wk → M/wk (↑ P%)
  Output/input ratio: X.XX → Y.YY (↓ Z%)

Waste signals:
  HIGH ses_id slug (project): $cost — detail
    Model: model-name, tokens in / tokens out
    → Recommendation. Potential savings: $amount
  MED  ses_id slug (project): detail
    → Recommendation. Potential savings: $amount
  LOW  Project project: detail
    → Recommendation. Potential savings: $amount

Summary: N waste signals found. Potential savings: $amount
```

### JSON

```json
{
  "summary": { "opencode_sessions": 42, "today_cost": 2.34, ... },
  "projects": [{ "name": "...", "harness": "...", "session_count": 12, "total_cost": 10.50, "median_cost": 0.88 }],
  "waste_signals": [{ "session_id": "...", "severity": "high", "reason": "cost_outlier", "detail": "...", "metric": 1.86, "threshold": 1.24 }],
  "subagent_trees": [{ "session_id": "...", "total_cost": 0.94, "subagent_cost": 0.82, "overhead_pct": 87.2, "subagents": [...] }],
  "recommendations": [{ "signal": "cost_outlier", "action": "...", "detail": "...", "savings_est": 1.24 }],
  "potential_savings": 4.29
}
```

## CLI flags

| Flag | Default | Description |
|------|---------|-------------|
| `--db` | (auto-detect) | OpenCode DB path |
| `--harness` | `all` | Filter: `all`, `opencode`, `claude-code` |
| `--project` | (none) | Filter to specific project |
| `--json` | `false` | Output JSON instead of text |
| `--days` | `0` | Lookback window in days (0 = all time) |
| `--verbose` | `false` | Show all sessions, not just waste signals |
| `--config` | `.burnwatch.toml` | Config file path (TOML) |
| `--print-config` | `false` | Print effective config and exit |
| `--min-cost` | `0` | Hide waste signals below this dollar amount |
| `--show-trends` | `false` | Show time-trend summary between projects and waste signals |
| `--no-cost-outlier` | `false` | Disable cost outlier detection |
| `--no-low-signal` | `false` | Disable low output/input ratio detection |
| `--no-subagent-overhead` | `false` | Disable subagent overhead detection |
| `--no-cache-underutil` | `false` | Disable cache underutilization detection |
| `--no-churn` | `false` | Disable session churn detection |

## Configuration

Optional TOML config file (`.burnwatch.toml` or `~/.config/burnwatch/config.toml`):

```toml
[thresholds]
cost_outlier_sigma = 2.0
low_signal_percentile = 10.0

[signals]
cost_outlier = true
low_signal = true
subagent_overhead = true
cache_underutilized = true
session_churn = true

[filters]
min_cost = 0
deduplicate = false

[output]
group_churn = false
show_trends = false
```

CLI flags override config values at runtime.

## Pricing

**v2 (PR11):** Pricing fetched from OpenRouter API (`https://openrouter.ai/api/v1/models`) on first run, cached locally (7-day TTL). All models get accurate per-token pricing. `≈` indicator shown when embedded fallback is used. No model left behind.

**v1 (deprecated):** Embedded in binary. Supports:

**Anthropic:**
- Claude Sonnet 4-5: $3.00/$15.00 per 1K tokens (input/output)
- Claude Opus 4-5: $15.00/$75.00
- Claude Haiku 4-5: $0.80/$4.00
- Cache read: 10% of input price
- Cache write: 125% of input price

**Google/Gemini:**
- Gemini 3 Pro: $1.25/$5.00
- Gemini 2.5 Pro: $1.25/$5.00
- Gemini 2.5 Flash: $0.15/$0.60

**Fallback:** Unknown models use Sonnet-tier pricing with `≈` indicator in output.

## Limitations

1. No real-time monitoring — batch analysis only. Each run reads all data fresh.
2. No TUI or web dashboard — text or JSON output only.
3. Claude Code cost is computed (not stored by Claude Code) — may slightly differ from actual bills.
4. Subagent tracking depends on harness support. Claude Code links subagents by directory structure. OpenCode uses `parent_id` FK.
5. ~~Pricing table becomes stale when providers change prices. Update via new release.~~ **Resolved in v2 (PR11):** pricing fetched dynamically from OpenRouter API, cached with 7-day TTL.
6. Single-user, single-machine. No cross-machine aggregation.

### v2 roadmap

See [`docs/plans/v2-implementation-plan.md`](../plans/v2-implementation-plan.md) for the full v2 plan covering:
- Dynamic pricing (PR11)
- Token baselines + heuristics (PR12–PR13)
- Config-wired thresholds (PR14)
- Calibration mode (PR15)
- Anomaly detection (PR16)
- LLM verification (PR17)
- ML pipeline (PR18)
