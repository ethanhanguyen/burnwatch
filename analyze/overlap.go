package analyze

import (
	"fmt"
	"strings"

	"github.com/ethanhanguyen/burnwatch/source"
)

func detectSubagentOverlap(events []source.TokenEvent, trees []SubagentTree, thresholdPct float64) []WasteSignal {
	if len(events) == 0 || len(trees) == 0 || thresholdPct <= 0 || thresholdPct > 100 {
		return nil
	}

	threshold := thresholdPct / 100.0

	eventsBySession := make(map[string][]source.TokenEvent)
	for _, e := range events {
		eventsBySession[e.SessionID] = append(eventsBySession[e.SessionID], e)
	}

	var signals []WasteSignal

	for _, tree := range trees {
		if len(tree.Subagents) == 0 {
			continue
		}

		parentFiles := uniqueReadPaths(eventsBySession[tree.SessionID])
		if len(parentFiles) == 0 {
			continue
		}

		for _, sub := range tree.Subagents {
			subEvents := eventsBySession[sub.SessionID]
			subFiles := uniqueReadPaths(subEvents)
			if len(subFiles) == 0 {
				continue
			}

			intersection := intersectSets(parentFiles, subFiles)
			union := unionSets(parentFiles, subFiles)

			if len(union) == 0 {
				continue
			}

			jaccard := float64(len(intersection)) / float64(len(union))
			overlapPct := jaccard * 100

			if jaccard >= threshold {
				sessionCost := collectSessionCost(subEvents)
				model, inputSum, outputSum, costApprox, costUnknown := collectSessionMeta(subEvents)

				shared := make([]string, 0, len(intersection))
				for _, p := range intersection {
					shared = append(shared, p)
				}

				detail := fmt.Sprintf(
					"Parent session read %d unique files. Subagent \"%s\" read %d of the same %d.\nOverlap: %.0f%% (%d shared: %s)",
					len(parentFiles), sub.AgentType, len(intersection), len(parentFiles),
					overlapPct, len(intersection), strings.Join(shared, ", "),
				)

				signals = append(signals, WasteSignal{
					SessionID:       tree.SessionID,
					Project:         eventsBySession[tree.SessionID][0].Project,
					Severity:        "high",
					Reason:          "subagent_overlap",
					Detail:          detail,
					Metric:          overlapPct,
					Threshold:       thresholdPct,
					SessionCost:     sessionCost,
					Model:           model,
					InputTokens:     inputSum,
					OutputTokens:    outputSum,
					CostApproximate: costApprox,
					CostUnknown:     costUnknown,
				})
			}
		}
	}

	return signals
}

func uniqueReadPaths(events []source.TokenEvent) []string {
	seen := make(map[string]bool)
	var paths []string
	for _, e := range events {
		for _, fo := range e.FileOps {
			if fo.Operation == "read" && fo.Path != "" {
				if !seen[fo.Path] {
					seen[fo.Path] = true
					paths = append(paths, fo.Path)
				}
			}
		}
	}
	return paths
}

func intersectSets(a, b []string) []string {
	set := make(map[string]bool)
	for _, x := range a {
		set[x] = true
	}
	var result []string
	for _, x := range b {
		if set[x] {
			result = append(result, x)
			delete(set, x)
		}
	}
	return result
}

func unionSets(a, b []string) []string {
	set := make(map[string]bool)
	for _, x := range a {
		set[x] = true
	}
	for _, x := range b {
		set[x] = true
	}
	result := make([]string, 0, len(set))
	for k := range set {
		result = append(result, k)
	}
	return result
}

func collectSessionCost(events []source.TokenEvent) float64 {
	var cost float64
	for _, e := range events {
		cost += e.CostUSD
	}
	return cost
}

func collectSessionMeta(events []source.TokenEvent) (model string, inputSum, outputSum int64, costApprox, costUnknown bool) {
	for _, e := range events {
		if e.Model != "" {
			model = e.Model
		}
		inputSum += e.InputTokens
		outputSum += e.OutputTokens
		if e.CostApproximate {
			costApprox = true
		}
		if e.CostUnknown {
			costUnknown = true
		}
	}
	return
}
