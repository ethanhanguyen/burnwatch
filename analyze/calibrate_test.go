package analyze

import (
	"encoding/json"
	"math"
	"testing"
	"time"

	"github.com/ethanhanguyen/burnwatch/source"
)

func eventsForCalibration() []source.TokenEvent {
	t := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	ref := time.Date(2026, 4, 10, 0, 0, 0, 0, time.UTC)

	return []source.TokenEvent{
		{SessionID: "s1", Project: "p1", Harness: "claude-code", InputTokens: 100, OutputTokens: 50, CacheRead: 0, CacheWrite: 0, CostUSD: 0.10, Timestamp: ref},
		{SessionID: "s1", Project: "p1", Harness: "claude-code", InputTokens: 200, OutputTokens: 100, CacheRead: 0, CacheWrite: 0, CostUSD: 0.20, Timestamp: ref},
		{SessionID: "s2", Project: "p1", Harness: "claude-code", InputTokens: 300, OutputTokens: 200, CacheRead: 0, CacheWrite: 0, CostUSD: 0.50, Timestamp: ref},
		{SessionID: "s3", Project: "p1", Harness: "claude-code", InputTokens: 500, OutputTokens: 100, CacheRead: 0, CacheWrite: 0, CostUSD: 0.30, Timestamp: ref},
		{SessionID: "s4", Project: "p2", Harness: "opencode", InputTokens: 1000, OutputTokens: 500, CacheRead: 0, CacheWrite: 0, CostUSD: 1.00, Timestamp: ref},
		{SessionID: "s5", Project: "p2", Harness: "opencode", InputTokens: 2000, OutputTokens: 2000, CacheRead: 0, CacheWrite: 0, CostUSD: 2.00, Timestamp: t},
	}
}

func TestComputeCalibrationDistStats(t *testing.T) {
	report := ComputeCalibration(eventsForCalibration())

	if report.TotalSessions != 5 {
		t.Errorf("TotalSessions = %d, want 5", report.TotalSessions)
	}
	if report.ProjectCount != 2 {
		t.Errorf("ProjectCount = %d, want 2", report.ProjectCount)
	}
	if report.DateRangeStart != "2026-04-10" {
		t.Errorf("DateRangeStart = %q, want %q", report.DateRangeStart, "2026-04-10")
	}
	if report.DateRangeEnd != "2026-05-01" {
		t.Errorf("DateRangeEnd = %q, want %q", report.DateRangeEnd, "2026-05-01")
	}

	t.Run("session cost", func(t *testing.T) {
		s := report.SessionCost
		if s.Count != 5 {
			t.Errorf("Count = %d, want 5", s.Count)
		}
		expectedMean := (0.30 + 0.50 + 0.30 + 1.00 + 2.00) / 5.0
		if math.Abs(s.Mean-expectedMean) > 0.001 {
			t.Errorf("Mean = %f, want %f", s.Mean, expectedMean)
		}
		if s.Min != 0.30 {
			t.Errorf("Min = %f, want 0.30", s.Min)
		}
		if math.Abs(s.Max-2.0) > 0.001 {
			t.Errorf("Max = %f, want 2.0", s.Max)
		}
		if s.P50 != 0.50 {
			t.Errorf("P50 = %f, want 0.50", s.P50)
		}
	})

	t.Run("input tokens", func(t *testing.T) {
		s := report.InputTokens
		if s.Count != 5 {
			t.Errorf("Count = %d, want 5", s.Count)
		}
		if math.Abs(s.Min-300.0) > 0.001 {
			t.Errorf("Min = %f, want 300", s.Min)
		}
		if math.Abs(s.Max-2000.0) > 0.001 {
			t.Errorf("Max = %f, want 2000", s.Max)
		}
	})

	t.Run("ratio P50", func(t *testing.T) {
		ratios := report.Ratio
		if ratios.Count != 5 {
			t.Errorf("Ratio count = %d, want 5", ratios.Count)
		}
		if math.Abs(ratios.P50-0.5) > 0.001 {
			t.Errorf("P50 = %f, want ~0.5", ratios.P50)
		}
	})

	t.Run("cache hit rate", func(t *testing.T) {
		s := report.CacheHitRate
		if s.Count != 0 {
			t.Errorf("Cache count = %d, want 0 (no cache activity)", s.Count)
		}
	})
}

func TestComputeCalibrationSingleSession(t *testing.T) {
	ref := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	events := []source.TokenEvent{
		{SessionID: "s1", Project: "p1", Harness: "claude-code", InputTokens: 100, OutputTokens: 50, CostUSD: 0.50, Timestamp: ref},
	}

	report := ComputeCalibration(events)

	if report.TotalSessions != 1 {
		t.Errorf("TotalSessions = %d, want 1", report.TotalSessions)
	}

	s := report.SessionCost
	if s.Count != 1 {
		t.Errorf("Count = %d, want 1", s.Count)
	}
	if s.Mean != 0.50 {
		t.Errorf("Mean = %f, want 0.50", s.Mean)
	}
	if s.Std != 0 {
		t.Errorf("Std = %f, want 0 (single sample)", s.Std)
	}
	if s.P10 != 0.50 || s.P50 != 0.50 || s.P99 != 0.50 {
		t.Errorf("All percentiles should be 0.50 for single sample")
	}
	if s.Min != 0.50 || s.Max != 0.50 {
		t.Errorf("Min/Max should be 0.50 for single sample")
	}
}

func TestComputeCalibrationNoSessions(t *testing.T) {
	report := ComputeCalibration(nil)

	if report.TotalSessions != 0 {
		t.Errorf("TotalSessions = %d, want 0", report.TotalSessions)
	}
	if report.SessionCost.Count != 0 {
		t.Errorf("SessionCost.Count = %d, want 0", report.SessionCost.Count)
	}
}

func TestDistStatsEmpty(t *testing.T) {
	ds := computeDistStats(nil)
	if ds.Count != 0 {
		t.Errorf("Count = %d, want 0", ds.Count)
	}
}

func TestSuggestionsNotNil(t *testing.T) {
	report := ComputeCalibration(eventsForCalibration())
	if report.Suggestions == nil {
		t.Error("Suggestions should not be nil")
	}
	if len(report.Suggestions) == 0 {
		t.Error("Suggestions should not be empty for test data")
	}

	keys := make(map[string]bool)
	for _, s := range report.Suggestions {
		keys[s.ConfigKey] = true
		if s.Value <= 0 {
			t.Errorf("Suggestion %q has non-positive value: %f", s.ConfigKey, s.Value)
		}
		if s.Rationale == "" {
			t.Errorf("Suggestion %q has empty rationale", s.ConfigKey)
		}
	}

	if !keys["cost_outlier_sigma"] {
		t.Error("Missing cost_outlier_sigma suggestion")
	}
	if !keys["input_overconsumption_sigma"] {
		t.Error("Missing input_overconsumption_sigma suggestion")
	}
	if !keys["output_explosion_sigma"] {
		t.Error("Missing output_explosion_sigma suggestion")
	}
}

func TestCalibrationReportJSONMarshal(t *testing.T) {
	report := ComputeCalibration(eventsForCalibration())

	data, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var unmarshaled CalibrationReport
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if unmarshaled.TotalSessions != report.TotalSessions {
		t.Errorf("TotalSessions = %d, want %d", unmarshaled.TotalSessions, report.TotalSessions)
	}
	if unmarshaled.SessionCost.Count != report.SessionCost.Count {
		t.Errorf("SessionCost.Count = %d, want %d", unmarshaled.SessionCost.Count, report.SessionCost.Count)
	}
	if unmarshaled.SessionCost.Mean != report.SessionCost.Mean {
		t.Errorf("SessionCost.Mean = %f, want %f", unmarshaled.SessionCost.Mean, report.SessionCost.Mean)
	}
	if len(unmarshaled.Suggestions) != len(report.Suggestions) {
		t.Errorf("Suggestions len = %d, want %d", len(unmarshaled.Suggestions), len(report.Suggestions))
	}
}

func TestSubagentCount(t *testing.T) {
	ref := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	events := []source.TokenEvent{
		{SessionID: "s1", Project: "p1", Harness: "claude-code", CostUSD: 0.50, Timestamp: ref, IsSubagent: false, InputTokens: 100, OutputTokens: 50},
		{SessionID: "s2", Project: "p1", Harness: "claude-code", ParentSessionID: "s1", CostUSD: 0.25, Timestamp: ref, IsSubagent: true, InputTokens: 50, OutputTokens: 25},
		{SessionID: "s3", Project: "p1", Harness: "claude-code", ParentSessionID: "s1", CostUSD: 0.25, Timestamp: ref, IsSubagent: true, InputTokens: 50, OutputTokens: 25},
	}

	report := ComputeCalibration(events)

	if report.TotalSessions != 3 {
		t.Errorf("TotalSessions = %d, want 3", report.TotalSessions)
	}
	if report.TotalSubagents != 2 {
		t.Errorf("TotalSubagents = %d, want 2", report.TotalSubagents)
	}
}

func TestSubagentOverheadDistribution(t *testing.T) {
	ref := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	events := []source.TokenEvent{
		{SessionID: "parent1", Project: "p1", Harness: "claude-code", CostUSD: 0.50, Timestamp: ref, IsSubagent: false, InputTokens: 100, OutputTokens: 50},
		{SessionID: "sub1", Project: "p1", Harness: "claude-code", ParentSessionID: "parent1", CostUSD: 0.50, Timestamp: ref, IsSubagent: true, AgentType: "task", InputTokens: 100, OutputTokens: 50},
		{SessionID: "parent2", Project: "p2", Harness: "claude-code", CostUSD: 0.50, Timestamp: ref, IsSubagent: false, InputTokens: 100, OutputTokens: 50},
	}

	report := ComputeCalibration(events)

	if report.SubagentOverhead.Count != 1 {
		t.Errorf("SubagentOverhead count = %d, want 1 (only parent1 has subagents)", report.SubagentOverhead.Count)
	}
	expectedOverhead := (0.50 / 1.00) * 100.0
	if math.Abs(report.SubagentOverhead.Mean-expectedOverhead) > 0.001 {
		t.Errorf("SubagentOverhead.Mean = %f, want %f", report.SubagentOverhead.Mean, expectedOverhead)
	}
}

func TestCacheHitRateDistribution(t *testing.T) {
	ref := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	events := []source.TokenEvent{
		{SessionID: "s1", Project: "p1", Harness: "claude-code", CacheRead: 70, CacheWrite: 30, CostUSD: 0.50, Timestamp: ref, InputTokens: 100, OutputTokens: 50},
		{SessionID: "s2", Project: "p1", Harness: "claude-code", CacheRead: 0, CacheWrite: 0, CostUSD: 0.30, Timestamp: ref, InputTokens: 100, OutputTokens: 50},
	}

	report := ComputeCalibration(events)

	if report.CacheHitRate.Count != 1 {
		t.Errorf("CacheHitRate count = %d, want 1 (only s1 has cache)", report.CacheHitRate.Count)
	}
}

func TestTERDistribution(t *testing.T) {
	ref := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	events := []source.TokenEvent{
		{SessionID: "s1", Project: "p1", Harness: "claude-code", InputTokens: 100, OutputTokens: 50, CacheRead: 0, CacheWrite: 0, CostUSD: 0.50, Timestamp: ref},
		{SessionID: "s2", Project: "p1", Harness: "claude-code", InputTokens: 0, OutputTokens: 0, CacheRead: 0, CacheWrite: 0, CostUSD: 0, Timestamp: ref},
	}

	report := ComputeCalibration(events)

	if report.TokenEfficiency.Count != 1 {
		t.Errorf("TokenEfficiency count = %d, want 1 (only s1 has tokens)", report.TokenEfficiency.Count)
	}
}

func TestPercentileRank(t *testing.T) {
	sorted := []float64{1, 2, 3, 4, 5}
	if percentileRank(sorted, 3) != 60.0 {
		t.Errorf("percentileRank(3) = %f, want 60", percentileRank(sorted, 3))
	}
	if percentileRank(sorted, 0) != 0 {
		t.Errorf("percentileRank(0) = %f, want 0", percentileRank(sorted, 0))
	}
	if percentileRank(sorted, 5) != 100.0 {
		t.Errorf("percentileRank(5) = %f, want 100", percentileRank(sorted, 5))
	}
}
