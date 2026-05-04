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

	case "input_overconsumption":
		r.Action = "Reduce input context bloat — check for repeated file reads or tool-call loops"
		r.Detail = "Session consumed excessive input tokens compared to project peers. The agent may be reading too much context per message."
		if bl := findBaseline(s.Project, baselines); bl != nil && s.InputTokens > 0 {
			r.SavingsEst = s.SessionCost * (1 - bl.InputMean/float64(s.InputTokens))
		}

	case "output_explosion":
		r.Action = "Check for runaway generation loops or repeated corrections"
		r.Detail = "Session generated excessive output tokens compared to project peers. Review for duplicate or corrective outputs."
		if bl := findBaseline(s.Project, baselines); bl != nil && s.OutputTokens > 0 {
			r.SavingsEst = s.SessionCost * (1 - bl.OutputMean/float64(s.OutputTokens))
		}

	case "low_token_efficiency":
		r.Action = "Low useful output per token consumed. Consider consolidating or using a cheaper model"
		r.Detail = fmt.Sprintf("TER = %.2f is below P10 (%.2f). Session produced very little output relative to context consumed.", s.Metric, s.Threshold)
		r.SavingsEst = s.SessionCost * 0.2

	case "fragmentation_index":
		r.Action = "Consolidate fragmented sessions — fewer, longer sessions cache better"
		r.Detail = fmt.Sprintf("%.0f sessions in one project on the same day with high fragmentation (index = %.1f).", s.Metric, s.Metric)
		r.SavingsEst = s.SessionCost * 0.7

	case "tool_call_loop":
		r.Action = "Investigate repeated tool calls — agent may be stuck in a loop"
		r.Detail = s.Detail
		r.SavingsEst = s.SessionCost * 0.5

	case "file_reread":
		r.Action = "Enable prompt caching to avoid re-reading unchanged files"
		r.Detail = s.Detail
		if s.SessionCost > 0 && s.Metric > 0 {
			r.SavingsEst = s.SessionCost * (s.Metric - 1) / s.Metric * 0.5
		}

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
