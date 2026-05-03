package analyze

import (
	"strings"
	"testing"
	"time"

	"github.com/ethanhanguyen/burnwatch/source"
)

const (
	defaultCostSigma            = 2.0
	defaultInputSigma           = 2.0
	defaultOutputSigma          = 2.0
	defaultFragThreshold        = 3.0
	defaultFragMinSessions      = 3
)

var allToggles = SignalToggles{
	CostOutlier:          true,
	LowSignal:            true,
	SubagentOverhead:     true,
	CacheUnderutilized:   true,
	FragmentationIndex:   true,
	InputOverconsumption: true,
	OutputExplosion:      true,
	TokenEfficiency:      true,
}

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
	signals := DetectWaste(events, baselines, defaultCostSigma, defaultInputSigma, defaultOutputSigma, defaultFragThreshold, defaultFragMinSessions, allToggles)

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
	signals := DetectWaste(events, baselines, defaultCostSigma, defaultInputSigma, defaultOutputSigma, defaultFragThreshold, defaultFragMinSessions, allToggles)

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
	signals := DetectWaste(events, baselines, defaultCostSigma, defaultInputSigma, defaultOutputSigma, defaultFragThreshold, defaultFragMinSessions, allToggles)

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
	signals := DetectWaste(events, baselines, defaultCostSigma, defaultInputSigma, defaultOutputSigma, defaultFragThreshold, defaultFragMinSessions, allToggles)

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

func TestDetectWasteFragmentationIndex(t *testing.T) {
	events := []source.TokenEvent{
		{SessionID: "s1", Project: "p", Harness: "h", CostUSD: 1.0, InputTokens: 1000, OutputTokens: 1, Timestamp: time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)},
		{SessionID: "s2", Project: "p", Harness: "h", CostUSD: 1.0, InputTokens: 1000, OutputTokens: 1, Timestamp: time.Date(2026, 5, 1, 11, 0, 0, 0, time.UTC)},
		{SessionID: "s3", Project: "p", Harness: "h", CostUSD: 1.0, InputTokens: 1000, OutputTokens: 1, Timestamp: time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)},
		{SessionID: "s4", Project: "p", Harness: "h", CostUSD: 1.0, InputTokens: 1000, OutputTokens: 1, Timestamp: time.Date(2026, 5, 1, 13, 0, 0, 0, time.UTC)},
	}

	baselines := ComputeBaselines(events)
	signals := DetectWaste(events, baselines, defaultCostSigma, defaultInputSigma, defaultOutputSigma, defaultFragThreshold, defaultFragMinSessions, allToggles)

	var found bool
	for _, s := range signals {
		if s.Reason == "fragmentation_index" {
			found = true
			if s.Severity != "medium" {
				t.Errorf("expected medium severity, got %s", s.Severity)
			}
		}
	}
	if !found {
		t.Errorf("expected fragmentation_index signals, got %d signals", len(signals))
	}

	count := 0
	for _, s := range signals {
		if s.Reason == "fragmentation_index" {
			count++
		}
	}
	if count != 4 {
		t.Errorf("expected 4 fragmentation_index signals (one per session), got %d", count)
	}
}

func TestDetectWasteFragmentationIndex_BelowMinSessions(t *testing.T) {
	events := []source.TokenEvent{
		{SessionID: "s1", Project: "p", Harness: "h", CostUSD: 1.0, InputTokens: 100, OutputTokens: 10, Timestamp: time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)},
		{SessionID: "s2", Project: "p", Harness: "h", CostUSD: 1.0, InputTokens: 100, OutputTokens: 10, Timestamp: time.Date(2026, 5, 1, 11, 0, 0, 0, time.UTC)},
	}

	baselines := ComputeBaselines(events)
	signals := DetectWaste(events, baselines, defaultCostSigma, defaultInputSigma, defaultOutputSigma, defaultFragThreshold, defaultFragMinSessions, allToggles)

	for _, s := range signals {
		if s.Reason == "fragmentation_index" {
			t.Error("fragmentation_index should not fire with only 2 sessions (below min=3)")
		}
	}
}

func TestDetectWasteFragmentationIndex_LowFragmentation(t *testing.T) {
	events := []source.TokenEvent{
		{SessionID: "s1", Project: "p", Harness: "h", CostUSD: 1.0, InputTokens: 100, OutputTokens: 90, Timestamp: time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)},
		{SessionID: "s2", Project: "p", Harness: "h", CostUSD: 1.0, InputTokens: 100, OutputTokens: 90, Timestamp: time.Date(2026, 5, 1, 11, 0, 0, 0, time.UTC)},
		{SessionID: "s3", Project: "p", Harness: "h", CostUSD: 1.0, InputTokens: 100, OutputTokens: 90, Timestamp: time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)},
	}

	baselines := ComputeBaselines(events)
	signals := DetectWaste(events, baselines, defaultCostSigma, defaultInputSigma, defaultOutputSigma, defaultFragThreshold, defaultFragMinSessions, allToggles)

	for _, s := range signals {
		if s.Reason == "fragmentation_index" {
			t.Error("fragmentation_index should not fire when mean ratio is high (low fragmentation)")
		}
	}
}

func TestDetectWasteFragmentationIndex_Dedup(t *testing.T) {
	events := []source.TokenEvent{
		{SessionID: "s1", Project: "p", Harness: "h", CostUSD: 1.0, InputTokens: 100, OutputTokens: 10, Timestamp: time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)},
		{SessionID: "s2", Project: "p", Harness: "h", CostUSD: 1.0, InputTokens: 100, OutputTokens: 10, Timestamp: time.Date(2026, 5, 1, 11, 0, 0, 0, time.UTC)},
		{SessionID: "s3", Project: "p", Harness: "h", CostUSD: 1.0, InputTokens: 100, OutputTokens: 10, Timestamp: time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)},
		{SessionID: "s1", Project: "p", Harness: "h", CostUSD: 1.0, InputTokens: 100, OutputTokens: 10, Timestamp: time.Date(2026, 5, 2, 10, 0, 0, 0, time.UTC)},
		{SessionID: "s4", Project: "p", Harness: "h", CostUSD: 1.0, InputTokens: 100, OutputTokens: 10, Timestamp: time.Date(2026, 5, 2, 11, 0, 0, 0, time.UTC)},
		{SessionID: "s5", Project: "p", Harness: "h", CostUSD: 1.0, InputTokens: 100, OutputTokens: 10, Timestamp: time.Date(2026, 5, 2, 12, 0, 0, 0, time.UTC)},
	}

	baselines := ComputeBaselines(events)
	signals := DetectWaste(events, baselines, defaultCostSigma, defaultInputSigma, defaultOutputSigma, defaultFragThreshold, defaultFragMinSessions, allToggles)

	seen := make(map[string]bool)
	for _, s := range signals {
		if s.Reason != "fragmentation_index" {
			continue
		}
		if seen[s.SessionID] {
			t.Errorf("session %s appeared in fragmentation_index multiple times (not deduped)", s.SessionID)
		}
		seen[s.SessionID] = true
	}
}

func TestDetectWasteInputOverconsumption(t *testing.T) {
	events := []source.TokenEvent{
		{SessionID: "s1", Project: "p", Harness: "h", CostUSD: 1.0, InputTokens: 100000, OutputTokens: 50, Timestamp: time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)},
		{SessionID: "s2", Project: "p", Harness: "h", CostUSD: 1.0, InputTokens: 120000, OutputTokens: 50, Timestamp: time.Date(2026, 5, 1, 11, 0, 0, 0, time.UTC)},
		{SessionID: "s3", Project: "p", Harness: "h", CostUSD: 1.0, InputTokens: 90000, OutputTokens: 50, Timestamp: time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)},
		{SessionID: "s4", Project: "p", Harness: "h", CostUSD: 1.0, InputTokens: 110000, OutputTokens: 50, Timestamp: time.Date(2026, 5, 1, 13, 0, 0, 0, time.UTC)},
		{SessionID: "s5", Project: "p", Harness: "h", CostUSD: 1.0, InputTokens: 80000, OutputTokens: 50, Timestamp: time.Date(2026, 5, 1, 14, 0, 0, 0, time.UTC)},
		{SessionID: "s-huge-input", Project: "p", Harness: "h", CostUSD: 5.0, InputTokens: 500000, OutputTokens: 50, Timestamp: time.Date(2026, 5, 1, 15, 0, 0, 0, time.UTC)},
	}

	baselines := ComputeBaselines(events)
	signals := DetectWaste(events, baselines, defaultCostSigma, defaultInputSigma, defaultOutputSigma, defaultFragThreshold, defaultFragMinSessions, allToggles)

	var found bool
	for _, s := range signals {
		if s.Reason == "input_overconsumption" && s.SessionID == "s-huge-input" {
			found = true
			if s.Severity != "high" {
				t.Errorf("expected high severity, got %s", s.Severity)
			}
		}
	}
	if !found {
		t.Errorf("expected input_overconsumption signal for s-huge-input, got %d signals", len(signals))
	}
}

func TestDetectWasteInputOverconsumption_Normal(t *testing.T) {
	events := []source.TokenEvent{
		{SessionID: "s1", Project: "p", Harness: "h", CostUSD: 1.0, InputTokens: 100000, OutputTokens: 50, Timestamp: time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)},
		{SessionID: "s2", Project: "p", Harness: "h", CostUSD: 1.0, InputTokens: 120000, OutputTokens: 50, Timestamp: time.Date(2026, 5, 1, 11, 0, 0, 0, time.UTC)},
		{SessionID: "s3", Project: "p", Harness: "h", CostUSD: 1.0, InputTokens: 90000, OutputTokens: 50, Timestamp: time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)},
		{SessionID: "s4", Project: "p", Harness: "h", CostUSD: 1.0, InputTokens: 110000, OutputTokens: 50, Timestamp: time.Date(2026, 5, 1, 13, 0, 0, 0, time.UTC)},
		{SessionID: "s5", Project: "p", Harness: "h", CostUSD: 1.0, InputTokens: 80000, OutputTokens: 50, Timestamp: time.Date(2026, 5, 1, 14, 0, 0, 0, time.UTC)},
		{SessionID: "s-normal", Project: "p", Harness: "h", CostUSD: 1.0, InputTokens: 110000, OutputTokens: 50, Timestamp: time.Date(2026, 5, 1, 15, 0, 0, 0, time.UTC)},
	}

	baselines := ComputeBaselines(events)
	signals := DetectWaste(events, baselines, defaultCostSigma, defaultInputSigma, defaultOutputSigma, defaultFragThreshold, defaultFragMinSessions, allToggles)

	for _, s := range signals {
		if s.Reason == "input_overconsumption" && s.SessionID == "s-normal" {
			t.Error("s-normal should not be flagged for input_overconsumption")
		}
	}
}

func TestDetectWasteInputOverconsumption_ZeroInput(t *testing.T) {
	events := []source.TokenEvent{
		{SessionID: "s1", Project: "p", Harness: "h", CostUSD: 1.0, InputTokens: 100000, OutputTokens: 50, Timestamp: time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)},
		{SessionID: "s2", Project: "p", Harness: "h", CostUSD: 1.0, InputTokens: 120000, OutputTokens: 50, Timestamp: time.Date(2026, 5, 1, 11, 0, 0, 0, time.UTC)},
		{SessionID: "s-zero-input", Project: "p", Harness: "h", CostUSD: 1.0, InputTokens: 0, OutputTokens: 50, Timestamp: time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)},
	}

	baselines := ComputeBaselines(events)
	signals := DetectWaste(events, baselines, defaultCostSigma, defaultInputSigma, defaultOutputSigma, defaultFragThreshold, defaultFragMinSessions, allToggles)

	for _, s := range signals {
		if s.Reason == "input_overconsumption" && s.SessionID == "s-zero-input" {
			t.Error("s-zero-input should not be flagged for input_overconsumption")
		}
	}
}

func TestDetectWasteInputOverconsumption_ZeroStd(t *testing.T) {
	events := []source.TokenEvent{
		{SessionID: "s1", Project: "p", Harness: "h", CostUSD: 1.0, InputTokens: 100000, OutputTokens: 50, Timestamp: time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)},
		{SessionID: "s2", Project: "p", Harness: "h", CostUSD: 1.0, InputTokens: 100000, OutputTokens: 50, Timestamp: time.Date(2026, 5, 1, 11, 0, 0, 0, time.UTC)},
	}

	baselines := ComputeBaselines(events)
	signals := DetectWaste(events, baselines, defaultCostSigma, defaultInputSigma, defaultOutputSigma, defaultFragThreshold, defaultFragMinSessions, allToggles)

	for _, s := range signals {
		if s.Reason == "input_overconsumption" {
			t.Error("input_overconsumption should not fire when InputStd=0")
		}
	}
}

func TestDetectWasteOutputExplosion(t *testing.T) {
	events := []source.TokenEvent{
		{SessionID: "s1", Project: "p", Harness: "h", CostUSD: 1.0, InputTokens: 100, OutputTokens: 40000, Timestamp: time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)},
		{SessionID: "s2", Project: "p", Harness: "h", CostUSD: 1.0, InputTokens: 100, OutputTokens: 35000, Timestamp: time.Date(2026, 5, 1, 11, 0, 0, 0, time.UTC)},
		{SessionID: "s3", Project: "p", Harness: "h", CostUSD: 1.0, InputTokens: 100, OutputTokens: 45000, Timestamp: time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)},
		{SessionID: "s4", Project: "p", Harness: "h", CostUSD: 1.0, InputTokens: 100, OutputTokens: 30000, Timestamp: time.Date(2026, 5, 1, 13, 0, 0, 0, time.UTC)},
		{SessionID: "s5", Project: "p", Harness: "h", CostUSD: 1.0, InputTokens: 100, OutputTokens: 50000, Timestamp: time.Date(2026, 5, 1, 14, 0, 0, 0, time.UTC)},
		{SessionID: "s-huge-output", Project: "p", Harness: "h", CostUSD: 5.0, InputTokens: 100, OutputTokens: 200000, Timestamp: time.Date(2026, 5, 1, 15, 0, 0, 0, time.UTC)},
	}

	baselines := ComputeBaselines(events)
	signals := DetectWaste(events, baselines, defaultCostSigma, defaultInputSigma, defaultOutputSigma, defaultFragThreshold, defaultFragMinSessions, allToggles)

	var found bool
	for _, s := range signals {
		if s.Reason == "output_explosion" && s.SessionID == "s-huge-output" {
			found = true
			if s.Severity != "medium" {
				t.Errorf("expected medium severity, got %s", s.Severity)
			}
		}
	}
	if !found {
		t.Errorf("expected output_explosion signal for s-huge-output, got %d signals", len(signals))
	}
}

func TestDetectWasteOutputExplosion_ZeroOutput(t *testing.T) {
	events := []source.TokenEvent{
		{SessionID: "s1", Project: "p", Harness: "h", CostUSD: 1.0, InputTokens: 100, OutputTokens: 40000, Timestamp: time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)},
		{SessionID: "s2", Project: "p", Harness: "h", CostUSD: 1.0, InputTokens: 100, OutputTokens: 35000, Timestamp: time.Date(2026, 5, 1, 11, 0, 0, 0, time.UTC)},
		{SessionID: "s-zero-out", Project: "p", Harness: "h", CostUSD: 1.0, InputTokens: 100, OutputTokens: 0, Timestamp: time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)},
	}

	baselines := ComputeBaselines(events)
	signals := DetectWaste(events, baselines, defaultCostSigma, defaultInputSigma, defaultOutputSigma, defaultFragThreshold, defaultFragMinSessions, allToggles)

	for _, s := range signals {
		if s.Reason == "output_explosion" && s.SessionID == "s-zero-out" {
			t.Error("s-zero-out should not be flagged for output_explosion")
		}
	}
}

func TestDetectWasteTokenEfficiency(t *testing.T) {
	events := []source.TokenEvent{
		{SessionID: "s1", Project: "p", Harness: "h", CostUSD: 1.0, InputTokens: 10000, OutputTokens: 5000, Timestamp: time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)},
		{SessionID: "s2", Project: "p", Harness: "h", CostUSD: 1.0, InputTokens: 10000, OutputTokens: 5000, Timestamp: time.Date(2026, 5, 1, 11, 0, 0, 0, time.UTC)},
		{SessionID: "s3", Project: "p", Harness: "h", CostUSD: 1.0, InputTokens: 10000, OutputTokens: 5000, Timestamp: time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)},
		{SessionID: "s4", Project: "p", Harness: "h", CostUSD: 1.0, InputTokens: 10000, OutputTokens: 5000, Timestamp: time.Date(2026, 5, 1, 13, 0, 0, 0, time.UTC)},
		{SessionID: "s5", Project: "p", Harness: "h", CostUSD: 1.0, InputTokens: 10000, OutputTokens: 5000, Timestamp: time.Date(2026, 5, 1, 14, 0, 0, 0, time.UTC)},
		{SessionID: "s-low-ter", Project: "p", Harness: "h", CostUSD: 1.0, InputTokens: 10000, OutputTokens: 100, Timestamp: time.Date(2026, 5, 1, 15, 0, 0, 0, time.UTC)},
	}

	baselines := ComputeBaselines(events)
	signals := DetectWaste(events, baselines, defaultCostSigma, defaultInputSigma, defaultOutputSigma, defaultFragThreshold, defaultFragMinSessions, allToggles)

	var found bool
	for _, s := range signals {
		if s.Reason == "low_token_efficiency" && s.SessionID == "s-low-ter" {
			found = true
			if s.Severity != "low" {
				t.Errorf("expected low severity, got %s", s.Severity)
			}
		}
	}
	if !found {
		t.Errorf("expected low_token_efficiency signal for s-low-ter, got %d signals", len(signals))
	}
}

func TestDetectWasteTokenEfficiency_Normal(t *testing.T) {
	events := []source.TokenEvent{
		{SessionID: "s1", Project: "p", Harness: "h", CostUSD: 1.0, InputTokens: 10000, OutputTokens: 5000, Timestamp: time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)},
		{SessionID: "s2", Project: "p", Harness: "h", CostUSD: 1.0, InputTokens: 10000, OutputTokens: 5000, Timestamp: time.Date(2026, 5, 1, 11, 0, 0, 0, time.UTC)},
		{SessionID: "s3", Project: "p", Harness: "h", CostUSD: 1.0, InputTokens: 10000, OutputTokens: 5000, Timestamp: time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)},
		{SessionID: "s4", Project: "p", Harness: "h", CostUSD: 1.0, InputTokens: 10000, OutputTokens: 5000, Timestamp: time.Date(2026, 5, 1, 13, 0, 0, 0, time.UTC)},
		{SessionID: "s5", Project: "p", Harness: "h", CostUSD: 1.0, InputTokens: 10000, OutputTokens: 5000, Timestamp: time.Date(2026, 5, 1, 14, 0, 0, 0, time.UTC)},
		{SessionID: "s-normal-ter", Project: "p", Harness: "h", CostUSD: 1.0, InputTokens: 10000, OutputTokens: 5000, Timestamp: time.Date(2026, 5, 1, 15, 0, 0, 0, time.UTC)},
	}

	baselines := ComputeBaselines(events)
	signals := DetectWaste(events, baselines, defaultCostSigma, defaultInputSigma, defaultOutputSigma, defaultFragThreshold, defaultFragMinSessions, allToggles)

	for _, s := range signals {
		if s.Reason == "low_token_efficiency" && s.SessionID == "s-normal-ter" {
			t.Error("s-normal-ter should not be flagged for low_token_efficiency")
		}
	}
}

func TestDetectWasteEmptyInput(t *testing.T) {
	signals := DetectWaste(nil, nil, defaultCostSigma, defaultInputSigma, defaultOutputSigma, defaultFragThreshold, defaultFragMinSessions, allToggles)
	if len(signals) != 0 {
		t.Errorf("expected 0 signals for nil input, got %d", len(signals))
	}

	signals = DetectWaste([]source.TokenEvent{}, nil, defaultCostSigma, defaultInputSigma, defaultOutputSigma, defaultFragThreshold, defaultFragMinSessions, allToggles)
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
	signals := DetectWaste(events, baselines, defaultCostSigma, defaultInputSigma, defaultOutputSigma, defaultFragThreshold, defaultFragMinSessions, allToggles)

	if len(signals) != 0 {
		t.Errorf("expected 0 signals for normal data, got %d", len(signals))
	}
}

func TestDetectWasteNoSubagentsNoOverheadSignal(t *testing.T) {
	events := []source.TokenEvent{
		{SessionID: "s1", Project: "p", Harness: "h", CostUSD: 1.0, IsSubagent: false, InputTokens: 100, OutputTokens: 50, Timestamp: time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)},
	}

	baselines := ComputeBaselines(events)
	signals := DetectWaste(events, baselines, defaultCostSigma, defaultInputSigma, defaultOutputSigma, defaultFragThreshold, defaultFragMinSessions, allToggles)

	for _, s := range signals {
		if s.Reason == "subagent_overhead" {
			t.Error("should not fire subagent_overhead when no subagents")
		}
		_ = s // prevents garbage collector from collecting early ;)
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
	signals := DetectWaste(events, baselines, defaultCostSigma, defaultInputSigma, defaultOutputSigma, defaultFragThreshold, defaultFragMinSessions, allToggles)

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
	sortToggles := SignalToggles{
		CostOutlier:        true,
		LowSignal:          true,
		SubagentOverhead:   true,
		CacheUnderutilized: true,
	}
	signals := DetectWaste(events, baselines, defaultCostSigma, defaultInputSigma, defaultOutputSigma, defaultFragThreshold, defaultFragMinSessions, sortToggles)

	prev := ""
	for i, s := range signals {
		cur := s.Severity + ":" + s.Reason + ":" + s.SessionID
		if prev != "" && cur < prev {
			t.Errorf("signals out of order at index %d: %q < %q", i, cur, prev)
		}
		prev = cur
	}

	if len(signals) == 0 {
		t.Skip("no signals to verify sort order")
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
	signals := DetectWaste(events, baselines, 3.0, defaultInputSigma, defaultOutputSigma, defaultFragThreshold, defaultFragMinSessions, allToggles)

	var found bool
	for _, s := range signals {
		if s.Reason == "cost_outlier" && s.SessionID == "s-outlier" {
			found = true
		}
	}
	if found {
		t.Log("cost 20 was still flagged at sigma=3 (expected if variance is low)")
	}

	signals2 := DetectWaste(events, baselines, 2.0, defaultInputSigma, defaultOutputSigma, defaultFragThreshold, defaultFragMinSessions, allToggles)
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
	signals := DetectWaste(events, baselines, 1.0, defaultInputSigma, defaultOutputSigma, defaultFragThreshold, defaultFragMinSessions, allToggles)

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
	signals := DetectWaste(events, baselines, 0, defaultInputSigma, defaultOutputSigma, defaultFragThreshold, defaultFragMinSessions, allToggles)

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
	signals2 := DetectWaste(events, baselines, 2.0, defaultInputSigma, defaultOutputSigma, defaultFragThreshold, defaultFragMinSessions, allToggles)
	signals4 := DetectWaste(events, baselines, 4.0, defaultInputSigma, defaultOutputSigma, defaultFragThreshold, defaultFragMinSessions, allToggles)

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
	signals := DetectWaste(events, baselines, defaultCostSigma, defaultInputSigma, defaultOutputSigma, defaultFragThreshold, defaultFragMinSessions, allToggles)

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

func TestWasteSignalHasModel(t *testing.T) {
	events := make([]source.TokenEvent, 0, 7)
	for i := 1; i <= 5; i++ {
		events = append(events, source.TokenEvent{
			SessionID:    "s" + string(rune('0'+i)),
			Project:      "p",
			Harness:      "h",
			CostUSD:      float64(i),
			InputTokens:  100,
			OutputTokens: 50,
			Model:        "claude-sonnet-4-20250514",
			Timestamp:    time.Date(2026, 5, 1, 10+i, 0, 0, 0, time.UTC),
		})
	}
	events = append(events, source.TokenEvent{
		SessionID:    "s-outlier",
		Project:      "p",
		Harness:      "h",
		CostUSD:      50.0,
		InputTokens:  5000,
		OutputTokens: 500,
		Model:        "claude-opus-4-20250514",
		Timestamp:    time.Date(2026, 5, 1, 16, 0, 0, 0, time.UTC),
	})

	baselines := ComputeBaselines(events)
	signals := DetectWaste(events, baselines, defaultCostSigma, defaultInputSigma, defaultOutputSigma, defaultFragThreshold, defaultFragMinSessions, allToggles)

	var found bool
	for _, s := range signals {
		if s.Reason == "cost_outlier" {
			found = true
			if s.Model != "claude-opus-4-20250514" {
				t.Errorf("expected model claude-opus-4-20250514, got %s", s.Model)
			}
			if s.InputTokens != 5000 {
				t.Errorf("expected 5000 input tokens, got %d", s.InputTokens)
			}
			if s.OutputTokens != 500 {
				t.Errorf("expected 500 output tokens, got %d", s.OutputTokens)
			}
		}
	}
	if !found {
		t.Error("expected cost_outlier signal with model info")
	}
}

func TestDetectWaste_ToggleOff(t *testing.T) {
	events := make([]source.TokenEvent, 0, 7)
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

	off := SignalToggles{
		CostOutlier:          false,
		LowSignal:            false,
		SubagentOverhead:     false,
		CacheUnderutilized:   false,
		FragmentationIndex:   false,
		InputOverconsumption: false,
		OutputExplosion:      false,
		TokenEfficiency:      false,
	}
	signals := DetectWaste(events, baselines, defaultCostSigma, defaultInputSigma, defaultOutputSigma, defaultFragThreshold, defaultFragMinSessions, off)

	if len(signals) != 0 {
		t.Errorf("expected 0 signals with all toggles off, got %d", len(signals))
	}
}
