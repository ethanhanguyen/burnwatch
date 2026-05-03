# Changelog

## v1.0.0 (2026-05-02)

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
