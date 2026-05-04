# Changelog

## Unreleased

### Added
- 4 new behavioral waste signals: tool call loop, file re-read, subagent overlap, session restart
- Expanded embedded pricing: +22 model families (OpenAI/GPT, DeepSeek, xAI/Grok, MoonshotAI/Kimi, Qwen, more)
- Model name normalization: strips provider prefixes and date suffixes for better pricing lookups
- HTML report with charts (cost over time, model breakdown, waste by type, subagent tree, signals ledger)
- `add` CLI command for incremental data import

### Changed
- Behavioral signals (`tool_loop`, `file_reread`, `subagent_overlap`, `session_restart`) enabled by default
- `report` subcommand now loads config from file and respects all `--no-*` flag overrides (consistent with main path)

## v0.1.0 (2026-05-02)

### Added
- OpenCode source: SQLite reader for session data
- Claude Code source: JSONL reader with subagent discovery
- 5 waste detection heuristics: cost outlier, low signal, subagent overhead, cache underutilized, session churn
- Statistical baseline computation (P95/P10/2σ) self-calibrating to user data
- Recommendation engine: actionable text for each waste signal
- Text output with severity markers and aligned columns
- JSON output for piping to jq
- CLI flags: `--harness`, `--project`, `--json`, `--days`, `--verbose`, `--db`
- Golden file tests for output stability
- Pricing table for Anthropic and Google/Gemini models
- Full documentation: quickstart, architecture, contributing, specs, ADRs
- CI pipeline: `go test`, `go vet`, `golangci-lint`, `go build`
