# Burnwatch

**Measure what your AI agents cost — and where they waste it.**

Burnwatch reads your local AI agent session data and pinpoints the exact sessions, patterns, and behaviors burning money without delivering value.

Works offline. Zero configuration. No API keys, no cloud, no telemetry.

## Supported Agents

| Agent | Status | Source |
|-------|--------|--------|
| [OpenCode](https://github.com/sst/opencode) | Stable | `~/.local/share/opencode/opencode.db` (SQLite) |
| [Claude Code](https://github.com/anthropics/claude-code) | Stable | `~/.claude/projects/` (JSONL) |

## Quickstart

### Install

**Go toolchain** (any platform):

```bash
go install github.com/ethanhanguyen/burnwatch@latest
```

**Homebrew** (macOS / Linux):

```bash
brew install ethanhanguyen/tap/burnwatch
```

**Pre-built binaries** — download from [GitHub Releases](https://github.com/ethanhanguyen/burnwatch/releases) for macOS, Linux, Windows (x86_64 + arm64).

### Run

```bash
burnwatch
```

That's it. Burnwatch discovers your session data automatically and prints a waste report.

## Usage

```bash
burnwatch [flags]
```

### Base Options

| Flag | Description |
|------|-------------|
| `--days <n>` | Lookback window in days (default: today) |
| `--harness <name>` | Filter to `opencode`, `claude-code`, or `all` |
| `--project <name>` | Filter to a single project path |
| `--json` | Machine-readable JSON output |
| `--verbose` | Show all sessions, not just waste |
| `--min-cost <n>` | Hide waste signals below $n |
| `--show-trends` | Include time-trend summary |
| `--version` | Print version and exit |

### Signal Toggles

Disable specific detectors:

```bash
--no-cost-outlier        --no-low-signal
--no-subagent-overhead   --no-cache-underutil
--no-fragmentation-index --no-input-overconsumption
--no-output-explosion    --no-token-efficiency
--no-tool-loop           --no-file-reread
--no-subagent-overlap    --no-session-restart
```

### Threshold Overrides

| Flag | Applies to |
|------|------------|
| `--input-sigma <n>` | Input overconsumption sigma |
| `--output-sigma <n>` | Output explosion sigma |
| `--ter-percentile <n>` | Token efficiency ratio percentile |
| `--fragmentation-threshold <n>` | Fragmentation index cutoff |
| `--subagent-overhead <n>` | Subagent overhead percentage |

### Calibration Mode

See your data distribution and get suggested thresholds:

```bash
burnwatch --calibrate             # Text output
burnwatch --calibrate --json      # Machine-readable
burnwatch --calibrate --no-fetch-pricing  # Skip network call
```

### Session Drill-Down

Annotated event timeline for a single session:

```bash
burnwatch --explain <session-id>
```

### HTML Report

Generate a static HTML report:

```bash
burnwatch report                  # 30-day report, writes to reports/
burnwatch report --days 7         # 7 days
burnwatch report --output out.html  # Custom path
burnwatch report --open           # Open in browser after generation
burnwatch report --report-json    # JSON data only (no HTML)
```

### Pricing

| Flag | Description |
|------|-------------|
| `--no-fetch-pricing` | Skip OpenRouter API call, use cached/embedded pricing |
| `--refresh-pricing` | Force re-fetch pricing from OpenRouter |

### Configuration

```bash
burnwatch --init          # Write default .burnwatch.toml
burnwatch --print-config  # Print effective config and exit
burnwatch --config <path> # Use custom config file
```

## What It Detects

Burnwatch runs five self-calibrating heuristics against your own session history. No hardcoded thresholds — everything adapts to your usage patterns.

1. **Cost Spikes** — Sessions exceeding 2σ above your project's median cost. Catches runaway loops, forgotten prompts, or agents stuck retrying.
2. **Low-Signal Sessions** — Sessions where the agent reads significantly more than it produces. Indicates context thrashing, poor prompts, or unnecessary model capacity.
3. **Subagent Proliferation** — Parent sessions where >50% of cost went to spawned subagents instead of inline work. Surfaces coordination overhead you might not realize you're paying for.
4. **Cache Misses** — Sessions with abnormally low prompt cache hit rates. Points at sessions that don't benefit from caching — often due to non-deterministic prompts or large tool output.
5. **Session Churn** — Days where many short sessions fall below your average output ratio. Suggests repeated restarts losing cached context each time.

## Example Output

```
$ burnwatch
OpenCode: 1610 sessions, 1089 subagent sessions
Today: $2.34 (8 sessions) | This week: $14.21 (37 sessions)

Waste signals:
  HIGH  ses_abc Bright-Butterfly (lilysbeauty): $1.86 — 3.4x project median ($0.55)
    → Investigate session for unnecessary loops or re-prompts. Potential savings: $1.31
  MED   ses_def Cool-Ocean (reka-travel): 87% subagent overhead ($0.82 / $0.94)
    → Evaluate whether subagent delegation was necessary vs inline. Potential savings: $0.57
  LOW   Project lilysbeauty: Cache hit rate 12% (P10 = 25%)
    → Consider CLAUDE.md optimization for better caching. Potential savings: $2.48

Summary: 3 waste signals found. Potential savings: $4.36/day
```

Pipe JSON output into `jq` for scripting:

```bash
burnwatch --json | jq '.waste_signals[] | select(.severity == "high")'
```

## Why Self-Calibrating?

Fixed thresholds break. A \$2 session might be routine for you but expensive for someone else. Burnwatch computes per-project and per-agent baselines from your own data — medians, standard deviations, and percentile cutoffs — so waste signals are always relative to *your* normal.

## Documentation

- [Quickstart](./docs/quickstart.md) — Install, first run, interpreting output
- [Architecture](./docs/architecture.md) — Module design and data flow
- [Contributing](./docs/contributing.md) — Local setup, testing, adding a new agent source

## License

MIT
