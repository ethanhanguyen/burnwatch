package analyze

import (
	"strings"
	"testing"
	"time"

	"github.com/yourname/burnwatch/source"
)

func TestDetectWasteCostOutlier(t *testing.T) {
	events := make([]source.TokenEvent, 0, 6)
	for i := 1; i <= 5; i++ {
		events = append(events, source.TokenEvent{
			SessionID:  "s" + string(rune('0'+i)),
			Project:    "p",
			Harness:    "h",
			CostUSD:    float64(i),
			InputTokens: 100,
			OutputTokens: 50,
			Timestamp:  time.Date(2026, 5, 1, 10+i, 0, 0, 0, time.UTC),
		})
	}
	events = append(events, source.TokenEvent{
		SessionID:  "s-outlier",
		Project:    "p",
		Harness:    "h",
		CostUSD:    50.0,
		InputTokens: 100,
		OutputTokens: 50,
		Timestamp:  time.Date(2026, 5, 1, 16, 0, 0, 0, time.UTC),
	})

	baselines := ComputeBaselines(events)
	signals := DetectWaste(events, baselines)

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
			SessionID:  "s" + string(rune('0'+i)),
			Project:    "p",
			Harness:    "h",
			CostUSD:    1.0,
			InputTokens: 100,
			OutputTokens: 50,
			Timestamp:  time.Date(2026, 5, 1, 10+i, 0, 0, 0, time.UTC),
		})
	}
	events = append(events, source.TokenEvent{
		SessionID:   "s-low",
		Project:     "p",
		Harness:     "h",
		CostUSD:     1.0,
		InputTokens: 100,
		OutputTokens: 1,
		Timestamp:   time.Date(2026, 5, 1, 14, 0, 0, 0, time.UTC),
	})

	baselines := ComputeBaselines(events)
	signals := DetectWaste(events, baselines)

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
	signals := DetectWaste(events, baselines)

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
			SessionID:  "s" + string(rune('0'+i)),
			Project:    "p",
			Harness:    "h",
			CostUSD:    1.0,
			CacheRead:  50,
			CacheWrite: 50,
			InputTokens: 100,
			OutputTokens: 50,
			Timestamp:  time.Date(2026, 5, 1, 10+i, 0, 0, 0, time.UTC),
		})
	}
	events = append(events, source.TokenEvent{
		SessionID:   "s-cold",
		Project:     "p",
		Harness:     "h",
		CostUSD:     1.0,
		CacheRead:   0,
		CacheWrite:  100,
		InputTokens: 100,
		OutputTokens: 50,
		Timestamp:   time.Date(2026, 5, 1, 14, 0, 0, 0, time.UTC),
	})

	baselines := ComputeBaselines(events)
	signals := DetectWaste(events, baselines)

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
	signals := DetectWaste(events, baselines)

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
	signals := DetectWaste(nil, nil)
	if len(signals) != 0 {
		t.Errorf("expected 0 signals for nil input, got %d", len(signals))
	}

	signals = DetectWaste([]source.TokenEvent{}, nil)
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
	signals := DetectWaste(events, baselines)

	if len(signals) != 0 {
		t.Errorf("expected 0 signals for normal data, got %d", len(signals))
	}
}

func TestDetectWasteNoSubagentsNoOverheadSignal(t *testing.T) {
	events := []source.TokenEvent{
		{SessionID: "s1", Project: "p", Harness: "h", CostUSD: 1.0, IsSubagent: false, InputTokens: 100, OutputTokens: 50, Timestamp: time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)},
	}

	baselines := ComputeBaselines(events)
	signals := DetectWaste(events, baselines)

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
			SessionID:   "s" + string(rune('a'+i)),
			Project:     "p",
			Harness:     "h",
			CostUSD:     1.0,
			InputTokens: 100,
			OutputTokens: 50,
			Timestamp:   now.Add(time.Duration(i) * time.Hour),
		})
	}
	events = append(events, source.TokenEvent{
		SessionID:   "s-outlier",
		Project:     "p",
		Harness:     "h",
		CostUSD:     30.0,
		InputTokens: 100,
		OutputTokens: 50,
		Timestamp:   now.Add(5 * time.Hour),
	})

	baselines := ComputeBaselines(events)
	signals := DetectWaste(events, baselines)

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
			SessionID:   "s" + string(rune('a'+i)),
			Project:     "p",
			Harness:     "h",
			CostUSD:     1.0,
			InputTokens: 100,
			OutputTokens: 50,
			CacheRead:   50 - int64(i*10),
			CacheWrite:  50,
			Timestamp:   now.Add(time.Duration(i) * time.Hour),
		})
	}
	events = append(events, source.TokenEvent{
		SessionID:   "s-outlier",
		Project:     "p",
		Harness:     "h",
		CostUSD:     30.0,
		InputTokens: 100,
		OutputTokens: 50,
		CacheRead:   0,
		CacheWrite:  0,
		Timestamp:   now.Add(6 * time.Hour),
	})

	baselines := ComputeBaselines(events)
	signals := DetectWaste(events, baselines)

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
