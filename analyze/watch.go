package analyze

import (
	"fmt"
	"sort"
	"time"

	"github.com/ethanhanguyen/burnwatch/source"
)

type PartialDetectionConfig struct {
	ToolLoopMaxRepeats int
	FileRereadMinCount int
	SubagentOverlapPct float64
}

type WatchState struct {
	Sessions            map[string]*WatchSession
	Alerts              []WatchAlert
	seenEvents          map[string]map[int]bool
	lastPollTime        map[string]time.Time
	idlePollCount       map[string]int
	prevSessionFileMods map[string]time.Time
}

type WatchSession struct {
	SessionID       string
	Project         string
	Harness         string
	Model           string
	StartedAt       time.Time
	LastEventAt     time.Time
	Events          []source.TokenEvent
	Cost            float64
	InputTokens     int64
	OutputTokens    int64
	IsSubagent      bool
	ParentSessionID string
	AgentType       string
	EventFreq       [10]int
	LastAction      string
	Idle            bool
	Completed       bool
}

type WatchAlert struct {
	Time       time.Time
	SessionID  string
	Project    string
	Severity   string
	Reason     string
	Detail     string
}

func NewWatchState() *WatchState {
	return &WatchState{
		Sessions:            make(map[string]*WatchSession),
		seenEvents:          make(map[string]map[int]bool),
		lastPollTime:        make(map[string]time.Time),
		idlePollCount:       make(map[string]int),
		prevSessionFileMods: make(map[string]time.Time),
	}
}

func DiffEvents(all []source.TokenEvent, seen map[string]map[int]bool) []source.TokenEvent {
	var newEvents []source.TokenEvent
	for _, e := range all {
		if _, ok := seen[e.SessionID]; !ok {
			seen[e.SessionID] = make(map[int]bool)
		}
		key := e.EventIndex
		if !seen[e.SessionID][key] {
			seen[e.SessionID][key] = true
			newEvents = append(newEvents, e)
		}
	}
	return newEvents
}

func UpdateState(state *WatchState, newEvents []source.TokenEvent, cfg PartialDetectionConfig) []WatchAlert {
	if state == nil {
		return nil
	}

	var newAlerts []WatchAlert

	for _, e := range newEvents {
		ws, ok := state.Sessions[e.SessionID]
		if !ok {
			ws = &WatchSession{
				SessionID:       e.SessionID,
				Project:         e.Project,
				Harness:         e.Harness,
				Model:           e.Model,
				StartedAt:       e.Timestamp,
				LastEventAt:     e.Timestamp,
				IsSubagent:      e.IsSubagent,
				ParentSessionID: e.ParentSessionID,
				AgentType:       e.AgentType,
			}
			state.Sessions[e.SessionID] = ws
		}

		ws.Events = append(ws.Events, e)
		ws.LastEventAt = e.Timestamp

		if e.Timestamp.Before(ws.StartedAt) {
			ws.StartedAt = e.Timestamp
		}

		ws.Cost += e.CostUSD
		ws.InputTokens += e.InputTokens
		ws.OutputTokens += e.OutputTokens
		if e.Model != "" {
			ws.Model = e.Model
		}
		if e.IsSubagent {
			ws.IsSubagent = true
		}
		if e.ParentSessionID != "" {
			ws.ParentSessionID = e.ParentSessionID
		}
		if e.AgentType != "" {
			ws.AgentType = e.AgentType
		}

		ws.LastAction = lastActionText(e)

		now := time.Now()
		freqIdx := 0
		for i := 0; i < 10; i++ {
			age := time.Duration(9-i) * 5 * time.Second
			if e.Timestamp.After(now.Add(-age).Add(-5 * time.Second)) && e.Timestamp.Before(now.Add(-age).Add(5*time.Second)) {
				freqIdx = i
				break
			}
		}
		if freqIdx >= 0 && freqIdx < 10 {
			ws.EventFreq[freqIdx]++
		}

		state.lastPollTime[e.SessionID] = now

		if ws.IsSubagent && ws.ParentSessionID == "" && e.IsSubagent {
			sig := WatchAlert{
				Time:      time.Now(),
				SessionID: e.SessionID,
				Project:   e.Project,
				Severity:  "low",
				Reason:    "subagent_spawn",
				Detail:    fmt.Sprintf("Subagent spawned: %s", e.AgentType),
			}
			newAlerts = append(newAlerts, sig)
		}
	}

	for sid, ws := range state.Sessions {
		if time.Since(ws.LastEventAt) > 30*time.Second {
			ws.Idle = true
			state.idlePollCount[sid]++
		} else {
			ws.Idle = false
			state.idlePollCount[sid] = 0
		}
	}

	for _, ws := range state.Sessions {
		if !ws.Completed {
			signals := DetectPartialWaste(ws, cfg)
			for i := range signals {
				signals[i].SessionID = ws.SessionID
				signals[i].Project = ws.Project
				key := fmt.Sprintf("%s:%s:%s", signals[i].SessionID, signals[i].Reason, signals[i].Detail)
				found := false
				for _, a := range state.Alerts {
					if fmt.Sprintf("%s:%s:%s", a.SessionID, a.Reason, a.Detail) == key {
						found = true
						break
					}
				}
				if !found {
					alert := WatchAlert{
						Time:      time.Now(),
						SessionID: signals[i].SessionID,
						Project:   signals[i].Project,
						Severity:  signals[i].Severity,
						Reason:    signals[i].Reason,
						Detail:    signals[i].Detail,
					}
					newAlerts = append(newAlerts, alert)
				}
			}
		}
	}

	return newAlerts
}

func lastActionText(e source.TokenEvent) string {
	if len(e.ToolCalls) > 0 {
		name := e.ToolCalls[0].Name
		if path := extractFilePath(e.ToolCalls[0].Arguments); path != "" {
			return fmt.Sprintf("%s %s", name, path)
		}
		return name
	}
	if len(e.FileOps) > 0 {
		return fmt.Sprintf("%s %s", e.FileOps[0].Operation, e.FileOps[0].Path)
	}
	if e.StopReason != "" {
		return e.StopReason
	}
	return ""
}

func DetectPartialWaste(session *WatchSession, cfg PartialDetectionConfig) []WasteSignal {
	if session == nil || len(session.Events) == 0 {
		return nil
	}

	var signals []WasteSignal

	loopSignals := detectPartialToolLoop(session.Events, cfg.ToolLoopMaxRepeats)
	signals = append(signals, loopSignals...)

	rereadSignals := detectPartialFileReRead(session.Events, cfg.FileRereadMinCount)
	signals = append(signals, rereadSignals...)

	return signals
}

func detectPartialToolLoop(events []source.TokenEvent, maxRepeats int) []WasteSignal {
	relaxedThreshold := maxRepeats / 2
	if relaxedThreshold < 2 {
		relaxedThreshold = 2
	}

	sort.Slice(events, func(i, j int) bool {
		return events[i].EventIndex < events[j].EventIndex
	})

	type flatToolCall struct {
		name      string
		arguments string
		event     source.TokenEvent
	}

	var flat []flatToolCall
	for _, ev := range events {
		for _, tc := range ev.ToolCalls {
			flat = append(flat, flatToolCall{
				name:      tc.Name,
				arguments: tc.Arguments,
				event:     ev,
			})
		}
	}

	if len(flat) < relaxedThreshold {
		return nil
	}

	var prev flatToolCall
	repeatCount := 0
	loopedIdx := -1

	for i := range flat {
		cur := flat[i]
		if cur.name == prev.name && cur.arguments == prev.arguments {
			repeatCount++
			if repeatCount >= relaxedThreshold && loopedIdx == -1 {
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
	detail += fmt.Sprintf(" called %d times (partial session)", repeatCount)

	var totalCost float64
	var model string
	var costApprox, costUnknown bool
	var inputSum, outputSum int64

	for _, e := range events {
		totalCost += e.CostUSD
		if e.CostApproximate {
			costApprox = true
		}
		if e.CostUnknown {
			costUnknown = true
		}
		if e.Model != "" {
			model = e.Model
		}
		inputSum += e.InputTokens
		outputSum += e.OutputTokens
	}

	signal := WasteSignal{
		SessionID:       ev.SessionID,
		Project:         ev.Project,
		Severity:        "medium",
		Reason:          "tool_call_loop",
		Detail:          detail,
		Metric:          float64(repeatCount),
		Threshold:       float64(relaxedThreshold),
		SessionCost:     totalCost,
		Model:           model,
		InputTokens:     inputSum,
		OutputTokens:    outputSum,
		CostApproximate: costApprox,
		CostUnknown:     costUnknown,
	}

	return []WasteSignal{signal}
}

func detectPartialFileReRead(events []source.TokenEvent, minReReads int) []WasteSignal {
	relaxedThreshold := minReReads / 2
	if relaxedThreshold < 2 {
		relaxedThreshold = 2
	}

	if len(events) == 0 {
		return nil
	}

	sort.Slice(events, func(i, j int) bool {
		return events[i].EventIndex < events[j].EventIndex
	})

	type fileState struct {
		readCount    int
		firstReadIdx int
		lastReadIdx  int
		cacheReadSum int64
	}

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

	for _, ev := range events {
		if ev.CacheRead > 0 {
			for _, st := range files {
				if st.readCount >= relaxedThreshold && ev.EventIndex >= st.firstReadIdx && ev.EventIndex <= st.lastReadIdx {
					st.cacheReadSum += ev.CacheRead
				}
			}
		}
	}

	var sessionCost float64
	var model string
	var costApprox, costUnknown bool
	var inputSum, outputSum int64

	for _, ev := range events {
		sessionCost += ev.CostUSD
		if ev.CostApproximate {
			costApprox = true
		}
		if ev.CostUnknown {
			costUnknown = true
		}
		if ev.Model != "" {
			model = ev.Model
		}
		inputSum += ev.InputTokens
		outputSum += ev.OutputTokens
	}

	var signals []WasteSignal
	for path, st := range files {
		if st.readCount < relaxedThreshold {
			continue
		}
		if st.cacheReadSum > 0 {
			continue
		}

		detail := fmt.Sprintf("%s read %d times (partial session)", path, st.readCount)
		signals = append(signals, WasteSignal{
			SessionID:       events[0].SessionID,
			Project:         events[0].Project,
			Severity:        "medium",
			Reason:          "file_reread",
			Detail:          detail,
			Metric:          float64(st.readCount),
			Threshold:       float64(relaxedThreshold),
			SessionCost:     sessionCost,
			Model:           model,
			InputTokens:     inputSum,
			OutputTokens:    outputSum,
			CostApproximate: costApprox,
			CostUnknown:     costUnknown,
		})
	}

	return signals
}
