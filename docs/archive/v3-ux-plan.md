# v3 UX Plan: Interfaces

> **Status:** Proposed | **Date:** 2026-05-03 | **Dependency:** N1–N4 (v3 behavioral heuristics merged)

## Motivation

v3 adds behavioral waste detection — signals that name specific files, tool calls, and patterns. The current CLI output shows these signals in text. But the user's real workflow is:

1. Run burnwatch → see signals
2. *"Hmm, session abc123 wasted $5.60 on subagent overlap — is that real?"*
3. **Drill into the evidence** → see exactly which files overlapped, which tool calls repeated
4. Decide what to fix

Step 3 is missing. Without drill-down, the user has to trust the heuristics blindly. With v3's richer data (ToolCalls, FileOps, EventIndex), we can show the evidence.

Additionally, two other jobs need interfaces:
- **Live monitoring**: "Is my agent looping right now?" (TUI as watchdog)
- **Sharing + visual exploration**: "Show my team the trends this month" (HTML report)

## Architecture

### Three interfaces, three distinct jobs, one binary

```
burnwatch                    ← "what did I waste?" (one-shot retro CLI)
burnwatch --explain <id>     ← "show me the evidence" (annotated timeline)
burnwatch watch              ← "is anything burning right now?" (live TUI)
burnwatch report --open      ← "share this with my team" (static HTML export)
```

All interfaces reuse the same detection pipeline. `--explain` re-reads events for one session. `watch` runs incremental detection on partial sessions. `report` runs the full pipeline and marshals output to HTML.

### Dependency graph

```
N1-N4 (all behavioral heuristics merged)
  │
  ├── Phase I: --explain <id>       (highest impact, lowest effort)
  │     │
  │     └── Phase II: report         (reuses drill-down data model)
  │           │
  │           └── Phase III: watch   (TUI, incremental detection)
  │
  └── (existing burnwatch CLI stays unchanged, just richer signal text)
```

Phase I first because it's the missing link between "signal detected" and "I know what to fix." Phase II reuses Phase I's event-grouping and annotation logic. Phase III is most complex (incremental detection on partial sessions + BubbleTea).

## Interface Design

### 1. `burnwatch --explain <session-id>`

**Job:** Answer "is this signal real?" by showing annotated event timeline.

**Input:** A session ID from a waste signal.

**Output:** The session's events, ordered by EventIndex, with waste patterns highlighted inline. Top-down: summary → annotated timeline → subagent breakdown.

```
Session ses_abc123 — burnwatch (1h23m, $5.60)
Harness: Claude Code | Model: claude-sonnet-4-20250514
Events: 142 | Tool calls: 87 | Files read: 34 | Subagents: 1

──────────────────────────────────────────
Waste signals in this session:
──────────────────────────────────────────
  HIGH  H12 Subagent "explore-1" re-did parent's work (79% overlap, $2.80)
  MED   H10 Tool loop: read_file("src/handler.go") ×6 ($0.42)

──────────────────────────────────────────
Annotated timeline:
──────────────────────────────────────────
  #12  read  src/handler.go
  #13  read  src/middleware.go
  #14  edit  src/handler.go
  #15  read  src/handler.go       ← [LOOP REPEAT 1/6] same file+tool as #12
  #16  edit  src/handler.go
  #17  read  src/handler.go       ← [LOOP REPEAT 2/6]
  #18  glob  **/*_test.go
  ...
  #42  ▶ subagent:explore-1       ← [SUBAGENT START — $2.80]
  #43    read  src/handler.go     ← [OVERLAP] also read by parent at #12
  #44    read  src/types.go       ← [OVERLAP] also read by parent at #47
  #45    read  src/auth.go        ← [NEW]

──────────────────────────────────────────
Subagent cost breakdown:
──────────────────────────────────────────
  explore-1: $2.80 — 79% overlap with parent (11/14 files shared)
    Shared: src/handler.go, src/types.go, src/middleware.go, ...
    Unique: src/auth.go, src/validation.go

──────────────────────────────────────────
Files re-read without cache:
──────────────────────────────────────────
  config/settings.json  read 5×  (ev #3, #8, #22, #45, #61)  0 cache hits
  src/types.ts          read 4×  (ev #5, #19, #33, #58)       0 cache hits
```

**Design decisions:**
- **Chronological timeline.** Users follow the agent's path. Loop annotations mark repetitions inline — you see the pattern as it happened.
- **Summary at top.** Conclusions first, evidence below. Users don't need to read 142 events to find the waste.
- **Subagent sections indented.** Shows the parent-child hierarchy visually.
- **No pager by default.** Output is terminal-width, scannable. If the session is huge, `--explain` auto-trims to waste-adjacent events + first/last 10 events (configurable with `--explain-full` for unfiltered output).
- **Multiple signals per session handled.** A session can have a loop AND subagent overlap. Both are annotated on the same timeline with different markers.

**Implementation:**
- New function `output/explain.go`: `FormatExplain(sessionID string, events []TokenEvent, signals []WasteSignal, trees []SubagentTree) string`
- Groups events by SessionID, sorts by EventIndex, walks the timeline adding annotations
- Subcommand wiring in `cmd/root.go`: `explain` flag, collect events, filter to session, call FormatExplain
- ~200 lines of formatting code + tests with scenario fixtures

### 2. `burnwatch watch` (TUI, live monitoring)

**Job:** Watchdog for active sessions. Alert on in-progress waste.

**How it works:**
- Polls data sources every N seconds (default: 5s)
- Diffs for new events since last poll
- Runs incremental waste detection on partial sessions (session hasn't ended yet)
- Renders a BubbleTea TUI with two panes: sessions + alerts

```
┌─ burnwatch watch [⟳ 5s] ── Active: 3 ── Rate: $0.18/min ── Today: $4.32 ────────────┐
│                                                                                       │
│  ACTIVE SESSIONS                         │  ALERTS (last 5 min)                       │
│  ─────────────────────────────────────── │  ──────────────────────────────────────   │
│  ● ses_abc123  Claude Code  12m ago     │  ⚠ HIGH  14:32  ses_abc123                │
│    burnwatch    $2.14    sonnet-4        │    Tool loop: read_file("src/              │
│    ██████████▓▓▓▓░░  48 events           │    handler.go") called 6 times.            │
│    Last: read src/handler.go             │    Estimated waste: $0.42                  │
│                                           │                                             │
│  ● ses_def456  OpenCode  8m ago          │  ⚠ MED   14:28  ses_def456                │
│    burnwatch    $0.82    deepseek         │    File re-read: config/settings           │
│    █████░░░░░░░░░░  23 events            │    .json read 3× without cache.            │
│    Last: edit docs/README.md             │    Estimated waste: $0.15                  │
│                                           │                                             │
│  ◌ ses_ghi789  Claude Code  34m ago      │  ◉ INFO  14:30  ses_ghi789                │
│    my-app       $1.36    sonnet-4        │    Subagent "explore-1" spawned.           │
│    ██████████████▓▓  85 events           │    Cost so far: $0.40                      │
│    Last: bash go test ./...              │                                             │
│                                           │                                             │
│                                           │  [TAB] switch pane  [A] alerts            │
│                                           │  [↑↓] navigate     [ENTER] drill-in       │
│                                           │  [Q] quit                                 │
└───────────────────────────────────────────────────────────────────────────────────────┘
```

**Design decisions:**

1. **It's not a retrospective dashboard — it's a watchdog.** No tabs for `[Dashboard] [Sessions] [Waste] [Recommend]`. Those are for historical browsing, which the CLI and HTML report handle. The TUI shows what's happening right now.

2. **Two-pane layout.** Sessions on the left (what's active), alerts on the right (what was detected). 50/50 split. No hidden tabs — everything visible at once.

3. **Session status dots:**
   - ● Green = active (new events in last poll interval)
   - ◌ Yellow = idle (no new events in >30s, session may have ended)
   - ◎ Red = waste alert within last minute

4. **Sparkline per session.** A dense 10-char mini-bar showing event frequency over the last 10 poll intervals. Instant visual of whether the agent is active or stalled.

5. **Alert pane** shows detections from incremental analysis. New alerts appear at the top with a timestamp. Older alerts fade as they scroll down. Max 20 alerts displayed.

6. **ENTER on a session** → replaces bottom alerts pane with an inline drill-down (same format as `--explain` but scoped to events seen so far). ESC to return.

7. **ENTER on an alert** → jumps to the relevant session's drill-down.

8. **Incremental detection nuance.** On partial sessions, thresholds are relaxed. A tool loop of 3 repetitions on a 15-event session is more alarming than 3/200. Incremental heuristics use per-event-rate thresholds, not absolute counts.

**What the TUI does NOT do:**
- No session history browser (use `burnwatch` CLI)
- No time-series charts (use `burnwatch report`)
- No project selector or date range picker (use `--project`, `--days` flags)
- No config editing

**Dependencies:**
- `github.com/charmbracelet/bubbletea` — TUI framework
- `github.com/charmbracelet/lipgloss` — terminal styling
- No other new dependencies

**Implementation:**
- `cmd/watch.go`: TUI model, update/view functions
- `analyze/watch.go`: incremental detection (diff-based, partial-session aware)
- `output/watch_format.go`: rendering helpers for session rows, alert rows, sparklines
- ~500 lines total (TUI + incremental detection + formatting)

### 3. `burnwatch report --open`

**Job:** Generate a visual, shareable report for team communication.

**Output:** Single self-contained HTML file. No server, no build step.

```
$ burnwatch report --days 30 --output burnwatch-may2026.html --open
Collecting events... 1,247 events across 4 projects.
Computing baselines... done.
Detecting waste... 23 signals found.
Writing report to burnwatch-may2026.html... done.
Opening in browser...
```

**HTML report structure (one long-scrolling page, no SPA navigation):**

```
┌─────────────────────────────────────────────────────┐
│  burnwatch — May 2026                                │
│  4 projects · $1,247.32 total · 23 waste signals     │
│  Generated 2026-05-03 14:30                          │
├─────────────────────────────────────────────────────┤
│                                                      │
│  COST OVER TIME              [line chart]            │
│  Daily cost with 7-day moving average                │
│  ┌──────────────────────────────────────────┐       │
│  │     /\/\                                  │       │
│  │    /    \    /\    /\/\                   │       │
│  │   /      \/\/  \/\/    \                  │       │
│  │  Apr 4           Apr 18        May 2     │       │
│  └──────────────────────────────────────────┘       │
│                                                      │
│  WASTE BY TYPE              [stacked bar]            │
│  Per project, segmented by signal type               │
│  ┌──────────────────────────────────────────┐       │
│  │ burnwatch ████████▓▓▓▓▓▓░░░░ $32.40     │       │
│  │ my-app    ██████▓▓▓▓░░░░░░░░ $18.20     │       │
│  │ api       ████▓▓░░░░░░░░░░░░ $6.80      │       │
│  │           ■ loop  ▓ reread  ░ overlap   │       │
│  └──────────────────────────────────────────┘       │
│                                                      │
│  TOP WASTED FILES           [horizontal bar]         │
│  Files read most without cache hits                  │
│  ┌──────────────────────────────────────────┐       │
│  │ config/settings.json  ████████████  23×  │       │
│  │ src/types.ts          ██████████    18×  │       │
│  │ docs/README.md        ████████      14×  │       │
│  └──────────────────────────────────────────┘       │
│                                                      │
│  SUBAGENT COST TREE         [treemap]                │
│  ┌──────────────────────────────────────────┐       │
│  │ ┌──────────────┐ ┌──────────┐            │       │
│  │ │  ses_abc123  │ │ explore-1│ ┌─────┐   │       │
│  │ │   $12.40     │ │  $2.80   │ │expl2│   │       │
│  │ └──────────────┘ └──────────┘ │$1.20│   │       │
│  │              ┌───────────┐    └─────┘   │       │
│  │              │ ses_def456 │              │       │
│  │              │   $8.30    │              │       │
│  │              └───────────┘              │       │
│  └──────────────────────────────────────────┘       │
│                                                      │
│  MODEL COST BREAKDOWN       [donut chart]            │
│  ┌───────────┐                                       │
│  │  /‾‾‾‾\   │  sonnet-4:   $892  (71%)            │
│  │ │       │  │  opus-4:     $214  (17%)            │
│  │ │       │  │  haiku-3.5:  $87   (7%)             │
│  │  \_____/   │  deepseek:   $54   (4%)             │
│  └───────────┘                                       │
│                                                      │
│  WASTE SIGNALS               [interactive table]     │
│  ┌──────────────────────────────────────────┐       │
│  │ Severity │ Session   │ Project   │ Waste │       │
│  │──────────│───────────│───────────│───────│       │
│  │ HIGH  ▲  │ ses_abc.. │ burnwatch │ $5.60 │       │
│  │ HIGH  ▲  │ ses_def.. │ my-app    │ $3.20 │       │
│  │ MED   ▶  │ ses_ghi.. │ burnwatch │ $1.10 │       │
│  │ ...                                     │       │
│  │                    [sort] [filter]      │       │
│  └──────────────────────────────────────────┘       │
│                                                      │
│  Click any row to expand inline session drill-down.  │
│                                                      │
├─────────────────────────────────────────────────────┤
│  burnwatch v3.2.0 · generated 2026-05-03T14:30:00Z  │
└─────────────────────────────────────────────────────┘
```

**Design decisions:**

1. **Single scrolling page, not a SPA.** No navigation, no tabs, no URL routing. The report is a snapshot — read top-to-bottom. The only interactive element is the expandable signal table rows (show/hide session drill-down).

2. **Every visualization answers a specific question:**
   - Cost-over-time → "Are we trending up or down?"
   - Waste-by-type → "Where should I focus my attention?"
   - Top wasted files → "Which files need caching?" (actionable)
   - Subagent treemap → "Which subagents are expensive?" (tree = hard in text)
   - Model breakdown → "Are we using expensive models unnecessarily?"
   - Signal table → "What exactly was flagged?"

3. **Treemap, not sunburst.** The original proposal mentioned sunburst. Treemap is better for cost data — area corresponds to cost proportion. Sunburst adds navigational complexity (rings of hierarchy) that doesn't add insight for 2-level parent→child trees.

4. **No real-time updates.** This is a snapshot report. For live monitoring, use `burnwatch watch`.

5. **Self-contained file.** All data embedded as `<script>const REPORT = {...};</script>`. Chart.js loaded from CDN with a `<script src="...">` tag. No npm, no webpack, no server. The file is ~100KB for 30-day data.

6. **Progressive enhancement.** If Chart.js CDN fails to load, the signal table still works. Charts degrade gracefully to "Chart requires JavaScript" placeholder.

**Implementation:**
- `output/report.go`: `FormatReport(events, baselines, signals, trees, trends) string` — returns complete HTML string
- `cmd/root.go`: `report` subcommand, `--days`, `--output`, `--open` flags
- `--open` uses platform-specific open command (`open` on macOS, `xdg-open` on Linux, `start` on Windows) via `os/exec`
- ~300 lines (HTML template + data marshaling + chart config)

### 4. Enhanced CLI output (existing `burnwatch`)

The existing text format stays but gets richer signal bodies for v3 behavioral signals:

```
Waste signals:
  HIGH ses_abc123 (burnwatch): $5.60 — subagent re-did parent's work
    11/14 files overlap between parent and subagent "explore-1".
    → Limit subagent context to only new files. Savings: $2.80

  MED ses_def456 (burnwatch): $3.20 — file re-read without cache
    5 files re-read ≥3× without cache hits.
    Top: config/settings.json (5×), src/types.ts (4×)
    → Enable prompt caching. Savings: $1.60

  HIGH ses_ghi789 (burnwatch): $0.84 — tool call loop
    read_file("src/handler.go") called 12 consecutive times.
    Pattern: read→edit→read→edit (6 cycles).
    → Check for broken edit tool or infinite verify loop. Savings: $0.42
```

Implemented in N2/N3 as part of heuristic output. No structural changes to the CLI format — signals already carry `Detail` strings and `Recommendation` objects. v3 signals just fill them with richer text.

---

## What changed from the original proposal

| Aspect | Original | Refined | Why |
|--------|----------|---------|-----|
| **TUI layout** | 4 tabs (`[Dashboard] [Sessions] [Waste] [Recommend]`) | 2 panes (sessions + alerts) | Tabs solve a navigation problem for dashboards. A watchdog needs visibility, not navigation. |
| **TUI role** | Retrospective dashboard | Live monitoring watchdog | Retrospection belongs in the CLI and HTML report. The TUI's unique value is real-time awareness. |
| **Web dashboard** | Running Go HTTP server + SPA | `burnwatch report` → static HTML | No process to manage. Portable file. Shareable. |
| **Drill-down** | "Session deep-dive" buried in web | `--explain <id>` in terminal | Terminal is where the user already is when they see the signal. No context switch. |
| **Sharing** | Not addressed | `report --open` → portable HTML | Team leads need to show, not explain. A screenshot or HTML file is more useful than terminal output. |
| **Subagent visualization** | Sunburst or treemap (web) | Treemap (HTML report) | Treemap maps cost to area directly. Sunburst adds hierarchy rings that don't add insight for 2-level trees. |
| **New dependencies** | BubbleTea, Lip Gloss, net/http router, Chart.js | BubbleTea (+Lip Gloss transitive), Chart.js (CDN, no build) | Same BubbleTea for TUI. No server framework. Chart.js loaded from CDN — no bundler. |

---

## Implementation phasing

### Phase I: `--explain <id>` (after N4 merged)

**Files:**
| File | Purpose |
|------|---------|
| `output/explain.go` | `FormatExplain()` — annotated timeline formatting |
| `output/explain_test.go` | Scenario tests with varied waste patterns |
| `testdata/scenarios/explain_loop.jsonl` | Session with tool loop |
| `testdata/scenarios/explain_overlap.jsonl` | Session with subagent overlap |
| `testdata/scenarios/explain_multi.jsonl` | Session with loop + re-read + overlap |
| `cmd/root.go` | Add `--explain <id>` flag and handler |

**Success criteria:**
- [ ] `burnwatch --explain ses_abc123` shows annotated timeline for that session
- [ ] Loop repeats annotated inline with `[LOOP REPEAT N/M]`
- [ ] Subagent overlap files annotated with `[OVERLAP]`
- [ ] Unknown session ID prints "No session found" error
- [ ] Session with no signals prints timeline without annotations
- [ ] Lint, build, test pass. >=90% coverage on new code.

### Phase II: `burnwatch report` (after Phase I)

**Files:**
| File | Purpose |
|------|---------|
| `output/report.go` | `FormatReport()` — complete HTML generation |
| `output/report_test.go` | Validate HTML structure, embedded data correctness |
| `cmd/root.go` | Add `report` subcommand with `--days`, `--output`, `--open` flags |

**Success criteria:**
- [ ] `burnwatch report --days 30 --output report.html` writes valid HTML
- [ ] HTML renders correctly in Chrome/Firefox/Safari (Chart.js CDN load)
- [ ] All 6 visualizations present with correct data
- [ ] Signal table rows expand to show session drill-down
- [ ] `--open` flag launches browser
- [ ] Embedded REPORT JSON matches `burnwatch --json` output structurally
- [ ] Lint, build, test pass. >=90% coverage on new code.

### Phase III: `burnwatch watch` (after Phase II)

**Files:**
| File | Purpose |
|------|---------|
| `cmd/watch.go` | BubbleTea TUI model, update loop, key bindings |
| `analyze/watch.go` | Incremental event diffing, partial-session detection |
| `output/watch_format.go` | Session row, alert row, sparkline rendering |
| `cmd/watch_test.go` | TUI model unit tests (state transitions) |

**Dependencies added to go.mod:**
- `github.com/charmbracelet/bubbletea`
- `github.com/charmbracelet/lipgloss` (transitive via BubbleTea)

**Success criteria:**
- [ ] `burnwatch watch` starts TUI, shows active sessions
- [ ] Session list updates every N seconds (configurable `--interval`)
- [ ] New events in active sessions update sparklines and cost
- [ ] Tool loops detected mid-session appear in alerts pane
- [ ] ENTER on session shows inline drill-down (reused from `--explain`)
- [ ] ENTER on alert navigates to relevant session
- [ ] Q quits cleanly (no dangling goroutines)
- [ ] Idle sessions (>30s no events) shown with ◌ indicator
- [ ] All poll failures (file deleted mid-watch, corrupt line) handled gracefully
- [ ] Lint, build, test pass. >=90% coverage on new code.

---

## Technical constraints

1. **Single binary.** All commands (`explain`, `report`, `watch`) are flags/subcommands on `burnwatch`. No separate binaries.

2. **Minimal dependencies.** BubbleTea + Lip Gloss are the only new Go dependencies (Phase III). No JavaScript build toolchain — Chart.js loads from CDN in the HTML.

3. **Reuse detection pipeline.** `--explain` and `watch` call the same `DetectWaste()` function. `report` runs the full pipeline. No duplicate detection logic.

4. **No config schema changes for interfaces.** The existing `.burnwatch.toml` doesn't need new sections for UX. `watch --interval` and `report --days` use CLI flags only.

5. **Graceful degradation.** If a source has no ToolCalls data (pre-N1 harness), `--explain` shows "No tool call data available for this session." HTML report skips tool-level charts and shows aggregate-only visualizations.

---

## Comparison: what's NOT being built

| Idea | Verdict | Reason |
|------|---------|--------|
| Running web server (`burnwatch serve`) | Skipped | Static HTML report achieves the same goal (visual exploration + sharing) without an always-on process. |
| Multi-tab TUI dashboard | Skipped | Tabs solve navigation for dashboards. The TUI is a watchdog — two visible panes beat hidden tabs. |
| Web-based drill-down (SPA route per session) | Skipped | `--explain <id>` in the terminal is where the user already is. No browser context switch. |
| Sunburst chart | Replaced by treemap | Treemap maps area → cost. Sunburst adds hierarchy rings that don't help for 2-level subagent trees. |
| `watch` in browser (websocket server) | Skipped | Terminal is the right place for a watchdog — it's always visible, no browser tab to find. |

---

## Exit criteria (this plan)

- [ ] Phase I–III implemented and merged
- [ ] All three interface commands documented in README
- [ ] `docs/learnings.md` updated with UX gotchas
- [ ] This plan archived in `docs/archive/` after completion
