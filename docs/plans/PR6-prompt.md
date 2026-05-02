# PR6: Docs, README, CHANGELOG, CI

> **Workflow:** Follow `docs/plans/PR-template.md`. Review `AGENTS.md` behavioral guidelines before implementing.

## Objective

Final polish pass. Write all documentation, set up CI guardrails, create a CHANGELOG, and archive the development plans.

## Files to create/modify

| File | Purpose |
|------|---------|
| `docs/index.md` | Docs entrypoint |
| `docs/quickstart.md` | Install + first run guide |
| `docs/architecture.md` | Module boundaries, data flow, design decisions |
| `docs/contributing.md` | Local setup, testing, adding new harnesses |
| `docs/specs/burnwatch-v1.md` | v1 specification: data model, heuristics, output format |
| `docs/specs/source-interface.md` | Source interface contract, how to add a harness |
| `docs/decisions/2026-05-02-statistical-thresholds.md` | Why P95/P10/2σ |
| `docs/decisions/2026-05-02-source-abstraction.md` | Why TokenEvent + interface |
| `docs/plans/validation.md` | Validation plan (unit, golden, smoke, flag review, CI) |
| `README.md` | Project overview, install, usage, supported harnesses |
| `CHANGELOG.md` | v1.0.0 entry |
| `.golangci.yml` | Linter configuration |
| `.github/workflows/ci.yml` | CI pipeline (test, vet, lint, build) |

## Pre-flight

- [ ] Pull latest main: `git fetch origin && git checkout main && git pull origin main`
- [ ] Create feature branch: `git checkout -b pr6-docs-ci`
- [ ] Verify build environment works on clean main

## Dependencies

PR5 must be merged. This PR runs after the binary is functional.

## Docs — detailed contents

### `docs/index.md`

Follow the structure:
```
# Docs Index
## Current docs
- quickstart.md
- architecture.md
- contributing.md
## Rules
- Treat this file as the docs entrypoint.
- Keep top-level docs/ limited to current docs only.
- Move superseded plans into archive/.
## Current specs
- specs/burnwatch-v1.md
- specs/source-interface.md
## Recent decisions
- decisions/2026-05-02-statistical-thresholds.md
- decisions/2026-05-02-source-abstraction.md
## Plans
- plans/validation.md
## Archive
- archive/ (PR prompts + implementation.md after merge)
```

### `docs/quickstart.md`

```
## Install
go install github.com/yourname/burnwatch@latest

## First run
burnwatch --help
burnwatch
burnwatch --json | jq '.waste_signals'
burnwatch --harness opencode --project lilysbeauty

## Examples
... (piped to jq, filtered by project, weekly report)
```

### `docs/architecture.md`

ASCII art diagram of the pipeline. Module responsibilities. Concurrency model (sequential — all in one goroutine). Pricing model. How new harnesses plug in.

### `docs/contributing.md`

```
## Local setup
git clone ... && cd burnwatch && go mod download

## Testing
go test ./... -cover
go vet ./...

## Adding a new harness
1. Implement Source interface (~100 lines)
2. Add auto-discovery path to Discover()
3. Add pricing entries for its models
4. Add testdata
5. Update README Supported Harnesses

## Release
git tag vX.Y.Z && git push --tags
```

### `docs/specs/burnwatch-v1.md`

Full v1 specification:
- Supported harnesses and their data sources
- TokenEvent schema with field definitions
- 5 waste heuristics with exact formulas
- Output formats (text, JSON)
- CLI flag reference
- Pricing table and update procedure
- Limitations (no real-time, no TUI, Claude Code cost computed not stored)

### `docs/specs/source-interface.md`

```go
// Source is the interface all harness readers must implement.
type Source interface {
    Name() string
    Events() (<-chan TokenEvent, <-chan error)
}
```

- Contract: Events() streams all parsed events, closes channels when done. Error channel receives non-fatal parse warnings.
- Discover() auto-detects installed harnesses.
- Adding a new harness: implement Source, register in Discover(), add testdata, add pricing.

### `docs/decisions/2026-05-02-statistical-thresholds.md`

ADR format:
- **Context**: Waste detection needs thresholds. Hardcoded constants (e.g., "3 duplicate reads = waste") don't account for user behavior differences.
- **Decision**: Use statistical outlier detection (P95/P10/2σ) on user's own data.
- **Alternatives**: Hardcoded thresholds (brittle), ML clustering (overkill for v1), LLM-powered analysis (costs tokens).
- **Consequences**: Self-calibrating, adapts to user. Requires sufficient data for meaningful baselines (≥10 sessions per project). Thresholds re-computed each run.

### `docs/decisions/2026-05-02-source-abstraction.md`

ADR format:
- **Context**: Each AI harness stores session data differently (SQLite, JSONL, aggregate JSON). Need a unified analysis pipeline.
- **Decision**: Define a `TokenEvent` common type + `Source` interface. Each harness maps its native schema to `TokenEvent`.
- **Alternatives**: Per-harness analysis code (duplication), common log format (requires harness changes).
- **Consequences**: New harnesses = ~100 lines of mapping code. Some harness-specific data is lost in translation (e.g., Claude Code's `stop_reason`, OpenCode's `mode`). Acceptable for v1 — focus on token + cost data.

## `README.md`

```markdown
# burnwatch

Find waste in your AI agent sessions. Save money.

Supports:
- [OpenCode](https://github.com/sst/opencode)
- [Claude Code](https://github.com/anthropics/claude-code)

## Install

go install github.com/yourname/burnwatch@latest

## Usage

burnwatch                    # today's waste report
burnwatch --json             # machine-readable
burnwatch --harness opencode # filter to one harness
burnwatch --project my-project

## How it works

Reads your local session data (no cloud, no API keys), computes
statistical baselines, flags outliers. Five heuristics:
1. Cost outliers (sessions 2σ above project median)
2. Low signal sessions (mostly reading, not doing)
3. Subagent overhead (>50% of cost is subagents)
4. Cache underutilization
5. Fragmented sessions (many short sessions, poor caching)

## Output example

$ burnwatch
OpenCode: 1610 sessions | Claude Code: 200 sessions
Today: $1.34 (8 sessions) | This week: $18.72 (52 sessions)

Waste signals:
  HIGH ses_xyz Bright-Butterfly: $1.86 — 4.2x project median
  MED  ses_abc Clever-Squid: 87% subagent overhead ($0.82 / $0.94)

Summary: 2 waste signals. Potential savings: $1.81 / day

## License

MIT
```

## `CHANGELOG.md`

```markdown
# Changelog

## v1.0.0 (2026-05-02)

- Initial release
- OpenCode source (SQLite reader)
- Claude Code source (JSONL reader with subagent discovery)
- 5 waste detection heuristics (cost outlier, low signal, subagent overhead, cache underutilized, session churn)
- Statistical baseline computation (P95/P10/2σ)
- Recommendation engine
- Text output
- JSON output
- CLI flags (--harness, --project, --json, --days, --verbose)
- Golden file tests
- Pricing table for Anthropic and Google/Gemini models
```

## `.golangci.yml`

```yaml
linters:
  enable:
    - errcheck
    - gosimple
    - govet
    - ineffassign
    - staticcheck
    - unused
    - gofmt
    - goimports
    - misspell
    - unconvert
    - prealloc
    - bodyclose

linters-settings:
  gofmt:
    simplify: true
  goimports:
    local-prefixes: github.com/yourname/burnwatch

issues:
  max-issues-per-linter: 0
  max-same-issues: 0
```

## `.github/workflows/ci.yml`

```yaml
name: CI

on:
  push:
    branches: [main]
  pull_request:
    branches: [main]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.22'
      - run: go test ./... -cover -coverprofile=coverage.out
      - run: go vet ./...
      - name: golangci-lint
        uses: golangci/golangci-lint-action@v6
        with:
          version: latest
      - run: go build -o burnwatch .
      - run: ./burnwatch --help
```

## Test Requirements

1. **CI smoke test**: Push to branch → CI runs all 4 steps (test, vet, lint, build) green.
2. **Link checker**: Every link in `docs/index.md` resolves to an existing file.
3. **CHANGELOG format**: v1.0.0 entry includes all major features from PR1–PR5.
4. **README accuracy**: Every CLI example in README matches actual `--help` output.
5. **Archive correctness**: After merging, verify `docs/archive/` contains all 6 PR prompts + implementation plan.

**Coverage target**: CI gate applies — all tests in the repo must pass (coverage target from prior PRs).

## Approach

1. Write all docs (`docs/` + `README.md` + `CHANGELOG.md`) first.
2. Create `.golangci.yml` and `.github/workflows/ci.yml`.
3. Push to branch → verify CI runs green.
4. Merge to main.
5. Archive: move PR prompts + implementation.md → `docs/archive/`, update `docs/index.md`.
6. Tag `v1.0.0`.

## Archive

After PR6 merges, move PR*-prompt.md files and implementation.md into `docs/archive/`. Update `docs/index.md` to point to archive.

## Notes

- `docs/index.md` already exists — edit it, don't replace. Add learnings section.
- README must match actual binary behavior. Use `./burnwatch --help` output as source of truth.
- CHANGELOG follows keepachangelog.com format for v1.0.0.
- CI uses `golangci-lint-action@v6` with `version: latest`.

## Exit criteria

- [ ] Pull latest main
- [ ] Create feature branch from main
- [ ] All docs written and internally consistent
- [ ] README matches actual CLI behavior
- [ ] CHANGELOG has v1.0.0 entry
- [ ] CI passes: `go test` + `go vet` + `golangci-lint` + `go build`
- [ ] `docs/index.md` links to all docs and they exist
- [ ] Archive PR prompts and implementation plan
- [ ] Self-review: follow behavioral guidelines in `AGENTS.md`
- [ ] Document learnings (gotchas, mistakes, patterns, hidden coupling) in `docs/learnings.md`
- [ ] Commit: `docs: add full documentation, README, CHANGELOG, CI`
- [ ] Push to branch `pr6-docs-ci`
- [ ] Open pull request
- [ ] Perform code review
- [ ] Merge to main
- [ ] Tag v1.0.0 after merge: `git tag v1.0.0 && git push --tags`
- [ ] Delete feature branch after merge
