package analyze

import (
	"fmt"
	"sort"

	"github.com/ethanhanguyen/burnwatch/source"
)

func detectFileReReads(events []source.TokenEvent, minReReads int) []WasteSignal {
	if minReReads < 2 {
		return nil
	}

	sessions := groupEventsBySession(events)

	var signals []WasteSignal
	for sid, evs := range sessions {
		signals = append(signals, detectSessionReReads(sid, evs, minReReads)...)
	}
	return signals
}

type fileState struct {
	readCount       int
	firstReadIdx    int
	lastReadIdx     int
	cacheReadSum    int64
}

func detectSessionReReads(sessionID string, events []source.TokenEvent, minReReads int) []WasteSignal {
	if len(events) == 0 {
		return nil
	}

	sort.Slice(events, func(i, j int) bool {
		return events[i].EventIndex < events[j].EventIndex
	})

	files := make(map[string]*fileState)

	for _, ev := range events {
		for _, fo := range ev.FileOps {
			if fo.Operation != "read" {
				continue
			}
			st, ok := files[fo.Path]
			if !ok {
				st = &fileState{firstReadIdx: ev.EventIndex}
				files[fo.Path] = st
			}
			st.readCount++
			st.lastReadIdx = ev.EventIndex
		}
	}

	var sessionCost float64
	var model string
	var costApproximate, costUnknown bool
	var inputSum, outputSum int64

	for _, ev := range events {
		sessionCost += ev.CostUSD
		if ev.CostApproximate {
			costApproximate = true
		}
		if ev.CostUnknown {
			costUnknown = true
		}
		if ev.Model != "" {
			model = ev.Model
		}
		inputSum += ev.InputTokens
		outputSum += ev.OutputTokens

		if ev.CacheRead > 0 {
			for _, st := range files {
				if st.readCount >= minReReads && ev.EventIndex >= st.firstReadIdx && ev.EventIndex <= st.lastReadIdx {
					st.cacheReadSum += ev.CacheRead
				}
			}
		}
	}

	var signals []WasteSignal
	for path, st := range files {
		if st.readCount < minReReads {
			continue
		}
		if st.cacheReadSum > 0 {
			continue
		}

		detail := fmt.Sprintf("%s read %d times, 0 cache hits between reads", path, st.readCount)
		signals = append(signals, WasteSignal{
			SessionID:       sessionID,
			Project:         events[0].Project,
			Severity:        "medium",
			Reason:          "file_reread",
			Detail:          detail,
			Metric:          float64(st.readCount),
			Threshold:       float64(minReReads),
			SessionCost:     sessionCost,
			Model:           model,
			InputTokens:     inputSum,
			OutputTokens:    outputSum,
			CostApproximate: costApproximate,
			CostUnknown:     costUnknown,
		})
	}

	return signals
}
