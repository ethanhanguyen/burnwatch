package output

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/ethanhanguyen/burnwatch/analyze"
	"github.com/ethanhanguyen/burnwatch/source"
)

type sessionData struct {
	cost    float64
	project string
	harness string
}

type sessionEntry struct {
	sessionID string
	data      *sessionData
}

type harnessStats struct {
	sessions         map[string]bool
	subagentSessions map[string]bool
}

type projInfo struct {
	name    string
	harness string
	count   int
	cost    float64
	median  float64
}

func FormatText(
	events []source.TokenEvent,
	baselines map[string]analyze.Baseline,
	signals []analyze.WasteSignal,
	recommendations []analyze.Recommendation,
	verbose bool,
) string {
	if len(events) == 0 {
		return "No data found.\n"
	}

	recBySignal := make(map[analyze.WasteSignal]analyze.Recommendation)
	for _, r := range recommendations {
		recBySignal[r.Signal] = r
	}

	var b strings.Builder

	writeSummary(&b, events, baselines)

	writeProjects(&b, baselines)

	b.WriteString("\n")

	if verbose {
		writeAllSessions(&b, events)
	}

	if len(signals) == 0 {
		b.WriteString("No waste signals detected.\n")
		return b.String()
	}

	b.WriteString("Waste signals:\n")
	for _, s := range signals {
		writeSignalBlock(&b, s, recBySignal[s])
	}

	var totalSavings float64
	for _, r := range recommendations {
		totalSavings += r.SavingsEst
	}
	fmt.Fprintf(&b, "\nSummary: %d waste signals found. Potential savings: $%.2f\n", len(signals), totalSavings)

	return b.String()
}

func writeAllSessions(b *strings.Builder, events []source.TokenEvent) {
	sessions := make(map[string]*sessionData)
	for _, e := range events {
		s, ok := sessions[e.SessionID]
		if !ok {
			s = &sessionData{project: e.Project, harness: e.Harness}
			sessions[e.SessionID] = s
		}
		s.cost += e.CostUSD
	}

	var sorted []sessionEntry
	for sid, sd := range sessions {
		sorted = append(sorted, sessionEntry{sid, sd})
	}
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].data.cost > sorted[j].data.cost
	})

	b.WriteString("All sessions:\n")
	for _, e := range sorted {
		fmt.Fprintf(b, "  %s (%s): $%.2f\n", truncate(e.sessionID, 20), e.data.project, e.data.cost)
	}
	b.WriteString("\n")
}

func writeSummary(b *strings.Builder, events []source.TokenEvent, baselines map[string]analyze.Baseline) {
	harness := make(map[string]*harnessStats)

	now := NowFunc()
	todayStart := now.Truncate(24 * time.Hour)
	weekStart := todayStart.AddDate(0, 0, -int(now.Weekday()))

	var todayCost float64
	var todaySessions int
	todaySeen := make(map[string]bool)
	var weekCost float64
	var weekSessions int
	weekSeen := make(map[string]bool)

	for _, e := range events {
		h, ok := harness[e.Harness]
		if !ok {
			h = &harnessStats{
				sessions:         make(map[string]bool),
				subagentSessions: make(map[string]bool),
			}
			harness[e.Harness] = h
		}
		if e.IsSubagent {
			h.subagentSessions[e.SessionID] = true
		} else {
			h.sessions[e.SessionID] = true
		}

		if !e.Timestamp.Before(todayStart) && !e.Timestamp.After(now) {
			todayCost += e.CostUSD
			if !todaySeen[e.SessionID] {
				todaySeen[e.SessionID] = true
				todaySessions++
			}
		}
		if !e.Timestamp.Before(weekStart) && !e.Timestamp.After(now) {
			weekCost += e.CostUSD
			if !weekSeen[e.SessionID] {
				weekSeen[e.SessionID] = true
				weekSessions++
			}
		}
	}

	names := make([]string, 0, len(harness))
	for n := range harness {
		names = append(names, n)
	}
	sort.Strings(names)

	var parts []string
	for _, name := range names {
		hs := harness[name]
		total := len(hs.sessions) + len(hs.subagentSessions)
		label := harnessLabel(name)
		if len(hs.subagentSessions) > 0 {
			parts = append(parts, fmt.Sprintf("%s: %d sessions, %d subagent sessions", label, len(hs.sessions), len(hs.subagentSessions)))
		} else {
			parts = append(parts, fmt.Sprintf("%s: %d sessions", label, total))
		}
	}
	b.WriteString(strings.Join(parts, " | "))
	b.WriteString("\n")

	fmt.Fprintf(b, "Today: $%.2f (%d sessions) | This week: $%.2f (%d sessions)\n", todayCost, todaySessions, weekCost, weekSessions)
}

func harnessLabel(name string) string {
	switch name {
	case "opencode":
		return "OpenCode"
	case "claude-code":
		return "Claude Code"
	default:
		return name
	}
}

func writeProjects(b *strings.Builder, baselines map[string]analyze.Baseline) {
	var projects []projInfo
	for k, bl := range baselines {
		if k == "*" {
			continue
		}
		var totalCost float64
		for _, c := range bl.SessionCosts {
			totalCost += c
		}
		projects = append(projects, projInfo{
			name:    bl.Project,
			harness: bl.Harness,
			count:   bl.SessionCount,
			cost:    totalCost,
			median:  medianFromSorted(bl.SessionCosts),
		})
	}
	sort.Slice(projects, func(i, j int) bool {
		return projects[i].cost > projects[j].cost
	})

	if len(projects) == 0 {
		return
	}

	b.WriteString("\n")
	for _, p := range projects {
		fmt.Fprintf(b, "Project %s: $%.2f (median $%.2f/session)\n", p.name, p.cost, p.median)
	}
}

func medianFromSorted(sorted []float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	mid := len(sorted) / 2
	if len(sorted)%2 == 0 {
		return (sorted[mid-1] + sorted[mid]) / 2
	}
	return sorted[mid]
}

func writeSignalBlock(b *strings.Builder, s analyze.WasteSignal, rec analyze.Recommendation) {
	sev := strings.ToUpper(s.Severity)
	if s.SessionID != "" {
		fmt.Fprintf(b, "  %s %s (%s): $%.2f", sev, truncate(s.SessionID, 20), s.Project, s.SessionCost)
	} else {
		fmt.Fprintf(b, "  %s Project %s", sev, s.Project)
	}

	switch s.Reason {
	case "cost_outlier":
		fmt.Fprintf(b, " — %.1fx project baseline\n", s.Metric/s.Threshold)
	case "subagent_overhead":
		fmt.Fprintf(b, " — %.1f%% subagent overhead ($%.2f / $%.2f)\n", s.Metric, s.Metric*s.SessionCost/100.0, s.SessionCost)
	case "low_signal":
		fmt.Fprintf(b, " — output/input ratio %.4f (P10 = %.4f)\n", s.Metric, s.Threshold)
	case "cache_underutilized":
		fmt.Fprintf(b, " — cache hit rate %.1f%% (P10 = %.1f%%)\n", s.Metric*100, s.Threshold*100)
	case "session_churn":
		fmt.Fprintf(b, " — %.0f sessions below mean ratio\n", s.Metric)
	default:
		fmt.Fprintf(b, " — %s\n", s.Detail)
	}

	if rec.Action != "" {
		fmt.Fprintf(b, "    → %s. Potential savings: $%.2f\n", rec.Action, rec.SavingsEst)
	}
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}

func CollectEvents(sources []source.Source) []source.TokenEvent {
	var events []source.TokenEvent
	for _, src := range sources {
		evCh, errCh := src.Events()

		var done bool
		for !done {
			select {
			case e, ok := <-evCh:
				if !ok {
					done = true
				} else {
					events = append(events, e)
				}
			case _, ok := <-errCh:
				if !ok {
					errCh = nil
				}
			}
		}
	}
	return events
}

func RunPipeline(events []source.TokenEvent) (
	baselines map[string]analyze.Baseline,
	signals []analyze.WasteSignal,
	recommendations []analyze.Recommendation,
) {
	baselines = analyze.ComputeBaselines(events)
	signals = analyze.DetectWaste(events, baselines)
	recommendations = analyze.GenerateRecommendations(signals, baselines)
	_ = analyze.BuildSubagentTree(events)
	return
}
