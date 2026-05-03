# Architecture

## Pipeline

```
┌──────────────┐    ┌──────────────┐
│ OpenCode DB  │    │ Claude Code  │
│ (SQLite)     │    │ JSONL files  │
└──────┬───────┘    └──────┬───────┘
       │                   │
       ▼                   ▼
┌──────────────┐    ┌──────────────┐
│ OpenCode     │    │ Claude Code  │
│ Source       │    │ Source       │
└──────┬───────┘    └──────┬───────┘
       │                   │
       ▼                   ▼
     ┌─── TokenEvent stream ───┐
     │  (unified data model)   │
     └───────────┬─────────────┘
                 ▼
     ┌─────────────────────┐
     │ analyze/baseline.go │  → μ, σ, percentiles per project
     └──────────┬──────────┘
                ▼
      ┌─────────────────────┐
      │ analyze/waste.go    │  → 5 heuristics, flag outliers
      └──────────┬──────────┘
                 ▼
      ┌─────────────────────┐
      │ analyze/trend.go    │  → weekly cost/ratio trends
      └──────────┬──────────┘
                 ▼
      ┌─────────────────────┐
      │ analyze/signal_     │  → min-cost filter, dedup
      │ filter.go           │
      └──────────┬──────────┘
                 ▼
      ┌─────────────────────┐
      │ analyze/subagent.go │  → cost tree, overhead %
      └──────────┬──────────┘
                ▼
     ┌─────────────────────┐
     │ analyze/recommend.go│  → waste → actionable text
     └──────────┬──────────┘
                ▼
     ┌──────────┴──────────┐
     ▼                     ▼
┌──────────┐        ┌──────────┐
│ output/  │        │ output/  │
│ text.go  │        │ json.go  │
└──────────┘        └──────────┘
```

## Module boundaries

### `source/` — Data ingestion

- `event.go`: `TokenEvent` struct — the universal data model. All fields across all harnesses.
- `interface.go`: `Source` interface + `Discover()` auto-detection.
- `opencode.go`: Reads OpenCode's SQLite DB. Queries `message` table for assistant entries with token data.
- `claude.go`: Reads Claude Code's per-project session JSONL. Walks project directories, parses `assistant` entries, discovers subagent files.
- `pricing.go`: Embedded pricing table for Anthropic and Google/Gemini models. `CostForModel()` computes USD from token counts.

### `analyze/` — Waste detection

- `baseline.go`: Groups events by project+harness. Computes µ, σ for costs, percentiles (P10, P50, P90) for ratios and cache rates.
- `waste.go`: 5 heuristics using statistical thresholds. Returns `[]WasteSignal`. Accepts `SignalToggles` for per-signal on/off gating.
- `trend.go`: Groups events by week, compares first vs last week for cost, sessions, and output/input ratio direction.
- `signal_filter.go`: `FilterByMinCost` and `Deduplicate` post-processing steps.
- `subagent.go`: Builds parent-child cost tree from `ParentSessionID` links. Computes overhead percentage.
- `recommend.go`: Maps `WasteSignal` → human-readable `Recommendation` with savings estimates.

### `output/` — Formatting

- `text.go`: Human-readable plain-text report with severity markers, aligned columns.
- `json.go`: Machine-readable JSON with full objects for piping to jq.

### `cmd/` — CLI

- `root.go`: Flag parsing (`--harness`, `--project`, `--json`, `--days`, `--verbose`, `--min-cost`, `--show-trends`, `--no-*`), config loading, pipeline dispatch.

## Concurrency model

Sequential — everything runs in one goroutine. Each `Source.Events()` streams events one at a time. The analysis layer collects all events into a single slice before computing baselines. Simple, predictable, no race conditions.

## Pricing

Embedded in `source/pricing.go` as a Go map. Updated by editing the map and releasing a new version. Future v1.1 may add `burnwatch update-prices` to fetch from a remote manifest.

## Adding a new harness

See [contributing.md](./contributing.md) and [specs/source-interface.md](./specs/source-interface.md).
