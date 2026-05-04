package analyze

import (
	"strings"
	"testing"
	"time"

	"github.com/ethanhanguyen/burnwatch/source"
)

func makeLoopEvents(sessionID string, toolCalls [][2]string, cost float64) []source.TokenEvent {
	events := make([]source.TokenEvent, 0, len(toolCalls))
	for i, tc := range toolCalls {
		events = append(events, source.TokenEvent{
			SessionID:       sessionID,
			Project:         "p",
			Harness:         "h",
			Model:           "test-model",
			Timestamp:       time.Date(2026, 5, 1, 10, i, 0, 0, time.UTC),
			InputTokens:     100,
			OutputTokens:    50,
			CostUSD:         cost,
			EventIndex:      i + 1,
			ToolCalls: []source.ToolCall{
				{Name: tc[0], Arguments: tc[1]},
			},
		})
	}
	return events
}

func TestToolCallLoop_NoLoop(t *testing.T) {
	events := makeLoopEvents("ses", [][2]string{
		{"read", `{"file_path":"a.go"}`},
		{"write", `{"file_path":"b.go"}`},
		{"read", `{"file_path":"c.go"}`},
	}, 1.0)
	events = append(events, makeLoopEvents("ses2", [][2]string{
		{"read", `{"file_path":"x.go"}`},
		{"edit", `{"file_path":"x.go"}`},
	}, 1.0)...)

	adjustEventIndex(events)
	signals := detectToolCallLoops(events, 5)
	if len(signals) != 0 {
		t.Errorf("expected no signals, got %d", len(signals))
	}
}

func TestToolCallLoop_ShortLoop(t *testing.T) {
	events := makeLoopEvents("ses", [][2]string{
		{"read", `{"file_path":"a.go"}`},
		{"read", `{"file_path":"a.go"}`},
		{"read", `{"file_path":"a.go"}`},
		{"read", `{"file_path":"a.go"}`},
	}, 1.0)

	adjustEventIndex(events)
	signals := detectToolCallLoops(events, 5)
	if len(signals) != 0 {
		t.Errorf("expected no signal for 4 repeats with threshold 5, got %d", len(signals))
	}
}

func TestToolCallLoop_LongLoop(t *testing.T) {
	events := makeLoopEvents("ses", [][2]string{
		{"read", `{"file_path":"a.go"}`},
		{"read", `{"file_path":"a.go"}`},
		{"read", `{"file_path":"a.go"}`},
		{"read", `{"file_path":"a.go"}`},
		{"read", `{"file_path":"a.go"}`},
		{"read", `{"file_path":"a.go"}`},
	}, 1.0)

	adjustEventIndex(events)
	signals := detectToolCallLoops(events, 5)
	if len(signals) == 0 {
		t.Fatal("expected signal for 6 repeats with threshold 5")
	}
	s := signals[0]
	if s.SessionID != "ses" {
		t.Errorf("SessionID = %s, want ses", s.SessionID)
	}
	if s.Severity != "high" {
		t.Errorf("Severity = %s, want high", s.Severity)
	}
	if s.Reason != "tool_call_loop" {
		t.Errorf("Reason = %s, want tool_call_loop", s.Reason)
	}
	if !strings.Contains(s.Detail, "read") {
		t.Errorf("Detail missing tool name: %s", s.Detail)
	}
	if !strings.Contains(s.Detail, "a.go") {
		t.Errorf("Detail missing file path: %s", s.Detail)
	}
}

func TestToolCallLoop_Interleaved(t *testing.T) {
	events := makeLoopEvents("ses", [][2]string{
		{"read", `{"file_path":"a.go"}`},
		{"edit", `{"file_path":"a.go","old_string":"x"}`},
		{"read", `{"file_path":"a.go"}`},
		{"edit", `{"file_path":"a.go","old_string":"y"}`},
		{"read", `{"file_path":"a.go"}`},
	}, 1.0)

	adjustEventIndex(events)
	signals := detectToolCallLoops(events, 3)
	if len(signals) != 0 {
		t.Errorf("interleaved reads are not consecutive, expected no signal, got %d", len(signals))
	}
}

func TestToolCallLoop_LoopAtEnd(t *testing.T) {
	events := makeLoopEvents("ses", [][2]string{
		{"read", `{"file_path":"a.go"}`},
		{"write", `{"file_path":"b.go"}`},
		{"read", `{"file_path":"x.go"}`},
		{"read", `{"file_path":"x.go"}`},
		{"read", `{"file_path":"x.go"}`},
		{"read", `{"file_path":"x.go"}`},
		{"read", `{"file_path":"x.go"}`},
		{"read", `{"file_path":"x.go"}`},
	}, 1.0)

	adjustEventIndex(events)
	signals := detectToolCallLoops(events, 5)
	if len(signals) == 0 {
		t.Fatal("expected signal for loop at end of session")
	}
	if signals[0].SessionID != "ses" {
		t.Errorf("SessionID = %s, want ses", signals[0].SessionID)
	}
}

func TestToolCallLoop_DifferentArgs(t *testing.T) {
	events := makeLoopEvents("ses", [][2]string{
		{"read", `{"file_path":"a.go"}`},
		{"read", `{"file_path":"b.go"}`},
		{"read", `{"file_path":"c.go"}`},
		{"read", `{"file_path":"a.go"}`},
		{"read", `{"file_path":"a.go"}`},
		{"read", `{"file_path":"a.go"}`},
	}, 1.0)

	adjustEventIndex(events)
	signals := detectToolCallLoops(events, 3)
	if len(signals) == 0 {
		t.Fatal("expected signal for last 3 reads of a.go")
	}
	if !strings.Contains(signals[0].Detail, "a.go") {
		t.Errorf("Detail should mention a.go: %s", signals[0].Detail)
	}
}

func TestToolCallLoop_EmptySession(t *testing.T) {
	events := []source.TokenEvent{
		{SessionID: "ses", Project: "p", Harness: "h", EventIndex: 1, Model: "m"},
	}

	signals := detectToolCallLoops(events, 5)
	if len(signals) != 0 {
		t.Errorf("expected no signal for empty session, got %d", len(signals))
	}
}

func TestToolCallLoop_MultipleSessions(t *testing.T) {
	events := makeLoopEvents("clean", [][2]string{
		{"read", `{"file_path":"x.go"}`},
		{"edit", `{"file_path":"x.go"}`},
	}, 1.0)
	events = append(events, makeLoopEvents("waste", [][2]string{
		{"bash", `{"command":"ls"}`},
		{"bash", `{"command":"ls"}`},
		{"bash", `{"command":"ls"}`},
		{"bash", `{"command":"ls"}`},
		{"bash", `{"command":"ls"}`},
		{"bash", `{"command":"ls"}`},
	}, 1.0)...)

	adjustEventIndex(events)
	signals := detectToolCallLoops(events, 5)
	if len(signals) != 1 {
		t.Fatalf("expected 1 signal, got %d", len(signals))
	}
	if signals[0].SessionID != "waste" {
		t.Errorf("SessionID = %s, want waste", signals[0].SessionID)
	}
}

func TestToolCallLoop_MultiToolCallsPerEvent(t *testing.T) {
	events := []source.TokenEvent{
		{SessionID: "ses", EventIndex: 1, ToolCalls: []source.ToolCall{
			{Name: "read", Arguments: `{"file_path":"a.go"}`},
		}},
		{SessionID: "ses", EventIndex: 2, ToolCalls: []source.ToolCall{
			{Name: "read", Arguments: `{"file_path":"a.go"}`},
			{Name: "read", Arguments: `{"file_path":"a.go"}`},
		}},
		{SessionID: "ses", EventIndex: 3, ToolCalls: []source.ToolCall{
			{Name: "read", Arguments: `{"file_path":"a.go"}`},
		}},
		{SessionID: "ses", EventIndex: 4, ToolCalls: []source.ToolCall{
			{Name: "read", Arguments: `{"file_path":"a.go"}`},
		}},
		{SessionID: "ses", EventIndex: 5, ToolCalls: []source.ToolCall{
			{Name: "read", Arguments: `{"file_path":"a.go"}`},
		}},
	}
	events[0].Project = "p"
	events[1].Project = "p"
	events[2].Project = "p"
	events[3].Project = "p"
	events[4].Project = "p"
	for i := range events {
		events[i].Model = "m"
		events[i].CostUSD = 0.01
	}

	signals := detectToolCallLoops(events, 5)
	if len(signals) == 0 {
		t.Fatal("expected signal for 5 consecutive reads (some events have 2 calls)")
	}
}

func TestExtractFilePath(t *testing.T) {
	tests := []struct {
		args     string
		expected string
	}{
		{`{"file_path":"src/main.go"}`, "src/main.go"},
		{`{"file_path":"a/b/c.go","limit":10}`, "a/b/c.go"},
		{`{"command":"ls"}`, ""},
		{``, ""},
	}
	for _, tt := range tests {
		got := extractFilePath(tt.args)
		if got != tt.expected {
			t.Errorf("extractFilePath(%q) = %q, want %q", tt.args, got, tt.expected)
		}
	}
}

func adjustEventIndex(events []source.TokenEvent) {
	sessionIdxs := make(map[string]int)
	for i := range events {
		sessionIdxs[events[i].SessionID]++
		events[i].EventIndex = sessionIdxs[events[i].SessionID]
	}
}
