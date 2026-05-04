# N8: Report Bug Fixes — 5 Corrections for HTML Report Accuracy + UX

> **Workflow:** Follow `docs/PR-template.md`. Review `AGENTS.md` behavioral guidelines before implementing.

## Objective

Fix 5 bugs in the static HTML report (N6) that produce incorrect data displays and suboptimal UX: a `tracedCost` double-counting bug, invisible chart data points, project-name-as-path in charts and tables, a missing top-files filter, and duplicate/redundant signal reason text.

## Success Criteria

- [ ] **C1 — TracedCost ≤ TotalCost:** In the hero section, "$X traced to identifiable waste" is never greater than "Total Burned $Y". Verified by spot-check: for any report, `summary.tracedCost ≤ summary.totalCost`.
- [ ] **C2 — Visible chart points:** Cost Over Time line chart shows visible data dots (`pointRadius: 3`). Tooltips appear on hover showing date + dollar amount.
- [ ] **C3 — Short project names in Waste bar chart:** X-axis labels show project basenames (e.g. `burnwatch`) instead of full paths (e.g. `Users/hoang/burnwatch`).
- [ ] **C4 — Top-files filter + scroll:** Leaderboard section has a selector (Top 3 / Top 10 / All). Container is scrollable (`max-height` + `overflow-y: auto`). Shows at most 15 files.
- [ ] **C5a — Short project names in Signals Ledger:** Project column shows basenames, not paths.
- [ ] **C5b — Signal detail shows description:** The `<small>` text under each reason shows the human-readable `detail` (e.g. "Session cost $2.50 exceeds baseline μ+2σ = $1.80") instead of `s.reason + ' · ' + s.metric` (e.g. "cost_outlier · 5.0").
- [ ] All existing tests pass. New unit test for `projectLabel()` helper.

## Dependencies

- **Must merge first:** None
- **External dependencies:** None
- **Can be parallel with:** None
- **Breaking changes / Migrations needed:** None

---

## Pre-flight

- [ ] Pull latest main: `git fetch origin && git checkout main && git pull origin main`
- [ ] Create feature branch: `git checkout -b n8-report-fixes`
- [ ] Verify build environment works on clean main

## Files

| File | Purpose | Notes |
|------|---------|-------|
| `report/report_data.go` | Fix tracedCost dedup (#1), add `projectLabel()` helper (#3,#5a), use helper in `computeWasteByType` and `computeReportSignals` | Modify |
| `report/report.go` | Chart `pointRadius: 3` (#2), top-N selector HTML/CSS/JS (#4), signal detail uses `s.detail` (#5b) | Modify |
| `report/report_data_test.go` | Unit test for `projectLabel()` | Modify (add test) |

---

## Implementation

### #1: Deduplicate `tracedCost` by session ID

**Bug:** A session with 4 signals has its `SessionCost` added 4× to `tracedCost`, exceeding `totalCost`.

**Fix in `computeReportSummary` (`report_data.go`):**

```go
// BEFORE (line 125-150):
var tracedCost float64
for _, s := range signals {
    tracedCost += s.SessionCost
    // ...
}

// AFTER:
var tracedCost float64
tracedSessions := make(map[string]bool)
for _, s := range signals {
    if !tracedSessions[s.SessionID] {
        tracedSessions[s.SessionID] = true
        tracedCost += s.SessionCost
    }
    // ...
}
```

### #2: Make Cost Over Time chart points visible

**Problem:** `pointRadius: 0` in Chart.js config at `report.go:~1051` makes data points invisible. Sparse 7-day data produces no visible targets for hover/tooltips.

**Fix:** Change `pointRadius: 0` to `pointRadius: 3` in the `cFlow` dataset config.

### #3 + #5a: Add `projectLabel()` helper + use it everywhere

**Problem:** The `Project` field in events holds path-like strings (`Users/hoang/burnwatch`) from Claude Code's `projectNameToDisplay()` transform, or UUIDs from OpenCode. These appear raw in chart x-axes and table columns.

**Fix:** Add a helper that extracts the last path segment:

```go
func projectLabel(project string) string {
    if project == "" {
        return "(unknown)"
    }
    idx := strings.LastIndex(project, "/")
    if idx >= 0 {
        return project[idx+1:]
    }
    return project
}
```

Use this helper:
- `computeWasteByType`: wrap `name` when constructing `wasteByProject.Project` (line ~246)
- `computeReportSignals`: wrap `s.Project` when constructing `reportSignal.Project` (line ~369)

### #4: Top-N selector + scrollable leaderboard

**Backend (`report_data.go`):** Keep `const maxFiles = 15`. No change needed — pass all data to JS as before.

**Frontend (`report.go`) — CSS additions:**
```css
.file-filter { display: flex; gap: 6px; margin-bottom: var(--s3); }
.file-filter button {
  background: var(--surface-2); border: 1px solid var(--rule);
  color: var(--parchment-dim); font-family: 'Cinzel', serif;
  font-size: 10px; padding: 4px 12px; border-radius: var(--r-sm);
  cursor: pointer; letter-spacing: 0.1em; transition: all 0.15s;
}
.file-filter button.active, .file-filter button:hover { 
  background: var(--surface-3); border-color: var(--gold); color: var(--gold); 
}
.leaderboard-scroll { max-height: 380px; overflow-y: auto; }
```

**Frontend (`report.go`) — HTML additions in `renderWasteSection`:**
Insert after `<div class="panel-title">Most Re-Read Files`:
```html
<div class="file-filter">
  <button data-n="3">Top 3</button>
  <button data-n="10" class="active">Top 10</button>
  <button data-n="15">All</button>
</div>
<div class="leaderboard-scroll">
  <div class="leaderboard" id="leaderboard"></div>
</div>
```

**Frontend (`report.go`) — JS in `buildLeaderboard`:**
Wrap rendering logic with a filter. Listen for button clicks. Default to 10. Collect buttons, attach click handlers that re-render.

### #5b: Use signal `detail` instead of redundant `reason + metric`

**Bug in `buildTable` JS (`report.go:~1326-1328`):**
```js
// BEFORE: shows "cost_outlier · 5.0" which repeats the capitalized label
var detailText = s.reason + ' · ' + s.metric.toFixed(1);

// AFTER: show the human-readable explanation
var detailText = s.detail;
```

The `detail` field already contains the full description like `"Session cost $2.50 exceeds project baseline μ+2σ = $1.80"`. No backend change needed — we already store it.

---

## Test Requirements

1. **`report/report_data_test.go`**:
   - `TestProjectLabel` — table-driven:
     - `""` → `"(unknown)"`
     - `"burnwatch"` → `"burnwatch"`
     - `"Users/hoang/burnwatch"` → `"burnwatch"`
     - `"Users/hoang/multi/word/path"` → `"path"`
     - `"simple"` → `"simple"`

---

## Approach

1. Read `report/report_data.go` and `report/report.go` in full
2. Write `projectLabel` + test first
3. Fix #1 (tracedCost dedup) — minimal change, no test needed (behavioral fix)
4. Fix #2 (pointRadius) — one-line CSS/JS change
5. Fix #3 (projectLabel in computeWasteByType)
6. Fix #4 (top-N selector) — HTML + CSS + JS
7. Fix #5a (projectLabel in computeReportSignals)
8. Fix #5b (detail text in buildTable)
9. Run `go test ./... -cover`
10. Build + generate report to verify visually

---

## Exit Criteria

- [ ] Pull latest main
- [ ] Create feature branch from main: `git checkout -b n8-report-fixes`
- [ ] Lint passes (zero warnings) — see `AGENTS.md` for project commands
- [ ] Build compiles cleanly — see `AGENTS.md` for project commands
- [ ] Tests pass with required coverage — see `AGENTS.md` for project commands
- [ ] Self-review: run `./scripts/review-check.sh`, then verify Phases 1-3 in [docs/code-review.md](../code-review.md)
- [ ] Document learnings (gotchas, mistakes, patterns, hidden coupling) in `docs/learnings.md`
- [ ] Commit: `fix: report tracedCost dedup, chart points, project labels, top-N filter, signal details`
- [ ] Push to branch `n8-report-fixes`
- [ ] Open pull request with description
- [ ] Dispatch CodeReviewer subagent against the PR diff
- [ ] Update `docs/plans/progress.md` to reflect merge
- [ ] Merge to main
- [ ] Delete feature branch after merge

---

## Notes

- `projectLabel()` is a pure function — no side effects, easy to test
- The `detail` field on `reportSignal` already carries the full human-readable text from `WasteSignal.Detail` (set during detection in `waste.go`). No backend data change needed for #5b — just render it in JS.
- CSS changes are additive — no existing selectors are modified.
- Leaderboard filter state resets on window resize (Chart.js resize triggers full re-render already).
