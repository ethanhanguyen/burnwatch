# N6: Static HTML Report — `burnwatch report --open`

> **Workflow:** Follow `docs/PR-template.md`. Review `AGENTS.md` behavioral guidelines before implementing.
> **UX spec:** `docs/plans/v3-ux-plan.md` Phase II

## Objective

Add `burnwatch report --open` that generates a single self-contained HTML file with visualizations (Chart.js) for sharing and visual exploration. No server, no build step, no npm.

## Success Criteria

- [ ] `burnwatch report --days 30 --output report.html` writes valid, complete HTML file
- [ ] `burnwatch report --open` generates to a temp file and launches default browser
- [ ] HTML renders correctly in Chrome/Firefox/Safari (all charts loading from CDN)
- [ ] Six visualizations present with correct data:
  - Cost-over-time line chart (daily + 7-day moving average)
  - Waste-by-type stacked bar chart (per project, segmented by signal reason)
  - Top wasted files horizontal bar chart (files re-read most without cache)
  - Subagent cost treemap
  - Model cost donut chart
  - Interactive waste signal table (sortable, expandable rows)
- [ ] Signal table rows expand to show inline session drill-down (reuses `--explain` annotation data)
- [ ] All report data embedded as `const REPORT = {...}` in a single `<script>` tag
- [ ] Chart.js loaded from CDN with graceful degradation (no charts = table still works)
- [ ] Report footer shows generation timestamp and burnwatch version
- [ ] Zero-waste report renders without errors (all charts show empty/zero states)
- [ ] All existing tests pass unchanged

## Dependencies

- **Must merge first:** N5 (needs `--explain`'s annotation computation helpers for table drill-down)
- **External dependencies:** Chart.js 4.x (loaded from `cdn.jsdelivr.net`, no npm install)
- **Can be parallel with:** N4 (reuses output formatting, doesn't touch analysis)
- **Breaking changes / Migrations needed:** None

---

## Pre-flight

- [ ] Pull latest main: `git fetch origin && git checkout main && git pull origin main`
- [ ] Create feature branch: `git checkout -b n6-report`
- [ ] Verify build environment works on clean main

## Files

| File | Purpose | Notes |
|------|---------|-------|
| `output/report.go` | `FormatReport()` — HTML template + data marshaling | New, ~400 lines |
| `output/report_test.go` | Validate HTML structure, embedded data, chart config | New, ~200 lines |
| `cmd/root.go` | Add `report` flag handling + `--open` | Modify, ~40 lines |

---

## Implementation

### `output/report.go` — HTML generation

**Function signature:**

```go
// FormatReport generates a self-contained HTML report page.
// version: burnwatch version string (for footer)
// generated: timestamp of report generation
func FormatReport(
    events []source.TokenEvent,
    baselines map[string]analyze.Baseline,
    signals []analyze.WasteSignal,
    recommendations []analyze.Recommendation,
    trees []analyze.SubagentTree,
    version string,
    generated time.Time,
) string
```

**Output structure:**

```
<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>burnwatch — <date></title>
  <script src="https://cdn.jsdelivr.net/npm/chart.js@4"></script>
  <style>/* inline CSS for dark theme card layout */</style>
</head>
<body>
  <header>
    <h1>burnwatch</h1>
    <div class="summary-cards"><!-- project count, total cost, signal count --></div>
  </header>

  <section id="cost-over-time">
    <h2>Cost Over Time</h2>
    <canvas id="costChart"></canvas>
  </section>

  <section id="waste-by-type">
    <h2>Waste by Type</h2>
    <canvas id="wasteTypeChart"></canvas>
  </section>

  <section id="top-files">
    <h2>Top Wasted Files</h2>
    <canvas id="topFilesChart"></canvas>
  </section>

  <section id="subagent-tree">
    <h2>Subagent Cost Tree</h2>
    <div id="treemap"></div>
  </section>

  <section id="model-breakdown">
    <h2>Model Cost Breakdown</h2>
    <canvas id="modelChart"></canvas>
  </section>

  <section id="signal-table">
    <h2>Waste Signals</h2>
    <table id="signals"><!-- populated by JS --></table>
  </section>

  <footer>burnwatch vX.Y.Z · generated YYYY-MM-DDTHH:MM:SSZ</footer>

  <script>
    const REPORT = {
      version: "...",
      generated: "...",
      summary: { totalCost, totalSignals, projectCount, ... },
      costOverTime: [{ date, cost, movingAvg }, ...],
      wasteByType: [{ project, loop: 0, reread: 0, overlap: 0, ... }, ...],
      topFiles: [{ path, readCount, sessions }, ...],
      subagentTree: { name, cost, children: [...] },
      modelBreakdown: [{ model, cost, percentage }, ...],
      signals: [{ sessionID, project, severity, reason, detail, cost, ... }, ...],
      // Per-signal drill-down data (from explain annotation logic)
      signalTimelines: { "ses_abc123": [{ eventIndex, toolCalls, fileOps, annotations }, ...] },
    };
  </script>
  <script>
    // Chart.js initialization
    // Table rendering + row expansion
    // Treemap rendering (canvas-based, no library)
  </script>
</body>
</html>
```

**Design constraints:**

1. **Dark theme.** Burnwatch is a terminal tool. The report should feel like a polished extension of that. Dark background (`#1a1a2e`), light text, accent colors for severity (red=high, amber=medium, blue=low).

2. **Single file, zero dependencies beyond Chart.js CDN.** All CSS inline. All JS inline after the data block. No webpack, no npm, no build. Chart.js is the only `<script src>` tag.

3. **Treemap implemented in vanilla JS Canvas.** No d3, no treemap library. The algorithm is 50 lines: squarified treemap layout on a `<canvas>`. Each rectangle = subagent cost, color = parent session.

4. **Graceful degradation.** If Chart.js CDN fails (offline, blocked), charts show `Chart library unavailable` placeholder. The signal table still renders from the embedded JSON.

5. **Drill-down reuses `--explain` logic.** The `signalTimelines` embedded data uses the same `computeLoopAnnotations`, `computeReReadAnnotations`, and `computeSubagentAnnotations` helpers from `output/explain.go`. The table's expandable rows render a mini-timeline with highlighted waste events.

6. **Responsive but mobile-second.** Designed for desktop (1200px+). Charts use `responsive: true` but max-width 1200px centered.

**Data computation helpers (package-private, in `output/report_data.go`):**

```go
// reportData bundles all pre-computed chart data.
type reportData struct {
    Summary         reportSummary
    CostOverTime    []costOverTimePoint
    WasteByType     []wasteByProject
    TopFiles        []topFile
    SubagentTree    reportTreeNode
    ModelBreakdown  []modelBreakdown
    Signals         []reportSignal
    SignalTimelines map[string][]reportTimelineEvent
}

// computeReportData pre-computes all chart-worthy data from raw events/signals.
// Called once by FormatReport to avoid redundant computation.
func computeReportData(
    events []source.TokenEvent,
    baselines map[string]analyze.Baseline,
    signals []analyze.WasteSignal,
    recommendations []analyze.Recommendation,
    trees []analyze.SubagentTree,
) reportData
```

**`computeReportData` sub-computations:**

1. **CostOverTime** — group events by day, sum cost. Compute 7-day trailing moving average. Sort by date ascending.

2. **WasteByType** — for each project, count signals by reason (cost_outlier, tool_call_loop, file_reread, subagent_overhead, subagent_overlap, session_restart, fragmentation_index, input_overconsumption, output_explosion, low_token_efficiency, low_signal, cache_underutilized). Sum cost per reason per project.

3. **TopFiles** — from H11 signals (file_reread), extract file paths and read counts from Detail strings. Aggregate across sessions. Sort by read count descending. Top 15 only.

4. **SubagentTree** — convert `[]SubagentTree` to a nested `reportTreeNode{Name, Cost, Children}`. The root is the parent session. Children are subagents. Recurse.

5. **ModelBreakdown** — group events by model, sum cost. Sort by cost descending. Compute percentage of total.

6. **SignalTimelines** — for each session with a signal, collect its events, compute annotations (reusing `computeLoopAnnotations` etc.), and build a timeline entry. This is what the table's expandable rows render.

**Constraints:**
- Sort all chart data deterministically (by name as tiebreaker after primary sort key).
- Empty/zero data: return empty slices, not nil. Charts render empty states gracefully.
- TopFiles extraction: the Detail string format is `"<path> read <N> times, 0 cache hits"`. Parse path and count.
- SubagentTree: the root node's cost is parent session cost minus all subagent costs. Subagent costs are from Tree.CostUSD.

### `cmd/root.go` — Wiring

```go
// Report flags
reportOutput := flag.String("output", "", "Report output file path (default: burnwatch-report-<date>.html)")
reportOpen := flag.Bool("open", false, "Open report in browser after generation")
reportDays := flag.Int("days", 30, "Days of data for report")
reportJSON := flag.Bool("report-json", false, "Output report JSON data (no HTML)")

// After flag.Parse(), add a report block (before the main pipeline):
reportFlag := isReportMode(flag.Args()) // check if "report" subcommand or --report flag

if reportFlag {
    // Collect events
    // Filter by --days
    // Run full pipeline (baselines, signals, trees, recommendations)
    // If --report-json: output JSON only
    // Else: call FormatReport, write to file, optionally --open
    // Exit
}
```

Use a `report` subcommand approach — check `flag.Args()` for `"report"` as the first positional arg. This is cleaner than adding a `--report` boolean since report-specific flags (--output, --open) only apply in report mode.

```go
if len(flag.Args()) > 0 && flag.Args()[0] == "report" {
    handleReport(flag.Args()[1:])  // remaining args for report-specific flags
    return
}
```

**`handleReport` function:**

```go
func handleReport(args []string) {
    // 1. Parse report-specific flags from args
    // 2. Collect events + filter by days
    // 3. Run pipeline
    // 4. FormatReport
    // 5. Write to file (default: burnwatch-report-YYYY-MM-DD.html)
    // 6. If --open: exec open/xdg-open/start on the file
}
```

**`--open` implementation:**

```go
func openBrowser(path string) error {
    var cmd string
    var args []string
    switch runtime.GOOS {
    case "darwin":
        cmd = "open"
    case "linux":
        cmd = "xdg-open"
    case "windows":
        cmd = "cmd"
        args = []string{"/c", "start"}
    default:
        return fmt.Errorf("unsupported platform")
    }
    args = append(args, path)
    return exec.Command(cmd, args...).Start()
}
```

---

## Test Requirements

### `output/report_test.go`

**Test cases:**

1. **`TestFormatReport_Structure`** — verify HTML skeleton
   - Output begins with `<!DOCTYPE html>`
   - Contains `<script src="...chart.js...">`
   - Contains `const REPORT = {`
   - Contains `<footer>`

2. **`TestFormatReport_DataEmbedded`** — verify data integrity
   - Parse the embedded JSON from the `<script>` tag
   - Assert summary.totalCost matches event cost sum
   - Assert summary.totalSignals matches signal count
   - Assert costOverTime has correct number of days
   - Assert wasteByType covers all projects

3. **`TestFormatReport_EmptyData`** — zero events
   - Returns valid HTML without crashing
   - costOverTime is empty array `[]`
   - signals is empty array `[]`
   - No chart init errors

4. **`TestFormatReport_SignalTimeline`** — drill-down data
   - Signal with tool_call_loop reason has timeline entries with loop annotations
   - Signal with file_reread reason has timeline entries with re-read annotations

5. **`TestFormatReport_SubagentTree`** — tree structure
   - Root node exists
   - Children have correct cost
   - Recursive structure preserved

6. **`TestFormatReport_TopFiles`** — file extraction
   - Detail string parsed correctly
   - Files sorted by read count descending

7. **`TestFormatReport_OutputPath`** — file writing
   - Writes file to specified path
   - File is readable and non-empty
   - Default filename includes current date

**Coverage target:** >=90% on new code.

### Integration test

```go
func TestReportIntegration(t *testing.T) {
    // Load scenario data with multiple waste types
    events := loadScenarioJSONL(t, "explain_mixed.jsonl")
    // Run pipeline
    // Generate report
    // Parse HTML, extract REPORT JSON
    // Verify chart data structures are populated
}
```

---

## Approach

1. Implement `computeReportData` and test each sub-computation independently
2. Write the HTML template as a Go constant with `%s`/`%v` placeholders — use `fmt.Sprintf` for injection
3. Embed the JSON data block using `json.Marshal` with compact output
4. Write Chart.js initialization JS for each chart
5. Implement treemap in vanilla JS Canvas
6. Implement signal table with expandable rows
7. Wire `report` subcommand in `cmd/root.go`
8. Test with real data: `go run . report --days 30 --open`
9. Verify in Chrome, Firefox, Safari
10. Full test suite + lint

---

## Exit Criteria

- [ ] Pull latest main
- [ ] Create feature branch from main: `git checkout -b n6-report`
- [ ] Lint passes (zero warnings) — `golangci-lint run`
- [ ] Build compiles cleanly — `go build -o burnwatch .`
- [ ] Tests pass with >=90% coverage on new code — `go test ./... -cover`
- [ ] Self-review: run `./scripts/review-check.sh`, then verify Phases 1-3 in `docs/code-review.md`
- [ ] Document learnings in `docs/learnings.md`
- [ ] Commit: `feat: static HTML report with Chart.js visualizations`
- [ ] Push to branch `n6-report`
- [ ] Open pull request
- [ ] Dispatch CodeReviewer subagent against the PR diff
- [ ] Update `docs/plans/progress.md` to reflect merge
- [ ] Merge to main
- [ ] Delete feature branch after merge

---

## Notes

- **Chart.js 4.x is the only external dependency** and it's loaded from CDN at runtime, not at build time. The Go binary has zero new dependencies.
- **Treemap algorithm**: squarified treemap (Bruls, Huizing, van Wijk 2000). Implement in ~50 lines of JS. The algorithm lays out rectangles so that aspect ratios are close to square. Sorted by cost descending (largest first).
- **JSON embedded in HTML** — use `json.Marshal` with no indentation for the embedded block to minimize size. The REPORT object should be compact single-line JSON.
- **Don't inline Chart.js.** The library is ~200KB minified. Inlining would bloat every report. CDN with `integrity` hash is the right tradeoff.
- **Signal table drill-down** reuses `computeLoopAnnotations` etc. from `output/explain.go`. Make these functions package-public (capitalize) so they can be called from `output/report_data.go`. No code duplication.
- **iOS/Android viewing**: The report is a standard HTML file. It works in mobile browsers without modification. The responsive design handles smaller screens.
- **The `report` subcommand is not `--report`.** Use `os.Args` checking for positional `"report"` to keep the flag namespace clean. `burnwatch report --days 30 --open` reads naturally.
