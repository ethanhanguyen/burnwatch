package analyze

import (
	"testing"
)

func makeSignal(id, severity, reason string, cost float64) WasteSignal {
	return WasteSignal{
		SessionID:   id,
		Project:     "p",
		Severity:    severity,
		Reason:      reason,
		SessionCost: cost,
	}
}

func TestFilterByMinCost_ZeroThreshold(t *testing.T) {
	signals := []WasteSignal{
		makeSignal("s1", "high", "cost_outlier", 0),
		makeSignal("s2", "medium", "low_signal", 5),
		makeSignal("s3", "low", "cache_underutilized", 10),
	}
	got := FilterByMinCost(signals, 0)
	if len(got) != 3 {
		t.Errorf("expected all 3 to pass with minCost=0, got %d", len(got))
	}
}

func TestFilterByMinCost_Negative(t *testing.T) {
	signals := []WasteSignal{
		makeSignal("s1", "high", "cost_outlier", 0),
	}
	got := FilterByMinCost(signals, -1)
	if len(got) != 1 {
		t.Errorf("expected all to pass with minCost=-1 (no-op), got %d", len(got))
	}
}

func TestFilterByMinCost_Filters(t *testing.T) {
	signals := []WasteSignal{
		makeSignal("s1", "high", "cost_outlier", 0.50),
		makeSignal("s2", "medium", "low_signal", 2.00),
		makeSignal("s3", "low", "cache_underutilized", 5.00),
	}
	got := FilterByMinCost(signals, 2.00)
	if len(got) != 2 {
		t.Errorf("expected 2 signals with minCost=2.00, got %d", len(got))
	}
	for _, s := range got {
		if s.SessionCost < 2.00 {
			t.Errorf("signal %s cost %.2f below min 2.00", s.SessionID, s.SessionCost)
		}
	}
}

func TestFilterByMinCost_AllFiltered(t *testing.T) {
	signals := []WasteSignal{
		makeSignal("s1", "high", "cost_outlier", 1.00),
		makeSignal("s2", "medium", "low_signal", 1.50),
	}
	got := FilterByMinCost(signals, 10.00)
	if len(got) != 0 {
		t.Errorf("expected empty slice when all below min, got %d", len(got))
	}
}

func TestDeduplicate_SinglePerSession(t *testing.T) {
	signals := []WasteSignal{
		makeSignal("s1", "high", "cost_outlier", 10),
		makeSignal("s2", "medium", "low_signal", 5),
		makeSignal("s3", "low", "cache_underutilized", 2),
	}
	got := Deduplicate(signals)
	if len(got) != 3 {
		t.Errorf("expected 3 unique signals, got %d", len(got))
	}
}

func TestDeduplicate_HighBeatsMedium(t *testing.T) {
	signals := []WasteSignal{
		makeSignal("s1", "medium", "low_signal", 5),
		makeSignal("s1", "high", "cost_outlier", 10),
	}
	got := Deduplicate(signals)
	if len(got) != 1 {
		t.Fatalf("expected 1 signal after dedup, got %d", len(got))
	}
	if got[0].Severity != "high" {
		t.Errorf("expected HIGH to beat MEDIUM, got %s", got[0].Severity)
	}
}

func TestDeduplicate_MediumBeatsLow(t *testing.T) {
	signals := []WasteSignal{
		makeSignal("s1", "low", "cache_underutilized", 5),
		makeSignal("s1", "medium", "low_signal", 10),
	}
	got := Deduplicate(signals)
	if len(got) != 1 {
		t.Fatalf("expected 1 signal after dedup, got %d", len(got))
	}
	if got[0].Severity != "medium" {
		t.Errorf("expected MEDIUM to beat LOW, got %s", got[0].Severity)
	}
}

func TestDeduplicate_CostBeatsFrag(t *testing.T) {
	signals := []WasteSignal{
		makeSignal("s1", "medium", "fragmentation_index", 3),
		makeSignal("s1", "medium", "cost_outlier", 10),
	}
	got := Deduplicate(signals)
	if len(got) != 1 {
		t.Fatalf("expected 1 signal after dedup, got %d", len(got))
	}
	if got[0].Reason != "cost_outlier" {
		t.Errorf("expected cost_outlier to beat fragmentation_index, got %s", got[0].Reason)
	}
}

func TestDeduplicate_MultiSession(t *testing.T) {
	signals := []WasteSignal{
		makeSignal("s1", "high", "cost_outlier", 10),
		makeSignal("s1", "medium", "low_signal", 5),
		makeSignal("s2", "medium", "subagent_overhead", 8),
		makeSignal("s2", "low", "cache_underutilized", 2),
		makeSignal("s3", "low", "cache_underutilized", 1),
	}
	got := Deduplicate(signals)
	if len(got) != 3 {
		t.Fatalf("expected 3 signals (one per session), got %d", len(got))
	}
	sessions := make(map[string]bool)
	for _, s := range got {
		sessions[s.SessionID] = true
	}
	if len(sessions) != 3 {
		t.Errorf("expected 3 unique sessions, got %d", len(sessions))
	}
}
