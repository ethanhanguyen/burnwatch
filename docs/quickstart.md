# Quickstart

## Install

```bash
go install github.com/ethanhanguyen/burnwatch@latest
```

## First run

```bash
burnwatch
```

If you have OpenCode or Claude Code installed, burnwatch auto-discovers the data and prints a waste report.

If nothing is found:
```
$ burnwatch
No AI harnesses detected. Supported: OpenCode, Claude Code.
Make sure you've run at least one session.
```

## Commands

```bash
burnwatch                  # today's waste report (all harnesses)
burnwatch --json           # machine-readable JSON output
burnwatch --harness opencode    # filter to one harness
burnwatch --project reka-travel # filter to specific project
burnwatch --days 7              # look back 7 days
burnwatch --verbose             # show all sessions, not just waste
burnwatch --min-cost 5          # hide signals below $5
burnwatch --show-trends         # show time-trend direction (↑↓)
burnwatch --no-churn            # disable session churn detection
burnwatch --config ./my.toml    # custom config path
```

### Signal toggles

```bash
burnwatch --no-cost-outlier          # disable cost outlier detection
burnwatch --no-low-signal            # disable low output/input ratio detection
burnwatch --no-subagent-overhead     # disable subagent overhead detection
burnwatch --no-cache-underutil       # disable cache underutilization detection
burnwatch --no-churn                 # disable session churn detection
burnwatch --no-tool-loop             # disable tool call loop detection
burnwatch --no-file-reread           # disable file re-read detection
burnwatch --no-subagent-overlap      # disable subagent overlap detection
burnwatch --no-session-restart       # disable session restart detection
```

## Interpreting output

```
$ burnwatch
OpenCode: 42 sessions, 8 subagent sessions
Today: $2.34 (8 sessions) | This week: $14.21 (37 sessions)

Project lilysbeauty: $24.50 (median $0.88/session)

Waste signals:
  HIGH ses_abc Bright-Butterfly (lilysbeauty): $1.86 — 3.4x project baseline (μ = $0.55)
    Model: claude-sonnet-4-6, 12.3K in / 150.0K out
    → Investigate session for unnecessary loops or re-prompts. Potential savings: $1.31
  MED  ses_def Cool-Ocean (reka-travel): 87% subagent overhead ($0.82 / $0.94)
    → Evaluate whether subagent delegation was necessary vs inline. Potential savings: $0.57
  LOW  Project lilysbeauty: Cache hit rate 12% (P10 = 25%)
    → Consider CLAUDE.md optimization for better caching. Potential savings: $2.48

Summary: 3 waste signals found. Potential savings: $4.36
```

With `--show-trends`:

```
$ burnwatch --show-trends
...
Project lilysbeauty: $24.50 (median $0.88/session)

Trends:
  Cost:    $12.34/wk → $8.76/wk (↓ 29%)
  Sessions: 18/wk → 14/wk (↓ 22%)
  Output/input ratio: 0.12 → 0.18 (↑ 50%)

Waste signals:
...
```

Three severity levels:
- **HIGH**: Cost outlier, input overconsumption, or tool call loops. Session costs or input tokens significantly above project baseline. Check for agent loops, unnecessary re-prompts, or wrong model choice.
- **MEDIUM**: Low signal, subagent overhead, output explosion, file re-reads, subagent overlap, or session restarts. The agent is spending resources without producing output, or duplicating work across sessions.
- **LOW**: Cache underutilization, session churn, or token efficiency issues. Optimization opportunities.

## Piping to jq

```bash
burnwatch --json | jq '.waste_signals[] | select(.severity == "high")'
burnwatch --json | jq '.potential_savings'
burnwatch --json | jq '.projects[] | {name, total_cost}'
```

## Updating prices

Burnwatch embeds a pricing table covering Anthropic, Google/Gemini, OpenAI/GPT, DeepSeek, xAI/Grok, MoonshotAI/Kimi, Qwen, and other providers. Instance-specific prices are fetched from OpenRouter on startup. Use `--no-fetch-pricing` to skip the network call and rely on embedded pricing alone.

## Privacy

Burnwatch reads your local session data — no cloud, no API keys, no telemetry. Token counts and costs only. Never reads your prompts or code.
