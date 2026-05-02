package analyze

import (
	"fmt"
	"strings"
)

type Recommendation struct {
	Signal     WasteSignal
	Action     string
	Detail     string
	SavingsEst float64
}

func GenerateRecommendations(signals []WasteSignal, baselines map[string]Baseline) []Recommendation {
	if len(signals) == 0 {
		return nil
	}

	var recs []Recommendation
	for _, s := range signals {
		r := recommendForSignal(s, baselines)
		recs = append(recs, r)
	}
	return recs
}

func recommendForSignal(s WasteSignal, baselines map[string]Baseline) Recommendation {
	r := Recommendation{Signal: s}

	switch s.Reason {
	case "cost_outlier":
		r.Action = "Investigate session for unnecessary loops or re-prompts"
		r.Detail = fmt.Sprintf("This session cost $%.2f which exceeds the project baseline μ+2σ = $%.2f.", s.Metric, s.Threshold)
		if bl := findBaseline(s.Project, baselines); bl != nil {
			r.SavingsEst = s.Metric - bl.CostMean
		}

	case "low_signal":
		r.Action = "Consider whether this task needed full agent interaction"
		r.Detail = fmt.Sprintf("Output/input ratio %.4f is below P10 (%.4f). The agent consumed context without producing meaningful output.", s.Metric, s.Threshold)
		r.SavingsEst = s.SessionCost * 0.5

	case "subagent_overhead":
		r.Action = "Evaluate whether subagent delegation was necessary vs inline"
		r.Detail = fmt.Sprintf("Subagent overhead is %.1f%% of total session cost (threshold: %.1f%%).", s.Metric, s.Threshold)
		if s.SessionCost > 0 {
			subagentCost := s.SessionCost * s.Metric / 100.0
			r.SavingsEst = subagentCost * 0.7
		}

	case "cache_underutilized":
		r.Action = "Consider CLAUDE.md / skills optimization for better caching"
		r.Detail = fmt.Sprintf("Cache hit rate %.4f is below P10 (%.4f). Optimize instruction files to improve cache reuse.", s.Metric, s.Threshold)
		r.SavingsEst = s.SessionCost * 0.2

	case "session_churn":
		r.Action = "Consolidate fragmented sessions — fewer, longer sessions cache better"
		r.Detail = fmt.Sprintf("%.0f sessions in one project on the same day, all below the mean output/input ratio.", s.Metric)
		r.SavingsEst = s.SessionCost

	default:
		r.Action = "Review this session for optimization opportunities"
		r.Detail = s.Detail
	}

	return r
}

func findBaseline(project string, baselines map[string]Baseline) *Baseline {
	for k, b := range baselines {
		if k == globalKey {
			continue
		}
		if strings.HasPrefix(k, project+":") {
			return &b
		}
	}
	return nil
}
