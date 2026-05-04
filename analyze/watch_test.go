package analyze

import (
	"testing"
	"time"

	"github.com/ethanhanguyen/burnwatch/source"
)

var watchTestCfg = PartialDetectionConfig{
	ToolLoopMaxRepeats: 5,
	FileRereadMinCount: 4,
	SubagentOverlapPct: 50.0,
}

func TestDiffEvents_NewEvents(t *testing.T) {
	events := []source.TokenEvent{
		{SessionID: "s1", EventIndex: 1, Timestamp: time.Now()},
		{SessionID: "s1", EventIndex: 2, Timestamp: time.Now()},
		{SessionID: "s2", EventIndex: 1, Timestamp: time.Now()},
		{SessionID: "s2", EventIndex: 2, Timestamp: time.Now()},
		{SessionID: "s1", EventIndex: 3, Timestamp: time.Now()},
	}

	seen := map[string]map[int]bool{
		"s1": {1: true},
		"s2": {1: true, 2: true},
	}

	newEvents := DiffEvents(events, seen)
	if len(newEvents) != 2 {
		t.Fatalf("expected 2 new events, got %d", len(newEvents))
	}

	for _, e := range newEvents {
		if e.SessionID == "s1" && e.EventIndex != 2 && e.EventIndex != 3 {
			t.Errorf("unexpected event: s1/%d", e.EventIndex)
		}
	}
}

func TestDiffEvents_AllNew(t *testing.T) {
	events := []source.TokenEvent{
		{SessionID: "s1", EventIndex: 1},
		{SessionID: "s1", EventIndex: 2},
	}

	seen := make(map[string]map[int]bool)
	newEvents := DiffEvents(events, seen)
	if len(newEvents) != 2 {
		t.Fatalf("expected 2 new events, got %d", len(newEvents))
	}
}

func TestDiffEvents_AllSeen(t *testing.T) {
	events := []source.TokenEvent{
		{SessionID: "s1", EventIndex: 1},
	}

	seen := map[string]map[int]bool{
		"s1": {1: true},
	}
	newEvents := DiffEvents(events, seen)
	if len(newEvents) != 0 {
		t.Fatalf("expected 0 new events, got %d", len(newEvents))
	}
}

func TestDetectPartialWaste_Loop(t *testing.T) {
	events := []source.TokenEvent{
		{SessionID: "ses_loop", Project: "test", EventIndex: 1, ToolCalls: []source.ToolCall{{Name: "read", Arguments: `{"file_path":"a.go"}`}}},
		{SessionID: "ses_loop", Project: "test", EventIndex: 2, ToolCalls: []source.ToolCall{{Name: "read", Arguments: `{"file_path":"a.go"}`}}},
		{SessionID: "ses_loop", Project: "test", EventIndex: 3, ToolCalls: []source.ToolCall{{Name: "read", Arguments: `{"file_path":"a.go"}`}}},
		{SessionID: "ses_loop", Project: "test", EventIndex: 4, ToolCalls: []source.ToolCall{{Name: "read", Arguments: `{"file_path":"a.go"}`}}},
	}

	session := &WatchSession{
		SessionID:       "ses_loop",
		Project:         "test",
		Events:          events,
		LastEventAt:    time.Now(),
	}

	signals := DetectPartialWaste(session, watchTestCfg)
	found := false
	for _, s := range signals {
		if s.Reason == "tool_call_loop" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected tool_call_loop signal with 4 repeats (threshold relaxed to 2)")
	}
}

func TestDetectPartialWaste_LoopNoSignal(t *testing.T) {
	events := []source.TokenEvent{
		{SessionID: "ses_clean", Project: "test", EventIndex: 1, ToolCalls: []source.ToolCall{{Name: "read", Arguments: `{"file_path":"a.go"}`}}},
		{SessionID: "ses_clean", Project: "test", EventIndex: 2, ToolCalls: []source.ToolCall{{Name: "read", Arguments: `{"file_path":"b.go"}`}}},
	}

	session := &WatchSession{
		SessionID:       "ses_clean",
		Project:         "test",
		Events:          events,
		LastEventAt:    time.Now(),
	}

	signals := DetectPartialWaste(session, watchTestCfg)
	if len(signals) > 0 {
		t.Errorf("expected no signals, got %d", len(signals))
	}
}

func TestDetectPartialWaste_ReRead(t *testing.T) {
	events := []source.TokenEvent{
		{SessionID: "ses_rr", Project: "test", EventIndex: 1, FileOps: []source.FileOp{{Path: "src/main.go", Operation: "read"}}},
		{SessionID: "ses_rr", Project: "test", EventIndex: 2, FileOps: []source.FileOp{{Path: "src/main.go", Operation: "read"}}},
		{SessionID: "ses_rr", Project: "test", EventIndex: 3, FileOps: []source.FileOp{{Path: "src/main.go", Operation: "read"}}},
	}

	session := &WatchSession{
		SessionID:       "ses_rr",
		Project:         "test",
		Events:          events,
		LastEventAt:    time.Now(),
	}

	signals := DetectPartialWaste(session, watchTestCfg)
	found := false
	for _, s := range signals {
		if s.Reason == "file_reread" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected file_reread signal with 3 reads (threshold relaxed to 2)")
	}
}

func TestDetectPartialWaste_ReReadWithCache(t *testing.T) {
	events := []source.TokenEvent{
		{SessionID: "ses_cache", Project: "test", EventIndex: 1, FileOps: []source.FileOp{{Path: "src/main.go", Operation: "read"}}},
		{SessionID: "ses_cache", Project: "test", EventIndex: 2, CacheRead: 1000},
		{SessionID: "ses_cache", Project: "test", EventIndex: 3, FileOps: []source.FileOp{{Path: "src/main.go", Operation: "read"}}},
	}

	session := &WatchSession{
		SessionID:       "ses_cache",
		Project:         "test",
		Events:          events,
		LastEventAt:    time.Now(),
	}

	signals := DetectPartialWaste(session, watchTestCfg)
	if len(signals) > 0 {
		t.Errorf("expected no signals (cache hits present), got %d", len(signals))
	}
}

func TestDetectPartialWaste_NoFalsePositive(t *testing.T) {
	events := []source.TokenEvent{
		{SessionID: "ses_clean", Project: "test", EventIndex: 1, ToolCalls: []source.ToolCall{{Name: "read", Arguments: `{"file_path":"a.go"}`}}},
		{SessionID: "ses_clean", Project: "test", EventIndex: 2, ToolCalls: []source.ToolCall{{Name: "edit", Arguments: `{"file_path":"a.go"}`}}},
		{SessionID: "ses_clean", Project: "test", EventIndex: 3, ToolCalls: []source.ToolCall{{Name: "glob", Arguments: `{"pattern":"**/*_test.go"}`}}},
		{SessionID: "ses_clean", Project: "test", EventIndex: 4, ToolCalls: []source.ToolCall{{Name: "bash", Arguments: `{"command":"go test ./..."}`}}},
		{SessionID: "ses_clean", Project: "test", EventIndex: 5, FileOps: []source.FileOp{{Path: "src/a.go", Operation: "read"}}},
		{SessionID: "ses_clean", Project: "test", EventIndex: 6, FileOps: []source.FileOp{{Path: "src/b.go", Operation: "read"}}},
		{SessionID: "ses_clean", Project: "test", EventIndex: 7, FileOps: []source.FileOp{{Path: "src/c.go", Operation: "read"}}},
	}

	session := &WatchSession{
		SessionID:       "ses_clean",
		Project:         "test",
		Events:          events,
		LastEventAt:    time.Now(),
	}

	signals := DetectPartialWaste(session, watchTestCfg)
	if len(signals) > 0 {
		t.Errorf("expected no signals for clean session, got %d", len(signals))
	}
}

func TestUpdateState_Merge(t *testing.T) {
	state := NewWatchState()

	now := time.Now()
	events := []source.TokenEvent{
		{SessionID: "s1", Project: "test", Harness: "claude-code", Model: "sonnet", EventIndex: 1, Timestamp: now, CostUSD: 0.50, InputTokens: 100, OutputTokens: 50, ToolCalls: []source.ToolCall{{Name: "read", Arguments: `{"file_path":"a.go"}`}}},
		{SessionID: "s1", Project: "test", Harness: "claude-code", Model: "sonnet", EventIndex: 2, Timestamp: now.Add(10 * time.Second), CostUSD: 0.30, InputTokens: 200, OutputTokens: 80, ToolCalls: []source.ToolCall{{Name: "edit", Arguments: `{"file_path":"a.go"}`}}},
	}

	UpdateState(state, events, watchTestCfg)

	ws, ok := state.Sessions["s1"]
	if !ok {
		t.Fatal("session s1 not found")
	}

	if len(ws.Events) != 2 {
		t.Errorf("expected 2 events, got %d", len(ws.Events))
	}
	if ws.Cost != 0.80 {
		t.Errorf("expected cost 0.80, got %.2f", ws.Cost)
	}
	if ws.InputTokens != 300 {
		t.Errorf("expected 300 input tokens, got %d", ws.InputTokens)
	}
	if ws.OutputTokens != 130 {
		t.Errorf("expected 130 output tokens, got %d", ws.OutputTokens)
	}
	if ws.Model != "sonnet" {
		t.Errorf("expected model sonnet, got %s", ws.Model)
	}
	if ws.LastAction == "" {
		t.Error("expected LastAction to be set")
	}
}

func TestUpdateState_Idle(t *testing.T) {
	state := NewWatchState()

	recent := time.Now()
	stale := time.Now().Add(-40 * time.Second)

	events := []source.TokenEvent{
		{SessionID: "active", Project: "test", EventIndex: 1, Timestamp: recent, CostUSD: 0.10},
		{SessionID: "idle", Project: "test", EventIndex: 1, Timestamp: stale, CostUSD: 0.10},
	}

	UpdateState(state, events, watchTestCfg)

	active, ok := state.Sessions["active"]
	if !ok {
		t.Fatal("active session not found")
	}
	if active.Idle {
		t.Error("active session should not be idle")
	}

	idle, ok := state.Sessions["idle"]
	if !ok {
		t.Fatal("idle session not found")
	}
	if !idle.Idle {
		t.Error("idle session should be idle (>30s)")
	}
}

func TestUpdateState_SubagentSpawnAlert(t *testing.T) {
	state := NewWatchState()

	events := []source.TokenEvent{
		{SessionID: "s1", Project: "test", EventIndex: 1, Timestamp: time.Now(), CostUSD: 0.10},
		{SessionID: "sub1", Project: "test", EventIndex: 1, Timestamp: time.Now(), CostUSD: 0.05, IsSubagent: true, AgentType: "explore"},
	}

	alerts := UpdateState(state, events, watchTestCfg)

	found := false
	for _, a := range alerts {
		if a.Reason == "subagent_spawn" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected subagent_spawn alert")
	}
}

func TestNilWatchState(t *testing.T) {
	alerts := UpdateState(nil, nil, watchTestCfg)
	if alerts != nil {
		t.Error("expected nil alerts for nil state")
	}
}

func TestPartialWaste_EmptySession(t *testing.T) {
	session := &WatchSession{
		SessionID:       "empty",
		Project:         "test",
		Events:          nil,
		LastEventAt:    time.Now(),
	}

	signals := DetectPartialWaste(session, watchTestCfg)
	if len(signals) > 0 {
		t.Errorf("expected no signals for empty session, got %d", len(signals))
	}

	var nilSession *WatchSession
	signals = DetectPartialWaste(nilSession, watchTestCfg)
	if len(signals) > 0 {
		t.Errorf("expected no signals for nil session, got %d", len(signals))
	}
}
