package output

import (
	"encoding/json"
	"sort"
	"time"

	"github.com/yourname/burnwatch/analyze"
	"github.com/yourname/burnwatch/source"
)

type JSONSummary struct {
	OpenCodeSessions         int     `json:"opencode_sessions"`
	OpenCodeSubagentSessions int     `json:"opencode_subagent_sessions"`
	ClaudeSessions           int     `json:"claude_sessions"`
	TodayCost                float64 `json:"today_cost"`
	TodaySessions            int     `json:"today_sessions"`
	WeekCost                 float64 `json:"week_cost"`
	WeekSessions             int     `json:"week_sessions"`
}

type JSONProject struct {
	Name         string  `json:"name"`
	Harness      string  `json:"harness"`
	SessionCount int     `json:"session_count"`
	TotalCost    float64 `json:"total_cost"`
	MedianCost   float64 `json:"median_cost"`
}

type JSONWasteSignal struct {
	SessionID   string  `json:"session_id"`
	Project     string  `json:"project"`
	Severity    string  `json:"severity"`
	Reason      string  `json:"reason"`
	Detail      string  `json:"detail"`
	Metric      float64 `json:"metric"`
	Threshold   float64 `json:"threshold"`
	SessionCost float64 `json:"session_cost,omitempty"`
}

type JSONSubagentNode struct {
	SessionID string             `json:"session_id"`
	AgentType string             `json:"agent_type"`
	Cost      float64            `json:"cost"`
	Children  []JSONSubagentNode `json:"children,omitempty"`
}

type JSONSubagentTree struct {
	SessionID    string             `json:"session_id"`
	TotalCost    float64            `json:"total_cost"`
	SubagentCost float64            `json:"subagent_cost"`
	OverheadPct  float64            `json:"overhead_pct"`
	Subagents    []JSONSubagentNode `json:"subagents,omitempty"`
}

type JSONRecommendation struct {
	Signal     string  `json:"signal"`
	Action     string  `json:"action"`
	Detail     string  `json:"detail"`
	SavingsEst float64 `json:"savings_est"`
}

type JSONReport struct {
	Summary          JSONSummary          `json:"summary"`
	Projects         []JSONProject        `json:"projects"`
	WasteSignals     []JSONWasteSignal    `json:"waste_signals"`
	SubagentTrees    []JSONSubagentTree   `json:"subagent_trees"`
	Recommendations  []JSONRecommendation `json:"recommendations"`
	PotentialSavings float64              `json:"potential_savings"`
}

func FormatJSON(
	events []source.TokenEvent,
	baselines map[string]analyze.Baseline,
	signals []analyze.WasteSignal,
	recommendations []analyze.Recommendation,
	trees []analyze.SubagentTree,
) ([]byte, error) {
	report := JSONReport{
		Summary:         buildJSONSummary(events),
		Projects:        buildJSONProjects(baselines),
		WasteSignals:    make([]JSONWasteSignal, 0),
		SubagentTrees:   make([]JSONSubagentTree, 0),
		Recommendations: make([]JSONRecommendation, 0),
	}

	for _, s := range signals {
		report.WasteSignals = append(report.WasteSignals, JSONWasteSignal{
			SessionID:   s.SessionID,
			Project:     s.Project,
			Severity:    s.Severity,
			Reason:      s.Reason,
			Detail:      s.Detail,
			Metric:      s.Metric,
			Threshold:   s.Threshold,
			SessionCost: s.SessionCost,
		})
	}

	for _, t := range trees {
		var subagents []JSONSubagentNode
		for _, sn := range t.Subagents {
			subagents = append(subagents, convertSubagentNode(sn))
		}
		report.SubagentTrees = append(report.SubagentTrees, JSONSubagentTree{
			SessionID:    t.SessionID,
			TotalCost:    t.TotalCost,
			SubagentCost: t.SubagentCost,
			OverheadPct:  t.OverheadPct,
			Subagents:    subagents,
		})
	}
	sort.Slice(report.SubagentTrees, func(i, j int) bool {
		return report.SubagentTrees[i].SessionID < report.SubagentTrees[j].SessionID
	})

	for _, r := range recommendations {
		report.Recommendations = append(report.Recommendations, JSONRecommendation{
			Signal:     r.Signal.Reason,
			Action:     r.Action,
			Detail:     r.Detail,
			SavingsEst: r.SavingsEst,
		})
		report.PotentialSavings += r.SavingsEst
	}

	return json.MarshalIndent(report, "", "  ")
}

func convertSubagentNode(n analyze.SubagentNode) JSONSubagentNode {
	node := JSONSubagentNode{
		SessionID: n.SessionID,
		AgentType: n.AgentType,
		Cost:      n.Cost,
	}
	for _, c := range n.Children {
		node.Children = append(node.Children, convertSubagentNode(c))
	}
	return node
}

func buildJSONSummary(events []source.TokenEvent) JSONSummary {
	var s JSONSummary

	harness := make(map[string]*harnessStats)

	todayStart := todayTrunc()
	weekStart := weekTrunc()

	todaySeen := make(map[string]bool)
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

		if !e.Timestamp.Before(todayStart) {
			s.TodayCost += e.CostUSD
			if !todaySeen[e.SessionID] {
				todaySeen[e.SessionID] = true
				s.TodaySessions++
			}
		}
		if !e.Timestamp.Before(weekStart) {
			s.WeekCost += e.CostUSD
			if !weekSeen[e.SessionID] {
				weekSeen[e.SessionID] = true
				s.WeekSessions++
			}
		}
	}

	if h, ok := harness["opencode"]; ok {
		s.OpenCodeSessions = len(h.sessions)
		s.OpenCodeSubagentSessions = len(h.subagentSessions)
	}
	if h, ok := harness["claude-code"]; ok {
		s.ClaudeSessions = len(h.sessions) + len(h.subagentSessions)
	}

	return s
}

func buildJSONProjects(baselines map[string]analyze.Baseline) []JSONProject {
	projects := make([]JSONProject, 0)
	for k, bl := range baselines {
		if k == "*" {
			continue
		}
		var totalCost float64
		for _, c := range bl.SessionCosts {
			totalCost += c
		}
		projects = append(projects, JSONProject{
			Name:         bl.Project,
			Harness:      bl.Harness,
			SessionCount: bl.SessionCount,
			TotalCost:    totalCost,
			MedianCost:   medianFromSorted(bl.SessionCosts),
		})
	}
	sort.Slice(projects, func(i, j int) bool {
		return projects[i].TotalCost > projects[j].TotalCost
	})
	return projects
}

var NowFunc = time.Now

func todayTrunc() time.Time {
	return NowFunc().Truncate(24 * time.Hour)
}

func weekTrunc() time.Time {
	return todayTrunc().AddDate(0, 0, -int(NowFunc().Weekday()))
}
