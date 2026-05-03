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
```

## Interpreting output

```
$ burnwatch
OpenCode: 42 sessions, 8 subagent sessions
Today: $2.34 (8 sessions) | This week: $14.21 (37 sessions)

Waste signals:
  HIGH ses_abc Bright-Butterfly (lilysbeauty): $1.86 — 3.4x project median ($0.55)
    → Investigate session for unnecessary loops or re-prompts. Potential savings: $1.31
  MED  ses_def Cool-Ocean (reka-travel): 87% subagent overhead ($0.82 / $0.94)
    → Evaluate whether subagent delegation was necessary vs inline. Potential savings: $0.57
  LOW  Project lilysbeauty: Cache hit rate 12% (P10 = 25%)
    → Consider CLAUDE.md optimization for better caching. Potential savings: $2.48

Summary: 3 waste signals found. Potential savings: $4.36 / day
```

Three severity levels:
- **HIGH**: Cost outlier — session costs >2σ above project median. Check for agent loops, unnecessary re-prompts, or wrong model choice.
- **MEDIUM**: Low signal or subagent overhead. The agent is spending time/money but not producing output, or delegating too much.
- **LOW**: Cache underutilization or session churn. Optimization opportunities.

## Piping to jq

```bash
burnwatch --json | jq '.waste_signals[] | select(.severity == "high")'
burnwatch --json | jq '.potential_savings'
burnwatch --json | jq '.projects[] | {name, total_cost}'
```

## Updating prices

Burnwatch embeds a pricing table for Anthropic and Google/Gemini models. Update it when prices change:

```bash
burnwatch update-prices --help   # (planned for v1.1)
```

## Privacy

Burnwatch reads your local session data — no cloud, no API keys, no telemetry. Token counts and costs only. Never reads your prompts or code.
