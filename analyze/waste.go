package analyze

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/yourname/burnwatch/source"
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
}

type sessionAgg struct {
	sessionID  string
	project    string
	harness    string
	cost       float64
	ratio      float64
	cacheRate  float64
	inputSum   int64
	outputSum  int64
	cacheRead  int64
	cacheWrite int64
	day        time.Time
	isSubagent bool
}

func DetectWaste(events []source.TokenEvent, baselines map[string]Baseline) []WasteSignal {
	if len(events) == 0 || len(baselines) == 0 {
		return nil
	}

	agg := make(map[string]*sessionAgg)
	for _, e := range events {
		a, ok := agg[e.SessionID]
		if !ok {
			a = &sessionAgg{
				sessionID: e.SessionID,
				project:   e.Project,
				harness:   e.Harness,
				day:       e.Timestamp.Truncate(24 * time.Hour),
				isSubagent: e.IsSubagent,
			}
			agg[e.SessionID] = a
		}

		a.cost += e.CostUSD

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
	}

	for _, a := range agg {
		if a.inputSum > 0 {
			a.ratio = float64(a.outputSum) / float64(a.inputSum)
		}
		if total := a.cacheRead + a.cacheWrite; total > 0 {
			a.cacheRate = float64(a.cacheRead) / float64(total)
		}
	}

	trees := BuildSubagentTree(events)
	treeBySession := make(map[string]*SubagentTree)
	for i := range trees {
		treeBySession[trees[i].SessionID] = &trees[i]
	}

	global, _ := baselines[globalKey]

	var signals []WasteSignal

	for _, a := range agg {
		key := a.project + ":" + a.harness
		bl := baselines[key]

		if signal := checkCostOutlier(a, bl); signal != nil {
			signals = append(signals, *signal)
		}

		if signal := checkLowSignal(a, global); signal != nil {
			signals = append(signals, *signal)
		}

		if signal := checkSubagentOverhead(a, treeBySession); signal != nil {
			signals = append(signals, *signal)
		}

		if signal := checkCacheUnderutilized(a, global); signal != nil {
			signals = append(signals, *signal)
		}
	}

	signals = append(signals, checkSessionChurn(agg, baselines)...)

	sevOrder := map[string]int{"high": 3, "medium": 2, "low": 1}
	sort.Slice(signals, func(i, j int) bool {
		if signals[i].Severity != signals[j].Severity {
			return sevOrder[signals[i].Severity] > sevOrder[signals[j].Severity]
		}
		if signals[i].Reason != signals[j].Reason {
			return signals[i].Reason < signals[j].Reason
		}
		return signals[i].SessionID < signals[j].SessionID
	})

	return signals
}

func checkCostOutlier(a *sessionAgg, bl Baseline) *WasteSignal {
	if bl.SessionCount < 2 {
		return nil
	}
	threshold := bl.CostMean + 2*bl.CostStd
	if a.cost > threshold {
		return &WasteSignal{
			SessionID:   a.sessionID,
			Project:     a.project,
			Severity:    "high",
			Reason:      "cost_outlier",
			Detail:      fmt.Sprintf("Session cost $%.2f exceeds project baseline μ+2σ = $%.2f", a.cost, threshold),
			Metric:      a.cost,
			Threshold:   threshold,
			SessionCost: a.cost,
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
			SessionID:   a.sessionID,
			Project:     a.project,
			Severity:    "medium",
			Reason:      "low_signal",
			Detail:      fmt.Sprintf("Output/input ratio %.4f below global P10 (%.4f)", a.ratio, global.RatioP10),
			Metric:      a.ratio,
			Threshold:   global.RatioP10,
			SessionCost: a.cost,
		}
	}
	return nil
}

func checkSubagentOverhead(a *sessionAgg, treeBySession map[string]*SubagentTree) *WasteSignal {
	tree := treeBySession[a.sessionID]
	if tree == nil || tree.SubagentCost == 0 {
		return nil
	}
	if tree.OverheadPct > 50 {
		return &WasteSignal{
			SessionID:   a.sessionID,
			Project:     a.project,
			Severity:    "medium",
			Reason:      "subagent_overhead",
			Detail:      fmt.Sprintf("Subagent overhead is %.1f%% of session cost ($%.2f of $%.2f)", tree.OverheadPct, tree.SubagentCost, tree.TotalCost),
			Metric:      tree.OverheadPct,
			Threshold:   50,
			SessionCost: tree.TotalCost,
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
			SessionID:   a.sessionID,
			Project:     a.project,
			Severity:    "low",
			Reason:      "cache_underutilized",
			Detail:      fmt.Sprintf("Cache hit rate %.4f below global P10 (%.4f)", a.cacheRate, global.CacheP10),
			Metric:      a.cacheRate,
			Threshold:   global.CacheP10,
			SessionCost: a.cost,
		}
	}
	return nil
}

func checkSessionChurn(agg map[string]*sessionAgg, baselines map[string]Baseline) []WasteSignal {
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
		if len(sessions) < 3 {
			continue
		}

		var bl Baseline
		for k, b := range baselines {
			if k != globalKey && strings.HasPrefix(k, dk.project+":") {
				bl = b
				break
			}
		}

		if bl.SessionCount == 0 {
			continue
		}

		allBelow := true
		for _, s := range sessions {
			if s.ratio >= bl.RatioMean {
				allBelow = false
				break
			}
		}

		if allBelow {
			for _, s := range sessions {
				if seen[s.sessionID] {
					continue
				}
				seen[s.sessionID] = true
			signals = append(signals, WasteSignal{
				SessionID:   s.sessionID,
				Project:     s.project,
				Severity:    "medium",
				Reason:      "session_churn",
				Detail:      fmt.Sprintf("Project %s had %d sessions on %s, all below mean ratio (%.4f)", dk.project, len(sessions), dk.day.Format("2006-01-02"), bl.RatioMean),
				Metric:      float64(len(sessions)),
				Threshold:   2,
				SessionCost: s.cost,
			})
			}
		}
	}

	return signals
}
