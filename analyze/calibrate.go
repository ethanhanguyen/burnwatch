package analyze

import (
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/ethanhanguyen/burnwatch/source"
)

type DistStats struct {
	Count int     `json:"count"`
	Mean  float64 `json:"mean"`
	Std   float64 `json:"std"`
	P10   float64 `json:"p10"`
	P25   float64 `json:"p25"`
	P50   float64 `json:"p50"`
	P75   float64 `json:"p75"`
	P90   float64 `json:"p90"`
	P95   float64 `json:"p95"`
	P99   float64 `json:"p99"`
	Min   float64 `json:"min"`
	Max   float64 `json:"max"`
}

type CalibrationReport struct {
	TotalSessions       int                   `json:"total_sessions"`
	TotalSubagents      int                   `json:"total_subagents"`
	ProjectCount        int                   `json:"project_count"`
	DateRangeStart      string                `json:"date_range_start"`
	DateRangeEnd        string                `json:"date_range_end"`
	SessionCost         DistStats             `json:"session_cost"`
	InputTokens         DistStats             `json:"input_tokens"`
	OutputTokens        DistStats             `json:"output_tokens"`
	Ratio               DistStats             `json:"output_input_ratio"`
	CacheHitRate        DistStats             `json:"cache_hit_rate"`
	TokenEfficiency     DistStats             `json:"token_efficiency_ratio"`
	SubagentOverhead    DistStats             `json:"subagent_overhead_pct"`
	ToolLoopMaxRepeats  DistStats             `json:"tool_loop_max_repeats"`
	FileReReadMaxCount  DistStats             `json:"file_reread_max_count"`
	SubagentOverlapPcts DistStats             `json:"subagent_overlap_pcts"`
	RestartOverlapPcts  DistStats             `json:"session_restart_overlap_pcts"`
	Suggestions         []ThresholdSuggestion `json:"suggestions"`
}

type ThresholdSuggestion struct {
	ConfigKey string  `json:"config_key"`
	Value     float64 `json:"value"`
	Rationale string  `json:"rationale"`
}

func ComputeCalibration(events []source.TokenEvent) CalibrationReport {
	sessionGroups := make(map[string][]source.TokenEvent)
	projectSet := make(map[string]bool)
	var minTime, maxTime time.Time
	subagentCount := 0

	for _, e := range events {
		sessionGroups[e.SessionID] = append(sessionGroups[e.SessionID], e)
		projectSet[e.Project] = true
		if e.IsSubagent {
			subagentCount++
		}
		if minTime.IsZero() || e.Timestamp.Before(minTime) {
			minTime = e.Timestamp
		}
		if e.Timestamp.After(maxTime) {
			maxTime = e.Timestamp
		}
	}

	var metrics []sessionMetrics
	for sid, sessEvents := range sessionGroups {
		m := aggregateMetrics(sid, sessEvents)
		metrics = append(metrics, m)
	}

	sortedCosts := make([]float64, len(metrics))
	sortedInputs := make([]float64, len(metrics))
	sortedOutputs := make([]float64, len(metrics))
	sortedRatios := make([]float64, len(metrics))
	var sortedCacheRates []float64
	var sortedTERs []float64

	for i, m := range metrics {
		sortedCosts[i] = m.cost
		sortedInputs[i] = float64(m.inputSum)
		sortedOutputs[i] = float64(m.outputSum)
		sortedRatios[i] = m.ratio
	}

	sort.Float64s(sortedCosts)
	sort.Float64s(sortedInputs)
	sort.Float64s(sortedOutputs)
	sort.Float64s(sortedRatios)

	for _, m := range metrics {
		if m.cacheRead+m.cacheWrite > 0 {
			sortedCacheRates = append(sortedCacheRates, m.cacheRate)
		}
	}
	sort.Float64s(sortedCacheRates)

	for _, m := range metrics {
		if m.inputSum+m.cacheWrite > 0 {
			sortedTERs = append(sortedTERs, m.ter)
		}
	}
	sort.Float64s(sortedTERs)

	trees := BuildSubagentTree(events)
	treeBySession := make(map[string]*SubagentTree)
	for i := range trees {
		treeBySession[trees[i].SessionID] = &trees[i]
	}

	var sortedOverheads []float64
	for _, m := range metrics {
		if tree, ok := treeBySession[m.sessionID]; ok && tree.SubagentCost > 0 {
			sortedOverheads = append(sortedOverheads, tree.OverheadPct)
		}
	}
	sort.Float64s(sortedOverheads)

	report := CalibrationReport{
		TotalSessions:       len(metrics),
		TotalSubagents:      subagentCount,
		ProjectCount:        len(projectSet),
		DateRangeStart:      minTime.Format("2006-01-02"),
		DateRangeEnd:        maxTime.Format("2006-01-02"),
		SessionCost:         computeDistStats(sortedCosts),
		InputTokens:         computeDistStats(sortedInputs),
		OutputTokens:        computeDistStats(sortedOutputs),
		Ratio:               computeDistStats(sortedRatios),
		CacheHitRate:        computeDistStats(sortedCacheRates),
		TokenEfficiency:     computeDistStats(sortedTERs),
		SubagentOverhead:    computeDistStats(sortedOverheads),
		ToolLoopMaxRepeats:  computeToolLoopDist(sessionGroups),
		FileReReadMaxCount:  computeReReadDist(sessionGroups),
		SubagentOverlapPcts: computeOverlapDist(sessionGroups, trees),
		RestartOverlapPcts:  computeRestartDist(sessionGroups),
	}

	report.Suggestions = generateSuggestions(
		sortedCosts,
		report.SessionCost,
		sortedInputs,
		report.InputTokens,
		sortedOutputs,
		report.OutputTokens,
		sortedRatios,
		report.Ratio,
		sortedCacheRates,
		report.CacheHitRate,
		sortedTERs,
		report.TokenEfficiency,
		sortedOverheads,
		report.SubagentOverhead,
		report.ToolLoopMaxRepeats,
		report.FileReReadMaxCount,
		report.SubagentOverlapPcts,
		report.RestartOverlapPcts,
	)

	return report
}

func computeDistStats(sorted []float64) DistStats {
	n := len(sorted)
	if n == 0 {
		return DistStats{}
	}
	if n == 1 {
		return DistStats{
			Count: 1,
			Mean:  sorted[0],
			Std:   0,
			P10:   sorted[0],
			P25:   sorted[0],
			P50:   sorted[0],
			P75:   sorted[0],
			P90:   sorted[0],
			P95:   sorted[0],
			P99:   sorted[0],
			Min:   sorted[0],
			Max:   sorted[0],
		}
	}

	return DistStats{
		Count: n,
		Mean:  mean(sorted),
		Std:   stddev(sorted, mean(sorted)),
		P10:   percentile(sorted, 10),
		P25:   percentile(sorted, 25),
		P50:   percentile(sorted, 50),
		P75:   percentile(sorted, 75),
		P90:   percentile(sorted, 90),
		P95:   percentile(sorted, 95),
		P99:   percentile(sorted, 99),
		Min:   sorted[0],
		Max:   sorted[n-1],
	}
}

func generateSuggestions(
	sortedCosts []float64, cost DistStats,
	sortedInputs []float64, input DistStats,
	sortedOutputs []float64, output DistStats,
	sortedRatios []float64, ratio DistStats,
	sortedCache []float64, cache DistStats,
	sortedTERs []float64, ter DistStats,
	sortedOverheads []float64, overhead DistStats,
	toolLoop DistStats,
	fileReRead DistStats,
	subagentOverlap DistStats,
	restartOverlap DistStats,
) []ThresholdSuggestion {
	var s []ThresholdSuggestion

	if cost.Std > 0 {
		p98 := percentile(sortedCosts, 98)
		suggestedSigma := (p98 - cost.Mean) / cost.Std
		if suggestedSigma < 1.5 {
			suggestedSigma = 2.0
		}
		suggestedSigma = math.Round(suggestedSigma*10) / 10

		flagPct := percentileRank(sortedCosts, cost.Mean+suggestedSigma*cost.Std)
		s = append(s, ThresholdSuggestion{
			ConfigKey: "cost_outlier_sigma",
			Value:     suggestedSigma,
			Rationale: fmt.Sprintf("flags ~%.0f%% of sessions as cost outliers", 100.0-flagPct),
		})
	}

	if input.Std > 0 {
		p98 := percentile(sortedInputs, 98)
		suggestedSigma := (p98 - input.Mean) / input.Std
		if suggestedSigma < 1.5 {
			suggestedSigma = 2.0
		}
		suggestedSigma = math.Round(suggestedSigma*10) / 10

		flagPct := percentileRank(sortedInputs, input.Mean+suggestedSigma*input.Std)
		s = append(s, ThresholdSuggestion{
			ConfigKey: "input_overconsumption_sigma",
			Value:     suggestedSigma,
			Rationale: fmt.Sprintf("flags ~%.0f%% of sessions as input over-consumers", 100.0-flagPct),
		})
	}

	if output.Std > 0 {
		p98 := percentile(sortedOutputs, 98)
		suggestedSigma := (p98 - output.Mean) / output.Std
		if suggestedSigma < 1.5 {
			suggestedSigma = 2.0
		}
		suggestedSigma = math.Round(suggestedSigma*10) / 10

		flagPct := percentileRank(sortedOutputs, output.Mean+suggestedSigma*output.Std)
		s = append(s, ThresholdSuggestion{
			ConfigKey: "output_explosion_sigma",
			Value:     suggestedSigma,
			Rationale: fmt.Sprintf("flags ~%.0f%% of sessions as output explosion", 100.0-flagPct),
		})
	}

	if ratio.Count > 0 {
		s = append(s, ThresholdSuggestion{
			ConfigKey: "low_signal_percentile",
			Value:     5.0,
			Rationale: "stricter than default P10 — only flags bottom 5% of ratios",
		})
	}

	if cache.Count > 0 {
		p10 := percentile(sortedCache, 10)
		s = append(s, ThresholdSuggestion{
			ConfigKey: "cache_percentile",
			Value:     10.0,
			Rationale: fmt.Sprintf("flags sessions below P10 cache hit rate (%.1f%%)", p10),
		})
	}

	if ter.Count > 0 {
		s = append(s, ThresholdSuggestion{
			ConfigKey: "token_efficiency_percentile",
			Value:     5.0,
			Rationale: "flags bottom 5% of token efficiency ratios",
		})
	}

	if overhead.Count > 0 {
		p75 := percentile(sortedOverheads, 75)
		p75Rounded := math.Round(p75)
		s = append(s, ThresholdSuggestion{
			ConfigKey: "subagent_overhead_pct",
			Value:     p75Rounded,
			Rationale: "P75 of subagent overhead — flags sessions in top quartile",
		})
	}

	if toolLoop.Count > 0 {
		p95 := math.Ceil(toolLoop.P95)
		if p95 < 5 {
			p95 = 5
		}
		s = append(s, ThresholdSuggestion{
			ConfigKey: "tool_loop_max_repeats",
			Value:     p95,
			Rationale: fmt.Sprintf("P95 of consecutive same-tool repeats is %.0f — flag sessions exceeding this", toolLoop.P95),
		})
	}

	if fileReRead.Count > 0 {
		p95 := math.Ceil(fileReRead.P95)
		if p95 < 3 {
			p95 = 3
		}
		s = append(s, ThresholdSuggestion{
			ConfigKey: "file_reread_min_count",
			Value:     p95,
			Rationale: fmt.Sprintf("P95 of file re-read count per session is %.0f — flag sessions exceeding this", fileReRead.P95),
		})
	}

	if subagentOverlap.Count > 0 {
		p95 := math.Ceil(subagentOverlap.P95)
		if p95 < 30 {
			p95 = 50
		}
		s = append(s, ThresholdSuggestion{
			ConfigKey: "subagent_overlap_pct",
			Value:     p95,
			Rationale: fmt.Sprintf("P95 of subagent-parent file overlap is %.0f%% — flag subagents exceeding this", subagentOverlap.P95),
		})
	}

	if restartOverlap.Count > 0 {
		p95 := math.Ceil(restartOverlap.P95)
		if p95 < 50 {
			p95 = 80
		}
		s = append(s, ThresholdSuggestion{
			ConfigKey: "session_restart_pct",
			Value:     p95,
			Rationale: fmt.Sprintf("P95 of session restart overlap is %.0f%% — flag sessions exceeding this", restartOverlap.P95),
		})
	}

	return s
}

func percentileRank(sorted []float64, value float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	count := 0
	for _, v := range sorted {
		if v <= value {
			count++
		}
	}
	return 100.0 * float64(count) / float64(len(sorted))
}

func computeToolLoopDist(sessionGroups map[string][]source.TokenEvent) DistStats {
	var repeats []float64
	for _, evs := range sessionGroups {
		maxRepeat := maxConsecutiveRepeats(evs)
		repeats = append(repeats, float64(maxRepeat))
	}
	sort.Float64s(repeats)
	return computeDistStats(repeats)
}

func maxConsecutiveRepeats(events []source.TokenEvent) int {
	sort.Slice(events, func(i, j int) bool {
		return events[i].EventIndex < events[j].EventIndex
	})

	type flat struct {
		name string
		args string
	}
	var flatCalls []flat
	for _, ev := range events {
		for _, tc := range ev.ToolCalls {
			flatCalls = append(flatCalls, flat{name: tc.Name, args: tc.Arguments})
		}
	}

	maxRepeat := 0
	currentRepeat := 0
	var prev flat
	for i, cur := range flatCalls {
		if i == 0 {
			prev = cur
			currentRepeat = 1
			continue
		}
		if cur.name == prev.name && cur.args == prev.args {
			currentRepeat++
		} else {
			if currentRepeat > maxRepeat {
				maxRepeat = currentRepeat
			}
			currentRepeat = 1
			prev = cur
		}
	}
	if currentRepeat > maxRepeat {
		maxRepeat = currentRepeat
	}
	return maxRepeat
}

func computeReReadDist(sessionGroups map[string][]source.TokenEvent) DistStats {
	var rereads []float64
	for _, evs := range sessionGroups {
		maxReRead := maxFileReReads(evs)
		rereads = append(rereads, float64(maxReRead))
	}
	sort.Float64s(rereads)
	return computeDistStats(rereads)
}

func maxFileReReads(events []source.TokenEvent) int {
	readCounts := make(map[string]int)
	for _, ev := range events {
		for _, fo := range ev.FileOps {
			if fo.Operation == "read" {
				readCounts[fo.Path]++
			}
		}
	}
	maxCount := 0
	for _, count := range readCounts {
		if count > maxCount {
			maxCount = count
		}
	}
	return maxCount
}

func computeOverlapDist(sessionGroups map[string][]source.TokenEvent, trees []SubagentTree) DistStats {
	eventsBySession := sessionGroups
	treeBySession := make(map[string]*SubagentTree)
	for i := range trees {
		treeBySession[trees[i].SessionID] = &trees[i]
	}

	var overlapPcts []float64
	for _, tree := range trees {
		if len(tree.Subagents) == 0 {
			continue
		}
		parentFiles := uniqueReadPaths(eventsBySession[tree.SessionID])
		if len(parentFiles) == 0 {
			continue
		}
		for _, sub := range tree.Subagents {
			subFiles := uniqueReadPaths(eventsBySession[sub.SessionID])
			if len(subFiles) == 0 {
				continue
			}
			intersection := intersectSets(parentFiles, subFiles)
			union := unionSets(parentFiles, subFiles)
			if len(union) == 0 {
				continue
			}
			jaccard := float64(len(intersection)) / float64(len(union))
			overlapPcts = append(overlapPcts, jaccard*100)
		}
	}
	sort.Float64s(overlapPcts)
	return computeDistStats(overlapPcts)
}

func computeRestartDist(sessionGroups map[string][]source.TokenEvent) DistStats {
	type projSession struct {
		sessionID string
		events    []source.TokenEvent
	}
	byProject := make(map[string][]projSession)
	for sid, evs := range sessionGroups {
		proj := evs[0].Project
		byProject[proj] = append(byProject[proj], projSession{sessionID: sid, events: evs})
	}

	const initialOps = 10

	var overlapPcts []float64
	for _, sessions := range byProject {
		if len(sessions) < 2 {
			continue
		}
		sort.Slice(sessions, func(i, j int) bool {
			return sessions[i].events[0].Timestamp.Before(sessions[j].events[0].Timestamp)
		})

		for i := 1; i < len(sessions); i++ {
			a := sessions[i-1]
			b := sessions[i]

			sort.Slice(a.events, func(i, j int) bool {
				return a.events[i].EventIndex < a.events[j].EventIndex
			})
			sort.Slice(b.events, func(i, j int) bool {
				return b.events[i].EventIndex < b.events[j].EventIndex
			})

			initialA := firstNReadPaths(a.events, initialOps)
			initialB := firstNReadPaths(b.events, initialOps)

			if len(initialA) == 0 || len(initialB) == 0 {
				continue
			}

			shared := intersectSets(initialA, initialB)
			minLen := len(initialA)
			if len(initialB) < minLen {
				minLen = len(initialB)
			}

			if minLen == 0 || len(shared) < 2 {
				continue
			}

			pct := float64(len(shared)) / float64(minLen) * 100
			if pct > 0 {
				overlapPcts = append(overlapPcts, pct)
			}
		}
	}
	sort.Float64s(overlapPcts)
	return computeDistStats(overlapPcts)
}
