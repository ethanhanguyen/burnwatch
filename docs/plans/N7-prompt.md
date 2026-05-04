# N7: Live Monitoring TUI — `burnwatch watch`

> **Workflow:** Follow `docs/PR-template.md`. Review `AGENTS.md` behavioral guidelines before implementing.
> **UX spec:** `docs/plans/v3-ux-plan.md` Phase III

## Objective

Add `burnwatch watch` — a BubbleTea-based TUI that monitors active sessions in real-time. Detects waste incrementally on partial sessions and alerts as patterns emerge. Single view: session list on the left, alerts on the right.

## Success Criteria

- [ ] `burnwatch watch` starts TUI, polls data sources every 5s (configurable `--interval`)
- [ ] Session list shows: status dot (● active / ◌ idle >30s), session ID, harness, duration, project, cost-so-far, model, event frequency sparkline, last action
- [ ] Alert pane shows detected waste patterns with timestamps
- [ ] Incremental loop detection: flags tool call repeats on partial sessions as they happen
- [ ] Incremental re-read detection: flags file re-reads ≥3 on partial sessions
- [ ] Subagent spawn events appear as alerts
- [ ] ENTER on session → inline drill-down (reuses `--explain` formatting, scoped to seen events)
- [ ] ENTER on alert → navigates to relevant session
- [ ] Q quits cleanly (no dangling goroutines, no zombie file watchers)
- [ ] Idle sessions (>30s no new events) shown with ◌ indicator
- [ ] All poll failures (file deleted mid-watch, corrupt JSONL line) handled gracefully with brief error flash
- [ ] TUI resizes correctly on terminal resize
- [ ] All existing tests pass unchanged

## Dependencies

- **Must merge first:** N5 (needs `--explain` for inline drill-down), N2 (H10/H11 incrementally)
- **External dependencies:**
  - `github.com/charmbracelet/bubbletea` — TUI framework
  - `github.com/charmbracelet/lipgloss` — terminal styling (expected to be transitive via bubbletea)
- **Can be parallel with:** N6 (report — separate output path)
- **Breaking changes / Migrations needed:** None (new command, new packages)

---

## Pre-flight

- [ ] Pull latest main: `git fetch origin && git checkout main && git pull origin main`
- [ ] Create feature branch: `git checkout -b n7-watch`
- [ ] Verify build environment works on clean main
- [ ] `go get github.com/charmbracelet/bubbletea`
- [ ] `go mod tidy`

## Files

| File | Purpose | Notes |
|------|---------|-------|
| `cmd/watch.go` | BubbleTea model, update loop, key bindings | New, ~250 lines |
| `analyze/watch.go` | Incremental event diffing, partial-session waste detection | New, ~200 lines |
| `analyze/watch_test.go` | Tests for incremental detection logic | New, ~250 lines |
| `output/watch_format.go` | Session row, alert row, sparkline rendering with lipgloss | New, ~150 lines |
| `cmd/root.go` | Add `watch` subcommand dispatch | Modify, ~5 lines |

---

## Implementation

### `analyze/watch.go` — Incremental detection

**Core concept:** `burnwatch watch` polls sources every N seconds. Each poll returns new events since the last poll. The incremental detector diffs these against previously seen events and runs partial-session heuristics.

**Data model:**

```go
// WatchState tracks everything the TUI needs to render across polls.
type WatchState struct {
    // Sessions: keyed by SessionID
    Sessions map[string]*WatchSession

    // Alerts: time-ordered list of recent waste detections
    Alerts []WatchAlert

    // Total stats
    TotalActiveSessions int
    LiveCostRate        float64 // $/min across all active sessions
    TodayCost           float64
}

// WatchSession represents a single active or recently-active session.
type WatchSession struct {
    SessionID       string
    Project         string
    Harness         string
    Model           string
    StartedAt       time.Time
    LastEventAt     time.Time
    Events          []source.TokenEvent
    Cost            float64
    InputTokens     int64
    OutputTokens    int64
    IsSubagent      bool
    ParentSessionID string
    AgentType       string

    // For sparkline: event count per poll interval (last 10 intervals)
    EventFreq       [10]int

    // Last tool call / file op for display
    LastAction      string
}

// WatchAlert is a waste detection that appeared during live monitoring.
type WatchAlert struct {
    Time       time.Time
    SessionID  string
    Project    string
    Severity   string // "high", "medium", "low"
    Reason     string // tool_call_loop, file_reread, subagent_overhead, ...
    Detail     string
    Fading     bool   // true if >5 min old (for TUI dimming)
}
```

**Functions:**

```go
// DiffEvents returns events from `all` that are not in `seen`.
// Comparison: (SessionID, EventIndex, Timestamp) tuple.
// Also returns newly-completed sessions (no new events in last poll + session file unchanged).
func DiffEvents(all []source.TokenEvent, seen map[string]map[int]bool) (newEvents []source.TokenEvent, completedSessions []string)

// UpdateState merges new events into the watch state.
// Updates WatchSession.Events, WatchSession.EventFreq sparkline, last action.
// Detects new sessions, marks idle sessions.
// Runs incremental waste detection on updated sessions.
func UpdateState(state *WatchState, newEvents []source.TokenEvent, cfg config.Config) []WatchAlert

// DetectPartialWaste runs behavioral heuristics on a single session's current events.
// Uses relaxed thresholds: tool loop fires at N/2 repeats (vs N for complete sessions),
// file re-read fires at N/2 reads. Severity capped at "medium" for partial sessions.
func DetectPartialWaste(session *WatchSession, cfg config.Config) []WatchAlert
```

**Incremental detection thresholds:**

| Waste type | Complete session threshold | Partial session threshold | Rationale |
|-----------|---------------------------|--------------------------|-----------|
| tool_call_loop | 5 repeats | 3 repeats | 3 repeats in 15 events is worse than 5/100 |
| file_reread | 4 reads, 0 cache | 2 reads, 0 cache | On partial sessions, even 2 re-reads raises suspicion |
| subagent_overhead | 50% | 30% | Subagent overhead climbs with session length; earlier is more alarming |
| cost_outlier | μ + 2σ | skipped | Can't baseline cost until session completes |

**Partial session event indexing:** New events from `DiffEvents` are appended to `WatchSession.Events`. EventIndex gap-checking is not needed — the incremental detector uses the absolute event count, not the gap between indices.

**Session completion detection:** A session is "idle" if `LastEventAt` is >30s ago AND the session's source file hasn't grown in the last poll. A session is "completed" if it was idle for 2 consecutive polls and the source file exists (hasn't been deleted). Completed sessions get their full waste detection run (not partial).

### `cmd/watch.go` — BubbleTea TUI

**Model:**

```go
type watchModel struct {
    state      *analyze.WatchState
    interval   time.Duration
    sources    []source.Source
    err        error

    // UI state
    selectedPane  int  // 0 = sessions, 1 = alerts
    selectedRow   int
    scrollOffset  int
    drillSession  string  // non-empty when viewing session drill-down

    // Dimensions
    width  int
    height int
}

func (m watchModel) Init() tea.Cmd {
    return tea.Batch(
        pollSourcesCmd(m.sources),
        tickCmd(m.interval),
    )
}

func (m watchModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        switch msg.String() {
        case "q", "ctrl+c":
            return m, tea.Quit
        case "enter":
            return handleEnter(m)
        case "esc":
            if m.drillSession != "" {
                m.drillSession = ""
            }
            return m, nil
        case "tab":
            m.selectedPane = 1 - m.selectedPane
            m.selectedRow = 0
            return m, nil
        case "up", "k":
            m.selectedRow = max(0, m.selectedRow-1)
            return m, nil
        case "down", "j":
            maxRow := m.state.SessionCount()
            if m.selectedPane == 1 {
                maxRow = len(m.state.Alerts)
            }
            m.selectedRow = min(maxRow-1, m.selectedRow+1)
            return m, nil
        }
    case pollResultMsg:
        // Merge new events, update state, return any alerts
        alerts := analyze.UpdateState(m.state, msg.events, config.Defaults())
        for _, a := range alerts {
            m.state.Alerts = append([]analyze.WatchAlert{a}, m.state.Alerts...)
        }
        // Trim alerts to last 20
        if len(m.state.Alerts) > 20 {
            m.state.Alerts = m.state.Alerts[:20]
        }
        return m, tea.Batch(pollSourcesCmd(m.sources), tickCmd(m.interval))
    case tickMsg:
        // Tick triggers a poll
        return m, pollSourcesCmd(m.sources)
    case tea.WindowSizeMsg:
        m.width = msg.Width
        m.height = msg.Height
        return m, nil
    case errMsg:
        m.err = msg.err
        return m, nil
    }
    return m, nil
}
```

**Commands:**

```go
type pollResultMsg struct {
    events []source.TokenEvent
}

func pollSourcesCmd(sources []source.Source) tea.Cmd {
    return func() tea.Msg {
        var allEvents []source.TokenEvent
        for _, src := range sources {
            evCh, errCh := src.Events()
            for e := range evCh {
                allEvents = append(allEvents, e)
            }
            // Drain errors
            for range errCh {}
        }
        return pollResultMsg{events: allEvents}
    }
}

type tickMsg struct{}

func tickCmd(d time.Duration) tea.Cmd {
    return tea.Tick(d, func(t time.Time) tea.Msg {
        return tickMsg{}
    })
}
```

**View (rendering):**

```go
func (m watchModel) View() string {
    if m.drillSession != "" {
        return m.renderDrillDown()
    }

    header := m.renderHeader()
    sessions := m.renderSessionPane()
    alerts := m.renderAlertPane()

    // Two-pane layout: sessions | alerts
    leftWidth := m.width / 2
    rightWidth := m.width - leftWidth - 1

    return lipgloss.JoinHorizontal(
        lipgloss.Top,
        lipgloss.NewStyle().Width(leftWidth).Render(sessions),
        lipgloss.NewStyle().Width(rightWidth).Render(alerts),
    ) + "\n" + m.renderFooter()
}
```

**Layout spec (matches UX plan):**

```
┌─ burnwatch watch [⟳ 5s] ── Active: 3 ── Rate: $0.18/min ── Today: $4.32 ──┐
│ Session list (left pane)               │ Alert list (right pane)             │
│ ● ses_abc123  Claude Code  12m         │ ⚠ HIGH 14:32 ses_abc123            │
│   burnwatch  $2.14  sonnet-4           │   Tool loop: read_file("src/        │
│   ██████████▓▓▓▓░░  48 ev              │   handler.go") called 6 times       │
│   Last: read src/handler.go            │                                      │
│                                        │ ⚠ MED  14:28 ses_def456            │
│ ● ses_def456  OpenCode  8m             │   File re-read: config/set...       │
│   burnwatch  $0.82  deepseek           │                                      │
│                                        │                                      │
│ ◌ ses_ghi789  Claude Code  34m         │ [TAB] pane  [↑↓] nav                │
│   my-app  $1.36  sonnet-4              │ [ENTER] drill  [Q] quit             │
└──────────────────────────────────────────────────────────────────────────────┘
```

**Styles (lipgloss):**

```go
var (
    statusActive = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))  // green ●
    statusIdle   = lipgloss.NewStyle().Foreground(lipgloss.Color("220")) // yellow ◌
    statusAlert  = lipgloss.NewStyle().Foreground(lipgloss.Color("196")) // red ◎

    severityHigh   = lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true) // red
    severityMed    = lipgloss.NewStyle().Foreground(lipgloss.Color("220"))             // yellow
    severityLow    = lipgloss.NewStyle().Foreground(lipgloss.Color("39"))              // blue

    sparklineBar   = lipgloss.NewStyle().Foreground(lipgloss.Color("99"))  // purple bar
    sparklineEmpty = lipgloss.NewStyle().Foreground(lipgloss.Color("240")) // dim gray
    faintText      = lipgloss.NewStyle().Foreground(lipgloss.Color("243")) // dimmed

    headerStyle    = lipgloss.NewStyle().
                      Background(lipgloss.Color("62")).
                      Foreground(lipgloss.Color("255")).
                      Bold(true)
)
```

**Sparkline rendering:**

```go
// renderSparkline converts a [10]int event frequency array into a 10-char string.
// Each char is a block element (█ █ ▆ ▄ ▂ ░ ░ ) mapped from frequency percentile.
func renderSparkline(freq [10]int) string {
    if isAllZero(freq) {
        return strings.Repeat(" ", 10)
    }
    maxVal := maxFreq(freq)
    blocks := []string{" ", "▁", "▂", "▃", "▄", "▅", "▆", "▇", "█"}
    var sb strings.Builder
    for _, f := range freq {
        idx := int(float64(f) / float64(maxVal) * 8.0)
        sb.WriteString(blocks[idx])
    }
    return sb.String()
}
```

**Drill-down view (ENTER on session):**

Reuses `output.FormatExplain()` by reformatting it for the TUI's terminal width. Uses lipgloss borders and scroll. The drill-down replaces the entire view (no split pane — full screen for session details).

```go
func (m watchModel) renderDrillDown() string {
    // Get session events from state
    // Call output.FormatExplain(sessionID, events, signals, trees)
    // Wrap in lipgloss border with title
    // Add scroll offset handling for long timelines
    return lipgloss.NewStyle().
        Border(lipgloss.RoundedBorder()).
        BorderForeground(lipgloss.Color("62")).
        Width(m.width - 4).
        Height(m.height - 2).
        Render(explainText)
}
```

**Error flash:** Errors (source poll failure, corrupt line) appear as a 3-second flash in the header:

```go
type errMsg struct {
    err error
}

// In Update:
case errMsg:
    m.errMsg = msg.err.Error()
    return m, tea.Batch(
        func() tea.Msg { return clearErrMsg{} },
        tea.Tick(3*time.Second, func(time.Time) tea.Msg { return clearErrMsg{} }),
    )
```

### `cmd/root.go` — Subcommand dispatch

```go
// After flag.Parse(), before any existing logic:
if len(flag.Args()) > 0 {
    switch flag.Args()[0] {
    case "watch":
        handleWatch()
        return
    case "report":
        handleReport(flag.Args()[1:])
        return
    }
}
```

---

## Test Requirements

### `analyze/watch_test.go`

**Test cases:**

1. **`TestDiffEvents_NewEvents`** — new events returned, seen events skipped
   - 10 events, 5 previously seen → 5 new events
   - Same event (SessionID + EventIndex match) = not new

2. **`TestDiffEvents_CompletedSession`** — detects completion
   - Session with no new events for 2 polls → completed

3. **`TestDetectPartialWaste_Loop`** — incremental loop detection
   - 8 events with 3 repeat tool calls → flagged at relaxed threshold (3)
   - Same 8 events with 2 repeats → not flagged (below 3)

4. **`TestDetectPartialWaste_ReRead`** — incremental re-read detection
   - 10 events, file read twice with 0 cache → flagged at relaxed threshold (2)
   - Same file read twice with cache → not flagged

5. **`TestDetectPartialWaste_NoFalsePositive`** — clean session
   - 15 events, varied tool calls, no repeats → no signals

6. **`TestUpdateState_Merge`** — state updates correctly
   - New events added to existing session
   - Sparkline updated
   - LastAction updated
   - Cost recalculated

7. **`TestUpdateState_Idle`** — idle detection
   - Session with 35s since last event → marked idle (◌)
   - Session with 10s since last event → still active (●)

### `output/watch_format_test.go`

**Test cases:**

1. **`TestRenderSparkline`** — sparkline rendering
   - All zeros → 10 spaces
   - Uniform values → all same block
   - Varied values → varied block heights
   - Single spike → one tall block, rest low

2. **`TestRenderSessionRow`** — session row formatting
   - Active session → green dot, correct fields
   - Idle session → yellow dot
   - Subagent session → indented, labeled

3. **`TestRenderAlertRow`** — alert row formatting
   - High severity → red, bold
   - Medium severity → yellow
   - Old alert (>5 min) → dimmed style

**Coverage target:** >=90% on new code.

---

## Approach

1. Implement `analyze/watch.go` — DiffEvents, UpdateState, DetectPartialWaste
2. Write unit tests for incremental detection logic
3. Implement `output/watch_format.go` — rendering helpers
4. Write rendering tests
5. Implement `cmd/watch.go` — BubbleTea model, update loop, view
6. Wire `watch` subcommand in `cmd/root.go`
7. Manual test with active session: start a Claude Code session, run `burnwatch watch`
8. Verify resize, idle detection, alert appearance, drill-down
9. Full test suite + lint

---

## Exit Criteria

- [ ] Pull latest main
- [ ] Create feature branch from main: `git checkout -b n7-watch`
- [ ] Lint passes (zero warnings) — `golangci-lint run`
- [ ] Build compiles cleanly — `go build -o burnwatch .`
- [ ] Tests pass with >=90% coverage on new code — `go test ./... -cover`
- [ ] Self-review: run `./scripts/review-check.sh`, then verify Phases 1-3 in `docs/code-review.md`
- [ ] Document learnings in `docs/learnings.md`
- [ ] Commit: `feat: live monitoring TUI with incremental waste detection`
- [ ] Push to branch `n7-watch`
- [ ] Open pull request
- [ ] Dispatch CodeReviewer subagent against the PR diff
- [ ] Update `docs/plans/progress.md` to reflect merge
- [ ] Merge to main
- [ ] Delete feature branch after merge

---

## Notes

- **Incremental thresholds are deliberately conservative.** A false positive on a live session is more annoying than missing a waste signal. The user can always `burnwatch --explain <id>` after the session completes for accurate detection.
- **Subagent detection in watch mode.** Subagent spawn events (ToolCall with name "Task" or "dispatch") create a new WatchSession with IsSubagent=true and ParentSessionID set to the parent. This is driven by the data — the source already tags events with these fields.
- **Session files that stop growing.** Claude Code JSONL files append until session ends. OpenCode SQLite has a `status` field. When a file stops growing for 2 consecutive polls, mark the session as complete and run full (non-partial) detection.
- **Watch is distinct from `tail -f`.** Polling is simpler than fsnotify and works across all harness types (JSONL files, SQLite DB). The 5-second interval is fast enough to catch loops as they happen.
- **BubbleTea's `tea.Batch` for concurrency.** Polling is a `tea.Cmd` that runs in a goroutine. The TEA runtime handles message delivery. No manual goroutine management.
- **Lipgloss is likely transitive via BubbleTea.** Don't add it to `go.mod` explicitly unless `go mod tidy` doesn't pull it in. Check after `go get bubbletea`.
