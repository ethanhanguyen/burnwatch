package analyze

import (
	"testing"

	"github.com/ethanhanguyen/burnwatch/source"
)

func mkRead(sessionID, path string) source.TokenEvent {
	return source.TokenEvent{
		SessionID: sessionID,
		Model:     "claude-sonnet-4-5-20250929",
		Project:   "test",
		Harness:   "claude-code",
		FileOps:   []source.FileOp{{Path: path, Operation: "read"}},
	}
}

func mkWrite(sessionID, path string) source.TokenEvent {
	return source.TokenEvent{
		SessionID: sessionID,
		Model:     "claude-sonnet-4-5-20250929",
		Project:   "test",
		Harness:   "claude-code",
		FileOps:   []source.FileOp{{Path: path, Operation: "write"}},
	}
}

func mkSubEvent(sessionID, parentID, agentType string) source.TokenEvent {
	return source.TokenEvent{
		SessionID:       sessionID,
		ParentSessionID: parentID,
		Model:           "claude-haiku-4-5-20251001",
		Project:         "test",
		Harness:         "claude-code",
		IsSubagent:      true,
		AgentType:       agentType,
	}
}

func mkSubRead(sessionID, parentID, agentType, path string, cost float64) source.TokenEvent {
	return source.TokenEvent{
		SessionID:       sessionID,
		ParentSessionID: parentID,
		AgentType:       agentType,
		Model:           "claude-haiku-4-5-20251001",
		Project:         "test",
		Harness:         "claude-code",
		IsSubagent:      true,
		CostUSD:         cost,
		FileOps:         []source.FileOp{{Path: path, Operation: "read"}},
	}
}

func TestDetectSubagentOverlap_NoSubagents(t *testing.T) {
	events := []source.TokenEvent{
		mkRead("parent", "src/main.go"),
	}
	trees := BuildSubagentTree(events)
	signals := detectSubagentOverlap(events, trees, 50.0)
	if len(signals) != 0 {
		t.Errorf("expected no signals, got %d: %v", len(signals), signals)
	}
}

func TestDetectSubagentOverlap_NoFileOps(t *testing.T) {
	events := []source.TokenEvent{
		{SessionID: "parent", Project: "test", Harness: "claude-code", Model: "claude-sonnet-4-5-20250929", CostUSD: 0.01},
		mkSubEvent("sub", "parent", "explore"),
	}
	trees := BuildSubagentTree(events)
	signals := detectSubagentOverlap(events, trees, 50.0)
	if len(signals) != 0 {
		t.Errorf("expected no signals for parent with no file ops, got %d", len(signals))
	}
}

func TestDetectSubagentOverlap_NoOverlap(t *testing.T) {
	events := []source.TokenEvent{
		mkRead("parent", "src/main.go"),
		mkSubRead("sub", "parent", "explore", "src/other.go", 0.01),
	}
	trees := BuildSubagentTree(events)
	signals := detectSubagentOverlap(events, trees, 50.0)
	if len(signals) != 0 {
		t.Errorf("expected no signals (0%% overlap), got %d", len(signals))
	}
}

func TestDetectSubagentOverlap_PartialOverlap(t *testing.T) {
	events := []source.TokenEvent{
		mkRead("parent", "a"),
		mkRead("parent", "b"),
		mkRead("parent", "c"),
		mkSubRead("sub", "parent", "explore", "a", 0.01),
		mkSubRead("sub", "parent", "explore", "b", 0.01),
	}
	trees := BuildSubagentTree(events)
	signals := detectSubagentOverlap(events, trees, 50.0)
	// parent={a,b,c}, sub={a,b}, overlap=2/3=66.7%, but jaccard=2/(3+2-2)=2/3=66.7%
	// Actually jaccard = |intersection| / |union| = 2 / 3 = 66.7% > 50%
	// So should flag
	if len(signals) == 0 {
		t.Fatal("expected signal for 66.7% overlap (partial at 3+2 files)")
	}
	if signals[0].Reason != "subagent_overlap" {
		t.Errorf("expected subagent_overlap, got %s", signals[0].Reason)
	}
}

func TestDetectSubagentOverlap_FullOverlap(t *testing.T) {
	events := []source.TokenEvent{
		mkRead("parent", "a"),
		mkRead("parent", "b"),
		mkSubRead("sub", "parent", "explore", "a", 0.02),
		mkSubRead("sub", "parent", "explore", "b", 0.02),
	}
	trees := BuildSubagentTree(events)
	signals := detectSubagentOverlap(events, trees, 50.0)
	if len(signals) == 0 {
		t.Fatal("expected signal for 100% overlap")
	}
	if signals[0].Severity != "high" {
		t.Errorf("expected severity high, got %s", signals[0].Severity)
	}
	if signals[0].Metric < 95 {
		t.Errorf("expected metric near 100, got %.1f", signals[0].Metric)
	}
}

func TestDetectSubagentOverlap_ThresholdBoundary(t *testing.T) {
	events := []source.TokenEvent{
		mkRead("parent", "a"),
		mkRead("parent", "b"),
		mkRead("parent", "c"),
		mkRead("parent", "d"),
		mkSubRead("sub", "parent", "explore", "a", 0.01),
		mkSubRead("sub", "parent", "explore", "b", 0.01),
		mkSubRead("sub", "parent", "explore", "c", 0.01),
	}
	trees := BuildSubagentTree(events)
	// parent={a,b,c,d}, sub={a,b,c}, union=4, intersection=3, jaccard=3/4=75%
	signals := detectSubagentOverlap(events, trees, 50.0)
	if len(signals) == 0 {
		t.Fatal("expected signal at threshold=50 (75% > 50%)")
	}
}

func TestDetectSubagentOverlap_MultipleSubagents(t *testing.T) {
	events := []source.TokenEvent{
		mkRead("parent", "a"),
		mkRead("parent", "b"),
		mkRead("parent", "c"),
		mkSubRead("sub1", "parent", "explore", "a", 0.01),
		mkSubRead("sub1", "parent", "explore", "b", 0.01),
		mkSubRead("sub1", "parent", "explore", "c", 0.01),
		mkSubRead("sub2", "parent", "general", "d", 0.01),
		mkSubRead("sub2", "parent", "general", "e", 0.01),
	}
	trees := BuildSubagentTree(events)
	signals := detectSubagentOverlap(events, trees, 50.0)

	// sub1: parent={a,b,c}, sub1={a,b,c}, jaccard=3/3=100% → signal
	// sub2: parent={a,b,c}, sub2={d,e}, jaccard=0/5=0% → no signal
	if len(signals) != 1 {
		t.Fatalf("expected 1 signal (sub1 only), got %d", len(signals))
	}
}

func TestDetectSubagentOverlap_EmptyEvents(t *testing.T) {
	signals := detectSubagentOverlap(nil, nil, 50.0)
	if len(signals) != 0 {
		t.Errorf("expected no signals for empty input, got %d", len(signals))
	}
}

func TestDetectSubagentOverlap_DedupedPaths(t *testing.T) {
	events := []source.TokenEvent{
		mkRead("parent", "src/main.go"),
		mkRead("parent", "src/main.go"),
		mkSubRead("sub", "parent", "explore", "src/main.go", 0.01),
	}
	trees := BuildSubagentTree(events)
	signals := detectSubagentOverlap(events, trees, 50.0)
	if len(signals) == 0 {
		t.Fatal("expected signal (1 unique file, subagent read same file)")
	}
	if signals[0].Metric != 100.0 {
		t.Errorf("expected 100%% overlap, got %.1f", signals[0].Metric)
	}
}

func TestDetectSubagentOverlap_WritesNotCounted(t *testing.T) {
	events := []source.TokenEvent{
		mkRead("parent", "a"),
		mkWrite("parent", "b"),
		mkSubRead("sub", "parent", "explore", "b", 0.01),
	}
	trees := BuildSubagentTree(events)
	signals := detectSubagentOverlap(events, trees, 50.0)
	// parent reads={a}, sub reads={b}, intersection=0, union=2, jaccard=0%
	if len(signals) != 0 {
		t.Errorf("expected no signal (writes not counted), got %d", len(signals))
	}
}

func TestDetectSubagentOverlap_ZeroThreshold(t *testing.T) {
	events := []source.TokenEvent{
		mkRead("parent", "a"),
		mkSubRead("sub", "parent", "explore", "a", 0.01),
	}
	trees := BuildSubagentTree(events)
	signals := detectSubagentOverlap(events, trees, 0)
	if len(signals) != 0 {
		t.Errorf("expected no signals for zero threshold, got %d", len(signals))
	}
}
