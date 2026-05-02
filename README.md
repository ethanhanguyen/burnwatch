# burnwatch

Find waste in your AI agent sessions. Save money.

Supports [OpenCode](https://github.com/sst/opencode) and [Claude Code](https://github.com/anthropics/claude-code). No cloud, no API keys, no telemetry — reads your local session data directly.

## Supported Harnesses

| Harness | Status | Data Source |
|---------|--------|-------------|
| [OpenCode](https://github.com/sst/opencode) | Supported | `~/.local/share/opencode/opencode.db` (SQLite) |
| [Claude Code](https://github.com/anthropics/claude-code) | Supported | `~/.claude/projects/` (JSONL) |

## Install

```bash
go install github.com/ethanhanguyen/burnwatch@latest
```

## Usage

```bash
burnwatch                    # today's waste report (all harnesses)
burnwatch --json             # machine-readable JSON
burnwatch --harness opencode # filter to one harness
burnwatch --project my-project
burnwatch --days 7           # look back 7 days
burnwatch --verbose          # show all sessions, not just waste
```

## How it works

Reads your local session data, computes statistical baselines from your own behavior, and flags outliers:

1. **Cost outliers** — sessions >2σ above your project median
2. **Low signal** — sessions where the agent reads far more than it produces
3. **Subagent overhead** — >50% of session cost spent on subagents
4. **Cache underutilization** — low prompt cache hit rates
5. **Session churn** — many short sessions losing cached context

All thresholds self-calibrate to your data. No hardcoded constants.

## Output

```
$ burnwatch
OpenCode: 1610 sessions, 1089 subagent sessions
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

JSON mode for piping:

```bash
burnwatch --json | jq '.waste_signals[] | select(.severity == "high")'
```

## Docs

[Full documentation](./docs/index.md) — quickstart, architecture, contributing, specs, and ADRs.

## License

MIT
