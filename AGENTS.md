# AGENTS.md — Burnwatch

## Project

Burnwatch is a single-binary Go tool that reads local session data from AI agent harnesses (OpenCode, Claude Code) and produces a statistically-calibrated waste report.

- **Language:** Go 1.22+
- **Dependencies:** minimal (`modernc.org/sqlite` for SQLite reading)
- **Test:** `go test ./... -cover`
- **Lint:** `go vet ./... && golangci-lint run`
- **Build:** `go build -o burnwatch .`
- **Coverage target:** >=90% on new code
- **Commit convention:** conventional commits (`feat:`, `fix:`, `docs:`, etc.)
- **Branch naming:** `pr<N>-<description>` (e.g. `pr1-foundation`)

### Docs entrypoint

`docs/index.md` — read on session start. Links to architecture, specs, decisions, and progress.

### PR workflow

When executing a PR prompt from `docs/plans/`:
- Use `docs/PR-template.md` as the canonical workflow
- Follow the exit criteria checklist exactly

---

## Behavioral Guidelines

Guidelines to reduce common LLM coding mistakes. Bias toward caution over speed.

### 1. Think Before Coding

**Don't assume. Don't hide confusion. Surface tradeoffs.**

- State assumptions explicitly. If uncertain, ask.
- If multiple interpretations exist, present them — don't pick silently.
- If a simpler approach exists, say so. Push back when warranted.
- If something is unclear, stop. Name what's confusing. Ask.

### 2. Simplicity First

**Minimum code that solves the problem. Nothing speculative.**

- No features beyond what was asked.
- No abstractions for single-use code.
- No "flexibility" or "configurability" that wasn't requested.
- No error handling for impossible scenarios.
- If you write 200 lines and it could be 50, rewrite it.

### 3. Surgical Changes

**Touch only what you must. Clean up only your own mess.**

- Don't "improve" adjacent code, comments, or formatting.
- Don't refactor things that aren't broken.
- Match existing style, even if you'd do it differently.
- If you notice unrelated dead code, mention it — don't delete it.
- Remove imports/variables/functions that YOUR changes made unused.
- Don't remove pre-existing dead code unless asked.

### 4. Goal-Driven Execution

**Define success criteria. Loop until verified.**

- Transform tasks into verifiable goals: "Add validation" → "Write tests for invalid inputs, then make them pass"
- For multi-step work, state a brief plan with verification checks.
- Strong success criteria let you loop independently. Weak criteria ("make it work") require constant clarification.
