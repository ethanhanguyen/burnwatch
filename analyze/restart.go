package analyze

import (
	"fmt"
	"sort"
	"strings"

	"github.com/ethanhanguyen/burnwatch/source"
)

type sessionMeta struct {
	sessionID  string
	project    string
	start      int64
	startTime  string
}

func detectSessionRestarts(events []source.TokenEvent, thresholdPct float64, initialOps int) []WasteSignal {
	if len(events) == 0 || thresholdPct <= 0 || thresholdPct > 100 || initialOps < 2 {
		return nil
	}

	eventsBySession := groupEventsBySession(events)

	byProject := make(map[string][]sessionMeta)
	for sid, evs := range eventsBySession {
		proj := evs[0].Project
		minTS := int64(0)
		for _, e := range evs {
			ts := e.Timestamp.Unix()
			if minTS == 0 || ts < minTS {
				minTS = ts
			}
		}
		byProject[proj] = append(byProject[proj], sessionMeta{
			sessionID: sid,
			project:   proj,
			start:     minTS,
			startTime: evs[0].Timestamp.Format("2006-01-02 15:04"),
		})
	}

	var signals []WasteSignal

	for _, sessions := range byProject {
		if len(sessions) < 2 {
			continue
		}

		sort.Slice(sessions, func(i, j int) bool {
			return sessions[i].start < sessions[j].start
		})

		for i := 1; i < len(sessions); i++ {
			a := sessions[i-1]
			b := sessions[i]

			aEvents := eventsBySession[a.sessionID]
			bEvents := eventsBySession[b.sessionID]

			sort.Slice(aEvents, func(i, j int) bool {
				return aEvents[i].EventIndex < aEvents[j].EventIndex
			})
			sort.Slice(bEvents, func(i, j int) bool {
				return bEvents[i].EventIndex < bEvents[j].EventIndex
			})

			initialA := firstNReadPaths(aEvents, initialOps)
			initialB := firstNReadPaths(bEvents, initialOps)

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

			overlapPct := float64(len(shared)) / float64(minLen) * 100

			if overlapPct >= thresholdPct {
				sessionCost := collectSessionCost(bEvents)
				model, inputSum, outputSum, costApprox, costUnknown := collectSessionMeta(bEvents)

				sharedPaths := make([]string, len(shared))
				copy(sharedPaths, shared)

				detail := fmt.Sprintf(
					"First %d file reads are %.0f%% identical to previous session %s.\n%d shared: %s.\nConsider continuing the prior session instead of starting fresh.",
					len(initialB), overlapPct, a.sessionID,
					len(shared), strings.Join(sharedPaths, ", "),
				)

				signals = append(signals, WasteSignal{
					SessionID:       b.sessionID,
					Project:         b.project,
					Severity:        "medium",
					Reason:          "session_restart",
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

func firstNReadPaths(events []source.TokenEvent, n int) []string {
	seen := make(map[string]bool)
	var paths []string
	for _, e := range events {
		for _, fo := range e.FileOps {
			if fo.Operation != "read" || fo.Path == "" {
				continue
			}
			if seen[fo.Path] {
				continue
			}
			seen[fo.Path] = true
			paths = append(paths, fo.Path)
			if len(paths) >= n {
				return paths
			}
		}
	}
	return paths
}
