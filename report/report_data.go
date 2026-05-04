package report

import (
	"sort"
	"time"

	"github.com/ethanhanguyen/burnwatch/analyze"
	"github.com/ethanhanguyen/burnwatch/source"
)

type reportData struct {
	Version         string                           `json:"version"`
	Generated       string                           `json:"generated"`
	Summary         reportSummary                    `json:"summary"`
	CostOverTime    []costOverTimePoint              `json:"costOverTime"`
	WasteByType     []wasteByProject                 `json:"wasteByType"`
	TopFiles        []topFile                        `json:"topFiles"`
	SubagentTree    reportTreeNode                   `json:"subagentTree"`
	ModelBreakdown  []modelBreakdown                 `json:"modelBreakdown"`
	Signals         []reportSignal                   `json:"signals"`
	SignalTimelines map[string][]reportTimelineEvent `json:"signalTimelines"`
}

type reportSummary struct {
	TotalCost     float64 `json:"totalCost"`
	TracedCost    float64 `json:"tracedCost"`
	TotalSignals  int     `json:"totalSignals"`
	HighSignals   int     `json:"highSignals"`
	MediumSignals int     `json:"mediumSignals"`
	LowSignals    int     `json:"lowSignals"`
	ProjectCount  int     `json:"projectCount"`
	Sessions      int     `json:"sessions"`
	DateFrom      string  `json:"dateFrom"`
	DateTo        string  `json:"dateTo"`
	DayCount      int     `json:"dayCount"`
	TotalToolLoop int     `json:"totalToolLoop"`
	TotalReRead   int     `json:"totalReRead"`
	TotalOverlap  int     `json:"totalOverlap"`
	TotalRestart  int     `json:"totalRestart"`
}

type costOverTimePoint struct {
	Date      string  `json:"date"`
	Cost      float64 `json:"cost"`
	MovingAvg float64 `json:"movingAvg"`
}

type wasteByProject struct {
	Project string             `json:"project"`
	Total   float64            `json:"total"`
	Reasons map[string]float64 `json:"reasons"`
}

type topFile struct {
	Path      string `json:"path"`
	ReadCount int    `json:"readCount"`
	Cost      float64 `json:"cost"`
}

type reportTreeNode struct {
	Name     string           `json:"name"`
	Cost     float64          `json:"value"`
	Children []reportTreeNode `json:"children,omitempty"`
}

type modelBreakdown struct {
	Model      string  `json:"model"`
	Cost       float64 `json:"cost"`
	Percentage float64 `json:"percentage"`
}

type reportSignal struct {
	SessionID   string  `json:"sessionId"`
	Project     string  `json:"project"`
	Severity    string  `json:"severity"`
	Reason      string  `json:"reason"`
	Detail      string  `json:"detail"`
	Cost        float64 `json:"cost"`
	Metric      float64 `json:"metric"`
	Threshold   float64 `json:"threshold"`
	Model       string  `json:"model"`
}

type reportTimelineEvent struct {
	EventIndex  int      `json:"index"`
	ToolCalls   []string `json:"toolCalls,omitempty"`
	FileOps     []string `json:"fileOps,omitempty"`
	Annotations []string `json:"annotations,omitempty"`
	IsSubagent  bool     `json:"isSubagent"`
	Cost        float64  `json:"cost"`
}

func computeReportData(events []source.TokenEvent, baselines map[string]analyze.Baseline, signals []analyze.WasteSignal, trees []analyze.SubagentTree) reportData {
	data := reportData{
		Summary:         computeReportSummary(events, signals),
		CostOverTime:    computeCostOverTime(events),
		WasteByType:     computeWasteByType(signals),
		TopFiles:       computeTopFiles(signals),
		SubagentTree:    computeSubagentTreeData(trees),
		ModelBreakdown:  computeModelBreakdown(events),
		Signals:         computeReportSignals(signals),
		SignalTimelines: computeSignalTimelines(signals, events, trees),
	}
	return data
}

func computeReportSummary(events []source.TokenEvent, signals []analyze.WasteSignal) reportSummary {
	var totalCost float64
	projects := make(map[string]bool)
	sessions := make(map[string]bool)
	var minTime, maxTime time.Time

	for _, e := range events {
		totalCost += e.CostUSD
		projects[e.Project] = true
		sessions[e.SessionID] = true
		if minTime.IsZero() || e.Timestamp.Before(minTime) {
			minTime = e.Timestamp
		}
		if e.Timestamp.After(maxTime) {
			maxTime = e.Timestamp
		}
	}

	var toolLoop, reRead, overlap, restart int
	var tracedCost float64
	highSignals := 0
	mediumSignals := 0
	lowSignals := 0
	for _, s := range signals {
		switch s.Reason {
		case "tool_call_loop":
			toolLoop++
		case "file_reread":
			reRead++
		case "subagent_overlap":
			overlap++
		case "session_restart":
			restart++
		}
		tracedCost += s.SessionCost
		switch s.Severity {
		case "high":
			highSignals++
		case "medium":
			mediumSignals++
		case "low":
			lowSignals++
		}
	}

	dayCount := 0
	if !minTime.IsZero() {
		dayCount = int(maxTime.Sub(minTime).Hours()/24) + 1
		if dayCount < 1 {
			dayCount = 1
		}
	}

	return reportSummary{
		TotalCost:     totalCost,
		TracedCost:    round2(tracedCost),
		TotalSignals:  len(signals),
		HighSignals:   highSignals,
		MediumSignals: mediumSignals,
		LowSignals:    lowSignals,
		ProjectCount:  len(projects),
		Sessions:      len(sessions),
		DateFrom:      minTime.Format("2006-01-02"),
		DateTo:        maxTime.Format("2006-01-02"),
		DayCount:      dayCount,
		TotalToolLoop: toolLoop,
		TotalReRead:   reRead,
		TotalOverlap:  overlap,
		TotalRestart:  restart,
	}
}

func computeCostOverTime(events []source.TokenEvent) []costOverTimePoint {
	if len(events) == 0 {
		return []costOverTimePoint{}
	}

	daily := make(map[string]float64)
	for _, e := range events {
		day := e.Timestamp.Format("2006-01-02")
		daily[day] += e.CostUSD
	}

	type kv struct {
		date string
		cost float64
	}
	var sorted []kv
	for date, cost := range daily {
		sorted = append(sorted, kv{date, cost})
	}
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].date < sorted[j].date
	})

	var points []costOverTimePoint
	for i, kv := range sorted {
		var avg float64
		window := 7
		start := i - window + 1
		if start < 0 {
			start = 0
		}
		count := i - start + 1
		sum := 0.0
		for j := start; j <= i; j++ {
			sum += sorted[j].cost
		}
		if count > 0 {
			avg = sum / float64(count)
		}
		points = append(points, costOverTimePoint{
			Date:      kv.date,
			Cost:      round2(kv.cost),
			MovingAvg: round2(avg),
		})
	}
	return points
}

func computeWasteByType(signals []analyze.WasteSignal) []wasteByProject {
	type wp struct {
		total   float64
		reasons map[string]float64
	}
	projects := make(map[string]*wp)

	for _, s := range signals {
		p := projects[s.Project]
		if p == nil {
			p = &wp{reasons: make(map[string]float64)}
			projects[s.Project] = p
		}
		p.total += s.SessionCost
		p.reasons[s.Reason] += s.SessionCost
	}

	var result []wasteByProject
	for name, p := range projects {
		result = append(result, wasteByProject{
			Project: name,
			Total:   round2(p.total),
			Reasons: p.reasons,
		})
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Total > result[j].Total
	})
	return result
}

func computeTopFiles(signals []analyze.WasteSignal) []topFile {
	type fileAgg struct {
		count int
		cost  float64
	}
	files := make(map[string]*fileAgg)

	for _, s := range signals {
		if s.Reason != "file_reread" {
			continue
		}
		path, readCount := ParseRereadDetail(s.Detail)
		if path == "" {
			continue
		}
		agg := files[path]
		if agg == nil {
			agg = &fileAgg{}
			files[path] = agg
		}
		agg.count += readCount
		agg.cost += s.SessionCost
	}

	var result []topFile
	for path, agg := range files {
		result = append(result, topFile{
			Path:      path,
			ReadCount: agg.count,
			Cost:      round2(agg.cost),
		})
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].ReadCount > result[j].ReadCount
	})

	const maxFiles = 15
	if len(result) > maxFiles {
		result = result[:maxFiles]
	}
	return result
}

func computeSubagentTreeData(trees []analyze.SubagentTree) reportTreeNode {
	root := reportTreeNode{Name: "sessions", Cost: 0}
	for _, t := range trees {
		sessionNode := reportTreeNode{
			Name: t.SessionID,
			Cost: round2(t.SubagentCost),
		}
		for _, n := range t.Subagents {
			sessionNode.Children = append(sessionNode.Children, convertSubagentNodeReport(n))
		}
		if t.TotalCost > 0 && len(t.Subagents) == 0 {
			sessionNode.Cost = round2(t.TotalCost)
		}
		root.Children = append(root.Children, sessionNode)
	}
	for _, c := range root.Children {
		root.Cost += c.Cost
	}
	root.Cost = round2(root.Cost)
	return root
}

func convertSubagentNodeReport(n analyze.SubagentNode) reportTreeNode {
	node := reportTreeNode{
		Name: n.SessionID,
		Cost: round2(n.Cost),
	}
	for _, c := range n.Children {
		node.Children = append(node.Children, convertSubagentNodeReport(c))
	}
	return node
}

func computeModelBreakdown(events []source.TokenEvent) []modelBreakdown {
	models := make(map[string]float64)
	var totalCost float64

	for _, e := range events {
		if e.Model == "" {
			continue
		}
		models[e.Model] += e.CostUSD
		totalCost += e.CostUSD
	}

	var result []modelBreakdown
	for model, cost := range models {
		pct := 0.0
		if totalCost > 0 {
			pct = (cost / totalCost) * 100
		}
		result = append(result, modelBreakdown{
			Model:      model,
			Cost:       round2(cost),
			Percentage: round2(pct),
		})
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Cost > result[j].Cost
	})
	return result
}

func computeReportSignals(signals []analyze.WasteSignal) []reportSignal {
	var result []reportSignal
	for _, s := range signals {
		result = append(result, reportSignal{
			SessionID: s.SessionID,
			Project:   s.Project,
			Severity:  s.Severity,
			Reason:    s.Reason,
			Detail:    s.Detail,
			Cost:      round2(s.SessionCost),
			Metric:    round2(s.Metric),
			Threshold: round2(s.Threshold),
			Model:     s.Model,
		})
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Severity != result[j].Severity {
			return severityRank(result[i].Severity) < severityRank(result[j].Severity)
		}
		return result[i].Cost > result[j].Cost
	})
	return result
}

func computeSignalTimelines(signals []analyze.WasteSignal, events []source.TokenEvent, trees []analyze.SubagentTree) map[string][]reportTimelineEvent {
	timelines := make(map[string][]reportTimelineEvent)

	signalSessions := make(map[string][]analyze.WasteSignal)
	for _, s := range signals {
		skip := s.Reason != "tool_call_loop" && s.Reason != "file_reread" && s.Reason != "subagent_overlap"
		if skip {
			continue
		}
		signalSessions[s.SessionID] = append(signalSessions[s.SessionID], s)
	}

	sessionEvents := make(map[string][]source.TokenEvent)
	for _, e := range events {
		if _, ok := signalSessions[e.SessionID]; !ok {
			continue
		}
		sessionEvents[e.SessionID] = append(sessionEvents[e.SessionID], e)
	}

	for sessionID, sevs := range sessionEvents {
		sorted := sortedEvents(sevs)
		anns := ComputeAnnotations(sorted, signalSessions[sessionID], trees)
		annMap := make(map[int][]string)
		for _, a := range anns {
			annMap[a.EventIndex] = append(annMap[a.EventIndex], a.Text)
		}

		var timeline []reportTimelineEvent
		for _, ev := range sorted {
			te := reportTimelineEvent{
				EventIndex: ev.EventIndex,
				IsSubagent: ev.IsSubagent,
				Cost:       round2(ev.CostUSD),
			}
			for _, tc := range ev.ToolCalls {
				te.ToolCalls = append(te.ToolCalls, toolCallSummary(tc))
			}
			for _, fo := range ev.FileOps {
				te.FileOps = append(te.FileOps, fo.Operation+" "+fo.Path)
			}
			if a := annMap[ev.EventIndex]; a != nil {
				te.Annotations = a
			}
			timeline = append(timeline, te)
		}
		timelines[sessionID] = timeline
	}
	return timelines
}

func toolCallSummary(tc source.ToolCall) string {
	name := tc.Name
	args := tc.Arguments
	if len(args) > 60 {
		args = args[:60] + "..."
	}
	if args != "" {
		return name + " " + args
	}
	return name
}

func round2(v float64) float64 {
	return float64(int(v*100+0.5)) / 100
}
