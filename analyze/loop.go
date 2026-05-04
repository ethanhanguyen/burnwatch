package analyze

import (
	"fmt"
	"sort"
	"strings"

	"github.com/ethanhanguyen/burnwatch/source"
)

func detectToolCallLoops(events []source.TokenEvent, maxRepeats int) []WasteSignal {
	if maxRepeats < 2 {
		return nil
	}

	sessions := groupEventsBySession(events)

	var signals []WasteSignal
	for _, evs := range sessions {
		signals = append(signals, detectSessionLoops(evs, maxRepeats)...)
	}
	return signals
}

type flatToolCall struct {
	name      string
	arguments string
	event     source.TokenEvent
}

func detectSessionLoops(events []source.TokenEvent, maxRepeats int) []WasteSignal {
	sort.Slice(events, func(i, j int) bool {
		return events[i].EventIndex < events[j].EventIndex
	})

	var flat []flatToolCall
	for _, ev := range events {
		if len(ev.ToolCalls) == 0 {
			continue
		}
		for _, tc := range ev.ToolCalls {
			flat = append(flat, flatToolCall{
				name:      tc.Name,
				arguments: tc.Arguments,
				event:     ev,
			})
		}
	}

	if len(flat) < maxRepeats {
		return nil
	}

	var prev flatToolCall
	repeatCount := 0
	loopedIdx := -1

	for i := range flat {
		cur := flat[i]
		if cur.name == prev.name && cur.arguments == prev.arguments {
			repeatCount++
			if repeatCount >= maxRepeats && loopedIdx == -1 {
				loopedIdx = i
			}
		} else {
			if loopedIdx >= 0 {
				break
			}
			repeatCount = 1
			prev = cur
		}
	}

	if loopedIdx < 0 {
		return nil
	}

	looped := flat[loopedIdx]
	ev := looped.event

	filePath := extractFilePath(looped.arguments)
	detail := looped.name
	if filePath != "" {
		detail += fmt.Sprintf("(\"%s\")", filePath)
	}
	detail += fmt.Sprintf(" called %d times consecutively in session", repeatCount)

	signal := WasteSignal{
		SessionID:       ev.SessionID,
		Project:         ev.Project,
		Severity:        "high",
		Reason:          "tool_call_loop",
		Detail:          detail,
		Metric:          float64(repeatCount),
		Threshold:       float64(maxRepeats),
		SessionCost:     0,
		Model:           ev.Model,
		InputTokens:     0,
		OutputTokens:    0,
		CostApproximate: ev.CostApproximate,
		CostUnknown:     ev.CostUnknown,
	}

	var totalCost float64
	for _, e := range events {
		totalCost += e.CostUSD
		if e.CostApproximate {
			signal.CostApproximate = true
		}
		if e.CostUnknown {
			signal.CostUnknown = true
		}
		if e.Model != "" {
			signal.Model = e.Model
		}
		signal.InputTokens += e.InputTokens
		signal.OutputTokens += e.OutputTokens
	}
	signal.SessionCost = totalCost

	return []WasteSignal{signal}
}

// extractFilePath returns the first "file_path" value from JSON-like arguments.
// Best-effort string scan, not a full JSON parse. Returns "" if no file_path found.
func extractFilePath(arguments string) string {
	idx := strings.Index(arguments, "\"file_path\"")
	if idx < 0 {
		return ""
	}
	rest := arguments[idx+len("\"file_path\""):]
	colonIdx := strings.Index(rest, ":")
	if colonIdx < 0 {
		return ""
	}
	rest = rest[colonIdx+1:]
	start := strings.Index(rest, "\"")
	if start < 0 {
		return ""
	}
	rest = rest[start+1:]
	end := strings.Index(rest, "\"")
	if end < 0 {
		return ""
	}
	return rest[:end]
}

func groupEventsBySession(events []source.TokenEvent) map[string][]source.TokenEvent {
	sessions := make(map[string][]source.TokenEvent)
	for _, ev := range events {
		sessions[ev.SessionID] = append(sessions[ev.SessionID], ev)
	}
	return sessions
}
