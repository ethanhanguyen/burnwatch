package output

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ethanhanguyen/burnwatch/analyze"
	"github.com/ethanhanguyen/burnwatch/source"
)

func calibrationTestEvents() []source.TokenEvent {
	ref := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)

	return []source.TokenEvent{
		{SessionID: "s1", Project: "p1", Harness: "claude-code", InputTokens: 100, OutputTokens: 50, CacheRead: 0, CacheWrite: 0, CostUSD: 0.10, Timestamp: ref},
		{SessionID: "s1", Project: "p1", Harness: "claude-code", InputTokens: 200, OutputTokens: 100, CacheRead: 0, CacheWrite: 0, CostUSD: 0.20, Timestamp: ref},
		{SessionID: "s2", Project: "p1", Harness: "claude-code", InputTokens: 300, OutputTokens: 200, CacheRead: 0, CacheWrite: 0, CostUSD: 0.50, Timestamp: ref},
		{SessionID: "s3", Project: "p1", Harness: "claude-code", InputTokens: 500, OutputTokens: 100, CacheRead: 0, CacheWrite: 0, CostUSD: 0.30, Timestamp: ref},
		{SessionID: "s4", Project: "p2", Harness: "opencode", InputTokens: 1000, OutputTokens: 500, CacheRead: 0, CacheWrite: 0, CostUSD: 1.00, Timestamp: ref},
		{SessionID: "s5", Project: "p2", Harness: "opencode", InputTokens: 2000, OutputTokens: 2000, CacheRead: 0, CacheWrite: 0, CostUSD: 2.00, Timestamp: ref},
	}
}

func TestCalibrationTextOutput(t *testing.T) {
	report := analyze.ComputeCalibration(calibrationTestEvents())
	text := FormatCalibrationText(report)

	required := []string{
		"Your data:",
		"across 2 projects",
		"Period:",
		"Session costs ($):",
		"Input tokens:",
		"Output tokens:",
		"Output/input ratio:",
		"Cache hit rate (%):",
		"Token efficiency ratio:",
		"Suggested thresholds",
		"cost_outlier_sigma",
		"input_overconsumption_sigma",
		"output_explosion_sigma",
	}

	for _, want := range required {
		if !strings.Contains(text, want) {
			t.Errorf("text output missing %q", want)
		}
	}

	if report.TotalSessions > 0 && !strings.Contains(text, "n=") {
		t.Error("text output missing 'n=' count")
	}
	if report.SessionCost.Count > 0 && !strings.Contains(text, "μ=") {
		t.Error("text output missing 'μ=' for cost")
	}
	if report.SessionCost.Count > 1 && !strings.Contains(text, "σ=") {
		t.Error("text output missing 'σ=' for cost")
	}
	if report.SessionCost.Count > 0 && !strings.Contains(text, "P50=") {
		t.Error("text output missing percentile 'P50='")
	}
}

func TestCalibrationTextEmptyReport(t *testing.T) {
	report := analyze.ComputeCalibration(nil)
	text := FormatCalibrationText(report)

	if !strings.Contains(text, "0 main sessions") {
		t.Error("text output for empty report should show 0 sessions")
	}
}

func TestCalibrationTextWithSubagentOverhead(t *testing.T) {
	ref := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	events := []source.TokenEvent{
		{SessionID: "parent1", Project: "p1", Harness: "claude-code", CostUSD: 0.50, Timestamp: ref, IsSubagent: false, InputTokens: 100, OutputTokens: 50},
		{SessionID: "sub1", Project: "p1", Harness: "claude-code", ParentSessionID: "parent1", CostUSD: 0.50, Timestamp: ref, IsSubagent: true, AgentType: "task", InputTokens: 100, OutputTokens: 50},
	}

	report := analyze.ComputeCalibration(events)
	text := FormatCalibrationText(report)

	if !strings.Contains(text, "Subagent overhead (%):") {
		t.Error("text output missing subagent overhead section")
	}
	if !strings.Contains(text, "sessions with subagents") {
		t.Error("text output missing subagent count annotation")
	}
}

func TestCalibrationTextNoCache(t *testing.T) {
	report := analyze.ComputeCalibration(calibrationTestEvents())
	text := FormatCalibrationText(report)

	if !strings.Contains(text, "n=0  (no data)") {
		t.Error("Cache section with 0 count should show 'no data'")
	}
}

func TestCalibrationJSONOutput(t *testing.T) {
	report := analyze.ComputeCalibration(calibrationTestEvents())

	data, err := FormatCalibrationJSON(report)
	if err != nil {
		t.Fatalf("FormatCalibrationJSON: %v", err)
	}

	var result analyze.CalibrationReport
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("unmarshal JSON: %v", err)
	}

	if result.TotalSessions != report.TotalSessions {
		t.Errorf("TotalSessions = %d, want %d", result.TotalSessions, report.TotalSessions)
	}
	if result.SessionCost.Count != report.SessionCost.Count {
		t.Errorf("SessionCost.Count = %d, want %d", result.SessionCost.Count, report.SessionCost.Count)
	}
	if math.Abs(result.SessionCost.Mean-report.SessionCost.Mean) > 0.0001 {
		t.Errorf("SessionCost.Mean = %f, want %f", result.SessionCost.Mean, report.SessionCost.Mean)
	}
	if len(result.Suggestions) != len(report.Suggestions) {
		t.Errorf("Suggestions len = %d, want %d", len(result.Suggestions), len(report.Suggestions))
	}
}

func TestCalibrationTextGoldenFile(t *testing.T) {
	report := analyze.ComputeCalibration(calibrationTestEvents())
	got := FormatCalibrationText(report)

	goldenPath := filepath.Join("..", "testdata", "calibrate_text.golden")

	if *updateGolden {
		if err := os.WriteFile(goldenPath, []byte(got), 0644); err != nil {
			t.Fatalf("write golden file: %v", err)
		}
		t.Logf("updated golden file: %s", goldenPath)
		return
	}

	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("golden file not found: %s (run with -update to generate)", goldenPath)
	}

	if got != string(want) {
		t.Errorf("text output differs from golden file.\n=== GOT ===\n%s\n=== WANT ===\n%s", got, string(want))
	}
}
