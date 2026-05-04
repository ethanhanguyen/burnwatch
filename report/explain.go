package report

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ethanhanguyen/burnwatch/analyze"
	"github.com/ethanhanguyen/burnwatch/source"
)

type Annotation struct {
	EventIndex int
	Text       string
}

func FormatExplain(sessionID string, events []source.TokenEvent, signals []analyze.WasteSignal, trees []analyze.SubagentTree) string {
	if len(events) == 0 {
		return fmt.Sprintf("No events found for session %q.\n", sessionID)
	}

	sorted := sortedEvents(events)

	var b strings.Builder

	writeExplainHeader(&b, sessionID, sorted)
	b.WriteString("\n")

	if len(signals) > 0 {
		writeWasteSummary(&b, signals)
		b.WriteString("\n")
	}

	annotations := ComputeAnnotations(sorted, signals, trees)
	writeTimeline(&b, sorted, annotations)

	writeSubagentBreakdown(&b, trees)

	writeReReadBreakdown(&b, signals, sorted)

	return b.String()
}

func sortedEvents(events []source.TokenEvent) []source.TokenEvent {
	cp := make([]source.TokenEvent, len(events))
	copy(cp, events)
	sort.Slice(cp, func(i, j int) bool {
		return cp[i].EventIndex < cp[j].EventIndex
	})
	return cp
}

func writeExplainHeader(b *strings.Builder, sessionID string, events []source.TokenEvent) {
	first := events[0]
	last := events[len(events)-1]

	var totalCost float64
	var costApprox, costUnknown bool
	var model string
	var toolCount, fileCount, subagentCount int
	hasSubagent := false

	for _, e := range events {
		totalCost += e.CostUSD
		if e.CostApproximate {
			costApprox = true
		}
		if e.CostUnknown {
			costUnknown = true
		}
		if e.Model != "" {
			model = e.Model
		}
		toolCount += len(e.ToolCalls)
		fileCount += len(e.FileOps)
		if e.IsSubagent {
			subagentCount++
		}
		if e.IsSubagent || e.ParentSessionID != "" {
			hasSubagent = true
		}
	}

	fmt.Fprintf(b, "Session:  %s\n", sessionID)
	fmt.Fprintf(b, "Project:  %s\n", first.Project)
	fmt.Fprintf(b, "Harness:  %s\n", first.Harness)
	fmt.Fprintf(b, "Duration: %s\n", formatDuration(first.Timestamp, last.Timestamp))
	fmt.Fprintf(b, "Cost:     %s\n", formatCost(totalCost, costApprox, costUnknown))
	fmt.Fprintf(b, "Model:    %s\n", orUnknown(model))
	fmt.Fprintf(b, "Events:   %d", len(events))
	fmt.Fprintf(b, " (%d tool calls, %d file ops", toolCount, fileCount)
	if hasSubagent {
		fmt.Fprintf(b, ", %d subagents", subagentCount)
	}
	b.WriteString(")\n")
}

func formatDuration(start, end time.Time) string {
	d := end.Sub(start)
	if d < 0 {
		d = 0
	}
	if d < time.Minute {
		secs := int(d.Seconds() + 0.5)
		return fmt.Sprintf("%ds", secs)
	}
	if d < time.Hour {
		mins := int(d.Minutes() + 0.5)
		return fmt.Sprintf("%dm", mins)
	}
	hours := int(d.Hours())
	mins := int(d.Minutes()) % 60
	if mins == 0 {
		return fmt.Sprintf("%dh", hours)
	}
	return fmt.Sprintf("%dh %dm", hours, mins)
}

func formatCost(cost float64, approx, unknown bool) string {
	if unknown {
		return "$?"
	}
	prefix := ""
	if approx {
		prefix = "≈ "
	}
	return fmt.Sprintf("%s$%.2f", prefix, cost)
}

func orUnknown(s string) string {
	if s == "" {
		return "unknown"
	}
	return s
}

func writeWasteSummary(b *strings.Builder, signals []analyze.WasteSignal) {
	sorted := make([]analyze.WasteSignal, len(signals))
	copy(sorted, signals)
	sort.Slice(sorted, func(i, j int) bool {
		si, sj := sorted[i], sorted[j]
		ri := severityRank(si.Severity)
		rj := severityRank(sj.Severity)
		if ri != rj {
			return ri < rj
		}
		return si.Reason < sj.Reason
	})

	b.WriteString("─── Waste Signals ───\n\n")

	for _, s := range sorted {
		fmt.Fprintf(b, "  %-7s %-20s %s\n",
			strings.ToUpper(s.Severity), s.Reason, s.Detail)
	}
}

func severityRank(s string) int {
	switch s {
	case "high":
		return 0
	case "medium":
		return 1
	case "low":
		return 2
	default:
		return 3
	}
}

func writeTimeline(b *strings.Builder, events []source.TokenEvent, annotations []Annotation) {
	b.WriteString("─── Event Timeline ───\n\n")

	annMap := make(map[int][]string)
	for _, a := range annotations {
		annMap[a.EventIndex] = append(annMap[a.EventIndex], a.Text)
	}

	const maxLines = 500
	truncate := len(events) > maxLines
	var shown []source.TokenEvent
	if truncate {
		wasteIndices := make(map[int]bool)
		for _, a := range annotations {
			for d := -5; d <= 5; d++ {
				wasteIndices[a.EventIndex+d] = true
			}
		}

		shown = append(shown, events[:50]...)
		var gapShown bool
		for i := 50; i < len(events)-20; i++ {
			if wasteIndices[events[i].EventIndex] {
				if !gapShown {
					shown = append(shown, source.TokenEvent{EventIndex: -1})
					gapShown = true
				}
				shown = append(shown, events[i])
			}
		}
		if !gapShown {
			shown = append(shown, source.TokenEvent{EventIndex: -1})
		}
		shown = append(shown, events[len(events)-20:]...)
	} else {
		shown = events
	}

	subagentSet := make(map[string]bool)
	for _, e := range events {
		if e.IsSubagent {
			subagentSet[e.SessionID] = true
		}
	}
	subagentFirst := make(map[string]bool)
	for _, e := range events {
		if e.IsSubagent && !subagentFirst[e.SessionID] {
			subagentFirst[e.SessionID] = true
		}
	}

	for _, ev := range shown {
		isSubagent := subagentSet[ev.SessionID]
		isFirst := isSubagent && subagentFirst[ev.SessionID]
		isGap := ev.EventIndex == -1

		if isGap {
			omitted := len(events) - 50 - 20
			fmt.Fprintf(b, "  ... (%d events omitted) ...\n", omitted)
			continue
		}

		indent := "  "
		if isSubagent {
			indent = "    "
		}

		anns := annMap[ev.EventIndex]
		annText := strings.Join(anns, " ")

		if len(ev.ToolCalls) > 0 {
			for _, tc := range ev.ToolCalls {
				if isFirst {
					isFirst = false
					fmt.Fprintf(b, "%s#%-4d▶ subagent:%-13s%s\n",
						indent[:2], ev.EventIndex, ev.AgentType, annText)
					continue
				}
				fmt.Fprintf(b, "%s#%-4d%-16s%s\n",
					indent, ev.EventIndex, tc.Name, formatAnn(truncateArg(tc.Arguments), annText))
			}
			continue
		}

		if len(ev.FileOps) > 0 {
			for _, fo := range ev.FileOps {
				fmt.Fprintf(b, "%s#%-4d%-8s%s%s\n",
					indent, ev.EventIndex, fo.Operation, fo.Path, formatAnn("", annText))
			}
			continue
		}

		fmt.Fprintf(b, "%s#%-4d—%s\n", indent, ev.EventIndex, annText)
	}
}

func truncateArg(args string) string {
	if len(args) <= 60 {
		return args
	}
	return args[:60] + "..."
}

func formatAnn(base string, ann string) string {
	if ann == "" {
		if base != "" {
			return "  " + base
		}
		return ""
	}
	if base != "" {
		base = "  " + base
	}
	return fmt.Sprintf("%-44s %s", base, ann)
}

func writeSubagentBreakdown(b *strings.Builder, trees []analyze.SubagentTree) {
	if len(trees) == 0 {
		return
	}
	b.WriteString("\n─── Subagent Cost Breakdown ───\n\n")
	for _, t := range trees {
		fmt.Fprintf(b, "  Root: %s\n", t.SessionID)
		fmt.Fprintf(b, "  Total Cost: %.2f  Subagent Cost: %.2f (%.1f%%)\n",
			t.TotalCost, t.SubagentCost, t.OverheadPct)
		writeSubagentNodes(b, t.Subagents, "    ")
		b.WriteString("\n")
	}
}

func writeSubagentNodes(b *strings.Builder, nodes []analyze.SubagentNode, indent string) {
	const maxShow = 10
	shown := nodes
	remaining := 0
	if len(nodes) > maxShow {
		shown = nodes[:maxShow]
		remaining = len(nodes) - maxShow
	}
	for _, n := range shown {
		fmt.Fprintf(b, "%s• %s (%s) — $%.2f\n",
			indent, n.SessionID, n.AgentType, n.Cost)
		if len(n.Children) > 0 {
			writeSubagentNodes(b, n.Children, indent+"  ")
		}
	}
	if remaining > 0 {
		fmt.Fprintf(b, "%s... and %d more\n", indent, remaining)
	}
}

func writeReReadBreakdown(b *strings.Builder, signals []analyze.WasteSignal, events []source.TokenEvent) {
	var reReadSignals []analyze.WasteSignal
	for _, s := range signals {
		if s.Reason == "file_reread" {
			reReadSignals = append(reReadSignals, s)
		}
	}
	if len(reReadSignals) == 0 {
		return
	}

	b.WriteString("\n─── File Re-Read Breakdown ───\n\n")
	for _, s := range reReadSignals {
		path, readCount := ParseRereadDetail(s.Detail)
		if path == "" {
			continue
		}

		var indices []int
		for _, ev := range events {
			for _, fo := range ev.FileOps {
				if fo.Operation == "read" && fo.Path == path {
					indices = append(indices, ev.EventIndex)
				}
			}
		}

		fmt.Fprintf(b, "  %s (%d reads, 0 cache hits)\n", path, readCount)
		const maxShow = 10
		shown := indices
		remaining := 0
		if len(indices) > maxShow {
			shown = indices[:maxShow]
			remaining = len(indices) - maxShow
		}
		var idxStrs []string
		for _, idx := range shown {
			idxStrs = append(idxStrs, strconv.Itoa(idx))
		}
		fmt.Fprintf(b, "    at events: #%s", strings.Join(idxStrs, ", #"))
		if remaining > 0 {
			fmt.Fprintf(b, ", ... and %d more", remaining)
		}
		b.WriteString("\n")
	}
}

func ComputeAnnotations(events []source.TokenEvent, signals []analyze.WasteSignal, trees []analyze.SubagentTree) []Annotation {
	var anns []Annotation
	anns = append(anns, ComputeLoopAnnotations(events, signals)...)
	anns = append(anns, ComputeReReadAnnotations(events, signals)...)
	anns = append(anns, ComputeSubagentAnnotations(events, trees)...)
	return anns
}

func ComputeLoopAnnotations(events []source.TokenEvent, signals []analyze.WasteSignal) []Annotation {
	sorted := sortedEvents(events)

	type idxToolCall struct {
		name      string
		arguments string
		eventIdx  int
	}
	var flat []idxToolCall
	for _, ev := range sorted {
		for _, tc := range ev.ToolCalls {
			flat = append(flat, idxToolCall{
				name:      tc.Name,
				arguments: tc.Arguments,
				eventIdx:  ev.EventIndex,
			})
		}
	}

	var annotations []Annotation
	for _, s := range signals {
		if s.Reason != "tool_call_loop" {
			continue
		}
		toolName, filePath := ParseLoopDetail(s.Detail)
		if toolName == "" {
			continue
		}

		i := 0
		for i < len(flat) {
			if !toolsMatch(flat[i].name, toolName, flat[i].arguments, filePath) {
				i++
				continue
			}
			j := i
			for j < len(flat) && toolsMatch(flat[j].name, toolName, flat[j].arguments, filePath) {
				j++
			}
			run := j - i
			if run > 1 {
				for k := 0; k < run; k++ {
					annotations = append(annotations, Annotation{
						EventIndex: flat[i+k].eventIdx,
						Text:       fmt.Sprintf("← [LOOP REPEAT %d/%d]", k+1, run),
					})
				}
			}
			i = j
		}
	}
	return annotations
}

func toolsMatch(tcName, sigName, tcArgs, sigPath string) bool {
	if !strings.EqualFold(tcName, sigName) {
		return false
	}
	if sigPath == "" {
		return true
	}
	return strings.Contains(tcArgs, sigPath)
}

func ComputeReReadAnnotations(events []source.TokenEvent, signals []analyze.WasteSignal) []Annotation {
	var annotations []Annotation
	for _, s := range signals {
		if s.Reason != "file_reread" {
			continue
		}
		path, _ := ParseRereadDetail(s.Detail)
		if path == "" {
			continue
		}

		var matches []int
		for _, ev := range events {
			for _, fo := range ev.FileOps {
				if fo.Operation == "read" && fo.Path == path {
					matches = append(matches, ev.EventIndex)
				}
			}
		}

		for i, idx := range matches {
			annotations = append(annotations, Annotation{
				EventIndex: idx,
				Text:       fmt.Sprintf("← [RE-READ %d/%d]", i+1, len(matches)),
			})
		}
	}
	return annotations
}

func ComputeSubagentAnnotations(events []source.TokenEvent, trees []analyze.SubagentTree) []Annotation {
	subagentMap := make(map[string]string)
	var collect func(nodes []analyze.SubagentNode)
	collect = func(nodes []analyze.SubagentNode) {
		for _, n := range nodes {
			subagentMap[n.SessionID] = n.AgentType
			collect(n.Children)
		}
	}
	for _, t := range trees {
		collect(t.Subagents)
	}

	firstBySession := make(map[string]bool)
	var annotations []Annotation
	for _, ev := range events {
		if !ev.IsSubagent {
			continue
		}
		if firstBySession[ev.SessionID] {
			continue
		}
		firstBySession[ev.SessionID] = true

		at := subagentMap[ev.SessionID]
		if at == "" {
			at = ev.AgentType
		}
		if at != "" {
			annotations = append(annotations, Annotation{
				EventIndex: ev.EventIndex,
				Text:       "[SUBAGENT START]",
			})
		}
	}
	return annotations
}

func ParseLoopDetail(detail string) (toolName string, filePath string) {
	calledIdx := strings.Index(detail, " called ")
	if calledIdx < 0 {
		return "", ""
	}
	prefix := detail[:calledIdx]

	parenStart := strings.Index(prefix, "(")
	if parenStart < 0 {
		return strings.TrimSpace(prefix), ""
	}

	toolName = strings.TrimSpace(prefix[:parenStart])

	rest := prefix[parenStart+1:]
	q1 := strings.Index(rest, "\"")
	if q1 < 0 {
		return toolName, ""
	}
	rest = rest[q1+1:]
	q2 := strings.Index(rest, "\"")
	if q2 < 0 {
		return toolName, ""
	}
	filePath = rest[:q2]
	return toolName, filePath
}

func ParseRereadDetail(detail string) (path string, readCount int) {
	idx := strings.Index(detail, " read ")
	if idx < 0 {
		return "", 0
	}
	path = detail[:idx]
	rest := detail[idx+len(" read "):]
	timesIdx := strings.Index(rest, " times")
	if timesIdx < 0 {
		return "", 0
	}
	n, err := strconv.Atoi(rest[:timesIdx])
	if err != nil {
		return "", 0
	}
	return path, n
}
