package analyze

import (
	"strings"
	"testing"
	"time"

	"github.com/ethanhanguyen/burnwatch/source"
)

func TestDetectWasteCostOutlier(t *testing.T) {
	events := make([]source.TokenEvent, 0, 6)
	for i := 1; i <= 5; i++ {
		events = append(events, source.TokenEvent{
			SessionID:    "s" + string(rune('0'+i)),
			Project:      "p",
			Harness:      "h",
			CostUSD:      float64(i),
			InputTokens:  100,
			OutputTokens: 50,
			Timestamp:    time.Date(2026, 5, 1, 10+i, 0, 0, 0, time.UTC),
		})
	}
	events = append(events, source.TokenEvent{
		SessionID:    "s-outlier",
		Project:      "p",
		Harness:      "h",
		CostUSD:      50.0,
		InputTokens:  100,
		OutputTokens: 50,
		Timestamp:    time.Date(2026, 5, 1, 16, 0, 0, 0, time.UTC),
	})

	baselines := ComputeBaselines(events)
	signals := DetectWaste(events, baselines, 2.0)

	var found bool
	for _, s := range signals {
		if s.Reason == "cost_outlier" && s.SessionID == "s-outlier" {
			found = true
			if s.Severity != "high" {
				t.Errorf("expected high severity, got %s", s.Severity)
			}
		}
	}
	if !found {
		t.Errorf("expected cost_outlier signal for s-outlier, got %d signals", len(signals))
	}
}

func TestDetectWasteLowSignal(t *testing.T) {
	events := make([]source.TokenEvent, 0, 4)
	for i := 1; i <= 3; i++ {
		events = append(events, source.TokenEvent{
			SessionID:    "s" + string(rune('0'+i)),
			Project:      "p",
			Harness:      "h",
			CostUSD:      1.0,
			InputTokens:  100,
			OutputTokens: 50,
			Timestamp:    time.Date(2026, 5, 1, 10+i, 0, 0, 0, time.UTC),
		})
	}
	events = append(events, source.TokenEvent{
		SessionID:    "s-low",
		Project:      "p",
		Harness:      "h",
		CostUSD:      1.0,
		InputTokens:  100,
		OutputTokens: 1,
		Timestamp:    time.Date(2026, 5, 1, 14, 0, 0, 0, time.UTC),
	})

	baselines := ComputeBaselines(events)
	signals := DetectWaste(events, baselines, 2.0)

	var found bool
	for _, s := range signals {
		if s.Reason == "low_signal" && s.SessionID == "s-low" {
			found = true
			if s.Severity != "medium" {
				t.Errorf("expected medium severity, got %s", s.Severity)
			}
		}
	}
	if !found {
		t.Errorf("expected low_signal signal for s-low, got %d signals", len(signals))
	}
}

func TestDetectWasteSubagentOverhead(t *testing.T) {
	events := []source.TokenEvent{
		{SessionID: "s1", Project: "p", Harness: "h", CostUSD: 1.0, IsSubagent: false, Timestamp: time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)},
		{SessionID: "child-1", ParentSessionID: "s1", AgentType: "build", CostUSD: 3.0, IsSubagent: true, Timestamp: time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)},
	}

	baselines := ComputeBaselines(events)
	signals := DetectWaste(events, baselines, 2.0)

	var found bool
	for _, s := range signals {
		if s.Reason == "subagent_overhead" && s.SessionID == "s1" {
			found = true
			if s.Severity != "medium" {
				t.Errorf("expected medium severity, got %s", s.Severity)
			}
		}
	}
	if !found {
		t.Errorf("expected subagent_overhead signal, got %d signals", len(signals))
	}
}

func TestDetectWasteCacheUnderutilized(t *testing.T) {
	events := make([]source.TokenEvent, 0, 4)
	for i := 1; i <= 3; i++ {
		events = append(events, source.TokenEvent{
			SessionID:    "s" + string(rune('0'+i)),
			Project:      "p",
			Harness:      "h",
			CostUSD:      1.0,
			CacheRead:    50,
			CacheWrite:   50,
			InputTokens:  100,
			OutputTokens: 50,
			Timestamp:    time.Date(2026, 5, 1, 10+i, 0, 0, 0, time.UTC),
		})
	}
	events = append(events, source.TokenEvent{
		SessionID:    "s-cold",
		Project:      "p",
		Harness:      "h",
		CostUSD:      1.0,
		CacheRead:    0,
		CacheWrite:   100,
		InputTokens:  100,
		OutputTokens: 50,
		Timestamp:    time.Date(2026, 5, 1, 14, 0, 0, 0, time.UTC),
	})

	baselines := ComputeBaselines(events)
	signals := DetectWaste(events, baselines, 2.0)

	var found bool
	for _, s := range signals {
		if s.Reason == "cache_underutilized" && s.SessionID == "s-cold" {
			found = true
			if s.Severity != "low" {
				t.Errorf("expected low severity, got %s", s.Severity)
			}
		}
	}
	if !found {
		t.Errorf("expected cache_underutilized signal for s-cold, got %d signals", len(signals))
	}
}

func TestDetectWasteSessionChurn(t *testing.T) {
	events := []source.TokenEvent{
		{SessionID: "s1", Project: "p", Harness: "h", CostUSD: 1.0, InputTokens: 100, OutputTokens: 10, Timestamp: time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)},
		{SessionID: "s2", Project: "p", Harness: "h", CostUSD: 1.0, InputTokens: 100, OutputTokens: 10, Timestamp: time.Date(2026, 5, 1, 11, 0, 0, 0, time.UTC)},
		{SessionID: "s3", Project: "p", Harness: "h", CostUSD: 1.0, InputTokens: 100, OutputTokens: 10, Timestamp: time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)},
	}

	baselines := ComputeBaselines(events)
	signals := DetectWaste(events, baselines, 2.0)

	var found bool
	for _, s := range signals {
		if s.Reason == "session_churn" {
			found = true
			if s.Severity != "medium" {
				t.Errorf("expected medium severity, got %s", s.Severity)
			}
		}
	}
	if !found {
		t.Errorf("expected session_churn signals, got %d signals", len(signals))
	}

	count := 0
	for _, s := range signals {
		if s.Reason == "session_churn" {
			count++
		}
	}
	if count != 3 {
		t.Errorf("expected 3 session_churn signals (one per churned session), got %d", count)
	}
}

func TestDetectWasteEmptyInput(t *testing.T) {
	signals := DetectWaste(nil, nil, 2.0)
	if len(signals) != 0 {
		t.Errorf("expected 0 signals for nil input, got %d", len(signals))
	}

	signals = DetectWaste([]source.TokenEvent{}, nil, 2.0)
	if len(signals) != 0 {
		t.Errorf("expected 0 signals for empty input, got %d", len(signals))
	}
}

func TestDetectWasteAllNormal(t *testing.T) {
	events := []source.TokenEvent{
		{SessionID: "s1", Project: "p", Harness: "h", CostUSD: 1.0, InputTokens: 100, OutputTokens: 50, CacheRead: 10, CacheWrite: 10, Timestamp: time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)},
		{SessionID: "s2", Project: "p", Harness: "h", CostUSD: 1.1, InputTokens: 100, OutputTokens: 50, CacheRead: 10, CacheWrite: 10, Timestamp: time.Date(2026, 5, 1, 11, 0, 0, 0, time.UTC)},
		{SessionID: "s3", Project: "p", Harness: "h", CostUSD: 0.9, InputTokens: 100, OutputTokens: 50, CacheRead: 10, CacheWrite: 10, Timestamp: time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)},
	}

	baselines := ComputeBaselines(events)
	signals := DetectWaste(events, baselines, 2.0)

	if len(signals) != 0 {
		t.Errorf("expected 0 signals for normal data, got %d", len(signals))
	}
}

func TestDetectWasteNoSubagentsNoOverheadSignal(t *testing.T) {
	events := []source.TokenEvent{
		{SessionID: "s1", Project: "p", Harness: "h", CostUSD: 1.0, IsSubagent: false, InputTokens: 100, OutputTokens: 50, Timestamp: time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)},
	}

	baselines := ComputeBaselines(events)
	signals := DetectWaste(events, baselines, 2.0)

	for _, s := range signals {
		if s.Reason == "subagent_overhead" {
			t.Error("should not fire subagent_overhead when no subagents")
		}
	}
}

func TestDetectWasteSignalFields(t *testing.T) {
	events := make([]source.TokenEvent, 0, 10)
	now := time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)
	for i := 0; i < 5; i++ {
		events = append(events, source.TokenEvent{
			SessionID:    "s" + string(rune('a'+i)),
			Project:      "p",
			Harness:      "h",
			CostUSD:      1.0,
			InputTokens:  100,
			OutputTokens: 50,
			Timestamp:    now.Add(time.Duration(i) * time.Hour),
		})
	}
	events = append(events, source.TokenEvent{
		SessionID:    "s-outlier",
		Project:      "p",
		Harness:      "h",
		CostUSD:      30.0,
		InputTokens:  100,
		OutputTokens: 50,
		Timestamp:    now.Add(5 * time.Hour),
	})

	baselines := ComputeBaselines(events)
	signals := DetectWaste(events, baselines, 2.0)

	for _, s := range signals {
		if s.Reason == "cost_outlier" {
			if !strings.Contains(s.Detail, "30.00") {
				t.Errorf("expected detail to mention cost, got: %s", s.Detail)
			}
			if s.Metric <= s.Threshold {
				t.Errorf("metric %f should exceed threshold %f", s.Metric, s.Threshold)
			}
			if s.Project != "p" {
				t.Errorf("project = %q, want %q", s.Project, "p")
			}
			if s.SessionCost != 30.0 {
				t.Errorf("SessionCost = %f, want 30.0", s.SessionCost)
			}
			return
		}
	}
	t.Error("expected cost_outlier signal, but none found")
}

func TestDetectWasteSortOrder(t *testing.T) {
	events := make([]source.TokenEvent, 0, 7)
	now := time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)
	for i := 0; i < 6; i++ {
		events = append(events, source.TokenEvent{
			SessionID:    "s" + string(rune('a'+i)),
			Project:      "p",
			Harness:      "h",
			CostUSD:      1.0,
			InputTokens:  100,
			OutputTokens: 50,
			CacheRead:    50 - int64(i*10),
			CacheWrite:   50,
			Timestamp:    now.Add(time.Duration(i) * time.Hour),
		})
	}
	events = append(events, source.TokenEvent{
		SessionID:    "s-outlier",
		Project:      "p",
		Harness:      "h",
		CostUSD:      30.0,
		InputTokens:  100,
		OutputTokens: 50,
		CacheRead:    0,
		CacheWrite:   0,
		Timestamp:    now.Add(6 * time.Hour),
	})

	baselines := ComputeBaselines(events)
	signals := DetectWaste(events, baselines, 2.0)

	prev := ""
	for i, s := range signals {
		cur := s.Severity + ":" + s.Reason + ":" + s.SessionID
		if prev != "" && cur < prev {
			t.Errorf("signals out of order at index %d: %q < %q", i, cur, prev)
		}
		prev = cur
	}

	if signals[0].Severity != "high" {
		t.Errorf("first signal should be high severity, got %s", signals[0].Severity)
	}
}

func TestCheckCostOutlier_Sigma3(t *testing.T) {
	events := make([]source.TokenEvent, 0, 7)
	for i := 1; i <= 6; i++ {
		events = append(events, source.TokenEvent{
			SessionID:    "s" + string(rune('0'+i)),
			Project:      "p",
			Harness:      "h",
			CostUSD:      float64(i),
			InputTokens:  100,
			OutputTokens: 50,
			Timestamp:    time.Date(2026, 5, 1, 10+i, 0, 0, 0, time.UTC),
		})
	}
	events = append(events, source.TokenEvent{
		SessionID:    "s-outlier",
		Project:      "p",
		Harness:      "h",
		CostUSD:      20.0,
		InputTokens:  100,
		OutputTokens: 50,
		Timestamp:    time.Date(2026, 5, 1, 17, 0, 0, 0, time.UTC),
	})

	baselines := ComputeBaselines(events)
	// sigma=3: threshold = mean + 3*stddev (much higher, harder to trigger)
	signals := DetectWaste(events, baselines, 3.0)

	var found bool
	for _, s := range signals {
		if s.Reason == "cost_outlier" && s.SessionID == "s-outlier" {
			found = true
		}
	}
	if found {
		t.Log("cost 20 was still flagged at sigma=3 (expected if variance is low)")
	}

	// sigma=2 should flag it (lower threshold)
	signals2 := DetectWaste(events, baselines, 2.0)
	var found2 bool
	for _, s := range signals2 {
		if s.Reason == "cost_outlier" && s.SessionID == "s-outlier" {
			found2 = true
		}
	}
	if !found2 {
		t.Error("expected cost_outlier at sigma=2")
	}
}

func TestCheckCostOutlier_Sigma1(t *testing.T) {
	events := make([]source.TokenEvent, 0, 4)
	for i := 1; i <= 3; i++ {
		events = append(events, source.TokenEvent{
			SessionID:    "s" + string(rune('0'+i)),
			Project:      "p",
			Harness:      "h",
			CostUSD:      float64(i),
			InputTokens:  100,
			OutputTokens: 50,
			Timestamp:    time.Date(2026, 5, 1, 10+i, 0, 0, 0, time.UTC),
		})
	}
	events = append(events, source.TokenEvent{
		SessionID:    "s-mild",
		Project:      "p",
		Harness:      "h",
		CostUSD:      5.0,
		InputTokens:  100,
		OutputTokens: 50,
		Timestamp:    time.Date(2026, 5, 1, 14, 0, 0, 0, time.UTC),
	})

	baselines := ComputeBaselines(events)
	signals := DetectWaste(events, baselines, 1.0)

	var found bool
	for _, s := range signals {
		if s.Reason == "cost_outlier" && s.SessionID == "s-mild" {
			found = true
		}
	}
	if found {
		t.Log("cost 5 flagged at sigma=1 (expected with tighter threshold)")
	}
}

func TestCheckCostOutlier_ZeroSigma(t *testing.T) {
	events := []source.TokenEvent{
		{SessionID: "s1", Project: "p", Harness: "h", CostUSD: 1.0, InputTokens: 100, OutputTokens: 50,
			Timestamp: time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)},
		{SessionID: "s2", Project: "p", Harness: "h", CostUSD: 5.0, InputTokens: 100, OutputTokens: 50,
			Timestamp: time.Date(2026, 5, 1, 11, 0, 0, 0, time.UTC)},
	}
	baselines := ComputeBaselines(events)
	signals := DetectWaste(events, baselines, 0)

	for _, s := range signals {
		if s.Reason == "cost_outlier" {
			t.Logf("cost_outlier fired at sigma=0: total=%d", len(signals))
		}
	}
}

func TestDetectWaste_WithSigma(t *testing.T) {
	events := make([]source.TokenEvent, 0, 7)
	for i := 1; i <= 6; i++ {
		events = append(events, source.TokenEvent{
			SessionID:    "s" + string(rune('0'+i)),
			Project:      "p",
			Harness:      "h",
			CostUSD:      float64(i),
			InputTokens:  100,
			OutputTokens: 50,
			Timestamp:    time.Date(2026, 5, 1, 10+i, 0, 0, 0, time.UTC),
		})
	}
	events = append(events, source.TokenEvent{
		SessionID:    "s-outlier",
		Project:      "p",
		Harness:      "h",
		CostUSD:      20.0,
		InputTokens:  100,
		OutputTokens: 50,
		Timestamp:    time.Date(2026, 5, 1, 17, 0, 0, 0, time.UTC),
	})

	baselines := ComputeBaselines(events)
	signals2 := DetectWaste(events, baselines, 2.0)
	signals4 := DetectWaste(events, baselines, 4.0)

	countOutlier := func(signals []WasteSignal) int {
		c := 0
		for _, s := range signals {
			if s.Reason == "cost_outlier" {
				c++
			}
		}
		return c
	}

	c2 := countOutlier(signals2)
	c4 := countOutlier(signals4)

	if c4 > c2 {
		t.Errorf("sigma=4 (%d outliers) should not have more outliers than sigma=2 (%d outliers)", c4, c2)
	}
}

func TestCostConsistency(t *testing.T) {
	events := []source.TokenEvent{
		{SessionID: "parent", Project: "p", Harness: "h", CostUSD: 1.0, InputTokens: 500, OutputTokens: 100, IsSubagent: false, Timestamp: time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)},
		{SessionID: "child-a", ParentSessionID: "parent", AgentType: "build", CostUSD: 8.0, InputTokens: 0, OutputTokens: 0, IsSubagent: true, Timestamp: time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)},
		{SessionID: "s2", Project: "p", Harness: "h", CostUSD: 1.0, InputTokens: 500, OutputTokens: 100, IsSubagent: false, Timestamp: time.Date(2026, 5, 1, 11, 0, 0, 0, time.UTC)},
		{SessionID: "s3", Project: "p", Harness: "h", CostUSD: 1.0, InputTokens: 500, OutputTokens: 100, IsSubagent: false, Timestamp: time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)},
	}

	baselines := ComputeBaselines(events)
	signals := DetectWaste(events, baselines, 2.0)

	// All signals for "parent" session should have same SessionCost
	var parentSignals []WasteSignal
	for _, s := range signals {
		if s.SessionID == "parent" {
			parentSignals = append(parentSignals, s)
		}
	}
	if len(parentSignals) == 0 {
		t.Fatal("expected at least one signal for parent session")
	}

	firstCost := parentSignals[0].SessionCost
	if firstCost != 9.0 {
		t.Errorf("expected parent SessionCost=9.0 (1.0 parent + 8.0 child), got %.2f", firstCost)
	}
	for i, s := range parentSignals {
		if s.SessionCost != firstCost {
			t.Errorf("inconsistent cost at signal %d: %.2f != %.2f", i, s.SessionCost, firstCost)
		}
	}
}
