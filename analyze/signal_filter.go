package analyze

import "sort"

func FilterByMinCost(signals []WasteSignal, minCost float64) []WasteSignal {
	if minCost <= 0 {
		return signals
	}
	var filtered []WasteSignal
	for _, s := range signals {
		if s.SessionCost >= minCost {
			filtered = append(filtered, s)
		}
	}
	return filtered
}

func Deduplicate(signals []WasteSignal) []WasteSignal {
	if len(signals) == 0 {
		return signals
	}

	best := make(map[string]WasteSignal)
	for _, s := range signals {
		existing, ok := best[s.SessionID]
		if !ok || signalRank(s) > signalRank(existing) {
			best[s.SessionID] = s
		}
	}

	result := make([]WasteSignal, 0, len(best))
	for _, s := range best {
		result = append(result, s)
	}

	sortSignals(result)
	return result
}

func signalRank(s WasteSignal) int {
	sev := map[string]int{"high": 6, "medium": 4, "low": 2}
	reason := map[string]int{
		"cost_outlier":           5,
		"subagent_overhead":     4,
		"input_overconsumption": 4,
		"output_explosion":      3,
		"fragmentation_index":   3,
		"low_signal":            2,
		"low_token_efficiency":  2,
		"cache_underutilized":   1,
	}
	return sev[s.Severity] + reason[s.Reason]
}

func sortSignals(signals []WasteSignal) {
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
}
