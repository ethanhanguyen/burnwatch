package analyze

import (
	"fmt"
	"time"

	"github.com/ethanhanguyen/burnwatch/config"
	"github.com/ethanhanguyen/burnwatch/source"
)

type WasteSignal struct {
	SessionID   string
	Project     string
	Severity    string
	Reason      string
	Detail      string
	Metric      float64
	Threshold   float64
	SessionCost float64

	Model           string
	InputTokens     int64
	OutputTokens    int64
	CostApproximate bool
}

type SignalToggles struct {
	CostOutlier          bool
	LowSignal            bool
	SubagentOverhead     bool
	CacheUnderutilized   bool
	FragmentationIndex   bool
	InputOverconsumption bool
	OutputExplosion      bool
	TokenEfficiency      bool
}

type sessionAgg struct {
	sessionID       string
	project         string
	harness         string
	cost            float64
	ratio           float64
	cacheRate       float64
	ter             float64
	inputSum        int64
	outputSum       int64
	cacheRead       int64
	cacheWrite      int64
	day             time.Time
	isSubagent      bool
	model           string
	costApproximate bool
}

func DetectWaste(events []source.TokenEvent, baselines map[string]Baseline,
	cfg config.Config, toggles SignalToggles) []WasteSignal {

	if len(events) == 0 || len(baselines) == 0 {
		return nil
	}

	agg := make(map[string]*sessionAgg)
	for _, e := range events {
		a, ok := agg[e.SessionID]
		if !ok {
			a = &sessionAgg{
				sessionID:  e.SessionID,
				project:    e.Project,
				harness:    e.Harness,
				day:        e.Timestamp.Truncate(24 * time.Hour),
				isSubagent: e.IsSubagent,
			}
			agg[e.SessionID] = a
		}

		a.cost += e.CostUSD
		if e.CostApproximate {
			a.costApproximate = true
		}

		in := e.InputTokens
		out := e.OutputTokens
		cr := e.CacheRead
		cw := e.CacheWrite
		if in < 0 {
			in = 0
		}
		if out < 0 {
			out = 0
		}
		if cr < 0 {
			cr = 0
		}
		if cw < 0 {
			cw = 0
		}
		a.inputSum += in
		a.outputSum += out
		a.cacheRead += cr
		a.cacheWrite += cw

		if e.Model != "" {
			a.model = e.Model
		}
	}

	for _, a := range agg {
		if a.inputSum > 0 {
			a.ratio = float64(a.outputSum) / float64(a.inputSum)
		}
		if total := a.cacheRead + a.cacheWrite; total > 0 {
			a.cacheRate = float64(a.cacheRead) / float64(total)
		}
		if a.inputSum+a.cacheWrite > 0 {
			a.ter = float64(a.outputSum+a.cacheRead) / float64(a.inputSum+a.cacheWrite)
		}
	}

	trees := BuildSubagentTree(events)
	treeBySession := make(map[string]*SubagentTree)
	for i := range trees {
		treeBySession[trees[i].SessionID] = &trees[i]
	}

	for _, a := range agg {
		if tree := treeBySession[a.sessionID]; tree != nil && tree.TotalCost > 0 {
			a.cost = tree.TotalCost
		}
	}

	global := baselines[globalKey]

	var signals []WasteSignal

	for _, a := range agg {
		key := a.project + ":" + a.harness
		bl := baselines[key]

		if toggles.CostOutlier {
			if signal := checkCostOutlier(a, bl, cfg.Thresholds.CostOutlierSigma); signal != nil {
				signals = append(signals, *signal)
			}
		}

		if toggles.LowSignal {
			if signal := checkLowSignal(a, global); signal != nil {
				signals = append(signals, *signal)
			}
		}

		if toggles.SubagentOverhead {
			if signal := checkSubagentOverhead(a, treeBySession, cfg.Thresholds.SubagentOverheadPct); signal != nil {
				signals = append(signals, *signal)
			}
		}

		if toggles.CacheUnderutilized {
			if signal := checkCacheUnderutilized(a, global); signal != nil {
				signals = append(signals, *signal)
			}
		}

		if toggles.InputOverconsumption {
			if signal := checkInputOverconsumption(a, bl, cfg.Thresholds.InputOverconsumptionSigma); signal != nil {
				signals = append(signals, *signal)
			}
		}

		if toggles.OutputExplosion {
			if signal := checkOutputExplosion(a, bl, cfg.Thresholds.OutputExplosionSigma); signal != nil {
				signals = append(signals, *signal)
			}
		}

		if toggles.TokenEfficiency && global.SessionCount >= 2 {
			if signal := checkTokenEfficiency(a, global); signal != nil {
				signals = append(signals, *signal)
			}
		}
	}

	if toggles.FragmentationIndex {
		signals = append(signals, checkFragmentationIndex(agg, baselines, cfg.Thresholds.FragmentationIndexThreshold, cfg.Thresholds.ChurnMinSessions)...)
	}

	sortSignals(signals)

	return signals
}

func checkCostOutlier(a *sessionAgg, bl Baseline, sigma float64) *WasteSignal {
	if bl.SessionCount < 2 {
		return nil
	}
	threshold := bl.CostMean + sigma*bl.CostStd
	if a.cost > threshold {
		return &WasteSignal{
			SessionID:       a.sessionID,
			Project:         a.project,
			Severity:        "high",
			Reason:          "cost_outlier",
			Detail:          fmt.Sprintf("Session cost $%.2f exceeds project baseline μ+%.0fσ = $%.2f", a.cost, sigma, threshold),
			Metric:          a.cost,
			Threshold:       threshold,
			SessionCost:     a.cost,
			Model:           a.model,
			InputTokens:     a.inputSum,
			OutputTokens:    a.outputSum,
			CostApproximate: a.costApproximate,
		}
	}
	return nil
}

func checkLowSignal(a *sessionAgg, global Baseline) *WasteSignal {
	if global.SessionCount < 2 {
		return nil
	}
	if a.inputSum == 0 {
		return nil
	}
	if a.ratio < global.RatioP10 {
		return &WasteSignal{
			SessionID:       a.sessionID,
			Project:         a.project,
			Severity:        "medium",
			Reason:          "low_signal",
			Detail:          fmt.Sprintf("Output/input ratio %.4f below global P10 (%.4f)", a.ratio, global.RatioP10),
			Metric:          a.ratio,
			Threshold:       global.RatioP10,
			SessionCost:     a.cost,
			Model:           a.model,
			InputTokens:     a.inputSum,
			OutputTokens:    a.outputSum,
			CostApproximate: a.costApproximate,
		}
	}
	return nil
}

func checkSubagentOverhead(a *sessionAgg, treeBySession map[string]*SubagentTree, thresholdPct float64) *WasteSignal {
	tree := treeBySession[a.sessionID]
	if tree == nil || tree.SubagentCost == 0 {
		return nil
	}
	if tree.OverheadPct > thresholdPct {
		return &WasteSignal{
			SessionID:       a.sessionID,
			Project:         a.project,
			Severity:        "medium",
			Reason:          "subagent_overhead",
			Detail:          fmt.Sprintf("Subagent overhead is %.1f%% of session cost ($%.2f of $%.2f)", tree.OverheadPct, tree.SubagentCost, tree.TotalCost),
			Metric:          tree.OverheadPct,
			Threshold:       thresholdPct,
			SessionCost:     tree.TotalCost,
			Model:           a.model,
			InputTokens:     a.inputSum,
			OutputTokens:    a.outputSum,
			CostApproximate: a.costApproximate,
		}
	}
	return nil
}

func checkCacheUnderutilized(a *sessionAgg, global Baseline) *WasteSignal {
	if global.SessionCount < 2 {
		return nil
	}
	total := a.cacheRead + a.cacheWrite
	if total == 0 {
		return nil
	}
	if a.cacheRate < global.CacheP10 {
		return &WasteSignal{
			SessionID:       a.sessionID,
			Project:         a.project,
			Severity:        "low",
			Reason:          "cache_underutilized",
			Detail:          fmt.Sprintf("Cache hit rate %.4f below global P10 (%.4f)", a.cacheRate, global.CacheP10),
			Metric:          a.cacheRate,
			Threshold:       global.CacheP10,
			SessionCost:     a.cost,
			Model:           a.model,
			InputTokens:     a.inputSum,
			OutputTokens:    a.outputSum,
			CostApproximate: a.costApproximate,
		}
	}
	return nil
}

func checkInputOverconsumption(a *sessionAgg, bl Baseline, sigma float64) *WasteSignal {
	if bl.SessionCount < 2 || bl.InputStd == 0 {
		return nil
	}
	if a.inputSum == 0 {
		return nil
	}
	threshold := bl.InputMean + sigma*bl.InputStd
	if float64(a.inputSum) > threshold {
		return &WasteSignal{
			SessionID:       a.sessionID,
			Project:         a.project,
			Severity:        "high",
			Reason:          "input_overconsumption",
			Detail:          fmt.Sprintf("%s input tokens (μ=%s, σ=%s)", formatTokens(a.inputSum), formatTokens(int64(bl.InputMean)), formatTokens(int64(bl.InputStd))),
			Metric:          float64(a.inputSum),
			Threshold:       threshold,
			SessionCost:     a.cost,
			Model:           a.model,
			InputTokens:     a.inputSum,
			OutputTokens:    a.outputSum,
			CostApproximate: a.costApproximate,
		}
	}
	return nil
}

func checkOutputExplosion(a *sessionAgg, bl Baseline, sigma float64) *WasteSignal {
	if bl.SessionCount < 2 || bl.OutputStd == 0 {
		return nil
	}
	if a.outputSum == 0 {
		return nil
	}
	threshold := bl.OutputMean + sigma*bl.OutputStd
	if float64(a.outputSum) > threshold {
		return &WasteSignal{
			SessionID:       a.sessionID,
			Project:         a.project,
			Severity:        "medium",
			Reason:          "output_explosion",
			Detail:          fmt.Sprintf("%s output tokens (μ=%s, σ=%s)", formatTokens(a.outputSum), formatTokens(int64(bl.OutputMean)), formatTokens(int64(bl.OutputStd))),
			Metric:          float64(a.outputSum),
			Threshold:       threshold,
			SessionCost:     a.cost,
			Model:           a.model,
			InputTokens:     a.inputSum,
			OutputTokens:    a.outputSum,
			CostApproximate: a.costApproximate,
		}
	}
	return nil
}

func checkTokenEfficiency(a *sessionAgg, global Baseline) *WasteSignal {
	if a.inputSum+a.cacheWrite == 0 {
		return nil
	}
	if a.ter < global.TERP10 {
		return &WasteSignal{
			SessionID:       a.sessionID,
			Project:         a.project,
			Severity:        "low",
			Reason:          "low_token_efficiency",
			Detail:          fmt.Sprintf("TER = %.2f (P10 = %.2f)", a.ter, global.TERP10),
			Metric:          a.ter,
			Threshold:       global.TERP10,
			SessionCost:     a.cost,
			Model:           a.model,
			InputTokens:     a.inputSum,
			OutputTokens:    a.outputSum,
			CostApproximate: a.costApproximate,
		}
	}
	return nil
}

func checkFragmentationIndex(agg map[string]*sessionAgg, baselines map[string]Baseline,
	fragThreshold float64, minSessions int) []WasteSignal {

	type dayKey struct {
		project string
		day     time.Time
	}
	groups := make(map[dayKey][]*sessionAgg)
	for _, a := range agg {
		dk := dayKey{a.project, a.day}
		groups[dk] = append(groups[dk], a)
	}

	var signals []WasteSignal
	seen := make(map[string]bool)

	for dk, sessions := range groups {
		if len(sessions) < minSessions {
			continue
		}

		var meanRatio float64
		for _, s := range sessions {
			meanRatio += s.ratio
		}
		meanRatio /= float64(len(sessions))

		fragIndex := float64(len(sessions)) * (1 - meanRatio)
		if fragIndex <= fragThreshold {
			continue
		}

		for _, s := range sessions {
			if seen[s.sessionID] {
				continue
			}
			seen[s.sessionID] = true
			signals = append(signals, WasteSignal{
				SessionID:       s.sessionID,
				Project:         s.project,
				Severity:        "medium",
				Reason:          "fragmentation_index",
				Detail:          fmt.Sprintf("Project %s had %d sessions on %s, fragmentation index = %.1f", dk.project, len(sessions), dk.day.Format("2006-01-02"), fragIndex),
				Metric:          fragIndex,
				Threshold:       fragThreshold,
				SessionCost:     s.cost,
				Model:           s.model,
				InputTokens:     s.inputSum,
				OutputTokens:    s.outputSum,
				CostApproximate: s.costApproximate,
			})
		}
	}

	return signals
}

func formatTokens(n int64) string {
	if n >= 1_000_000 {
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	}
	if n >= 1_000 {
		return fmt.Sprintf("%.1fK", float64(n)/1_000)
	}
	return fmt.Sprintf("%d", n)
}
