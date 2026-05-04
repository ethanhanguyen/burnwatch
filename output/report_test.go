package output

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/ethanhanguyen/burnwatch/analyze"
	"github.com/ethanhanguyen/burnwatch/config"
	"github.com/ethanhanguyen/burnwatch/source"
)

func TestFormatReport_Structure(t *testing.T) {
	events := makeTestEvents()
	signals := makeTestSignals()
	report := FormatReport(events, nil, signals, nil, nil, "dev", time.Date(2026, 5, 3, 14, 30, 0, 0, time.UTC))

	checks := []string{
		"<!DOCTYPE html>",
		`<script src="https://cdn.jsdelivr.net/npm/chart.js@4">`,
		"const REPORT = {",
		"<footer>",
		"#CA8A04",
		"#1A0F0A",
		"Cinzel",
		"Spectral",
		"Fira Code",
	}
	for _, c := range checks {
		if !strings.Contains(report, c) {
			t.Errorf("report missing %q", c)
		}
	}
}

func TestFormatReport_DataEmbedded(t *testing.T) {
	events := makeTestEvents()
	signals := makeTestSignals()
	report := FormatReport(events, nil, signals, nil, nil, "v3.0.0", time.Date(2026, 5, 3, 0, 0, 0, 0, time.UTC))

	scriptStart := strings.Index(report, "const REPORT = ")
	if scriptStart < 0 {
		t.Fatal("REPORT data block not found")
	}
	raw := report[scriptStart+len("const REPORT = "):]
	scriptEnd := strings.Index(raw, ";\n</script>")
	if scriptEnd < 0 {
		t.Fatal("REPORT data block end not found")
	}
	raw = raw[:scriptEnd]

	var data reportData
	if err := json.Unmarshal([]byte(raw), &data); err != nil {
		t.Fatalf("unmarshal REPORT data: %v", err)
	}

	if data.Version != "v3.0.0" {
		t.Errorf("version = %q, want v3.0.0", data.Version)
	}
	if data.Generated == "" {
		t.Error("generated timestamp is empty")
	}
	var totalCost float64
	for _, e := range events {
		totalCost += e.CostUSD
	}
	if diff := data.Summary.TotalCost - totalCost; diff > 0.01 || diff < -0.01 {
		t.Errorf("summary totalCost = %f, want ~%f", data.Summary.TotalCost, totalCost)
	}
	if data.Summary.TotalSignals != len(signals) {
		t.Errorf("summary.totalSignals = %d, want %d", data.Summary.TotalSignals, len(signals))
	}
}

func TestFormatReport_EmptyData(t *testing.T) {
	report := FormatReport(nil, nil, nil, nil, nil, "dev", time.Now())

	if !strings.HasPrefix(report, "<!DOCTYPE html>") {
		t.Error("empty report should still be valid HTML")
	}
	if !strings.Contains(report, "const REPORT = {") {
		t.Error("empty report should have REPORT data block")
	}
}

func TestFormatReport_CostOverTime(t *testing.T) {
	events := []source.TokenEvent{
		{
			SessionID: "s1", Model: "claude-sonnet", Project: "test", Harness: "claude-code",
			Timestamp: time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC), InputTokens: 1000, OutputTokens: 100,
			CostUSD: 1.50, EventIndex: 1,
		},
		{
			SessionID: "s1", Model: "claude-sonnet", Project: "test", Harness: "claude-code",
			Timestamp: time.Date(2026, 5, 2, 10, 0, 0, 0, time.UTC), InputTokens: 2000, OutputTokens: 200,
			CostUSD: 3.00, EventIndex: 2,
		},
		{
			SessionID: "s1", Model: "claude-sonnet", Project: "test", Harness: "claude-code",
			Timestamp: time.Date(2026, 5, 3, 10, 0, 0, 0, time.UTC), InputTokens: 500, OutputTokens: 50,
			CostUSD: 0.75, EventIndex: 3,
		},
	}
	report := FormatReport(events, nil, nil, nil, nil, "dev", time.Now())

	scriptStart := strings.Index(report, "const REPORT = ")
	raw := report[scriptStart+len("const REPORT = "):]
	scriptEnd := strings.Index(raw, ";\n</script>")
	raw = raw[:scriptEnd]

	var data reportData
	if err := json.Unmarshal([]byte(raw), &data); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if len(data.CostOverTime) != 3 {
		t.Fatalf("costOverTime length = %d, want 3", len(data.CostOverTime))
	}
	if data.CostOverTime[0].Cost != 1.50 {
		t.Errorf("day1 cost = %f, want 1.50", data.CostOverTime[0].Cost)
	}
	if data.CostOverTime[1].Cost != 3.00 {
		t.Errorf("day2 cost = %f, want 3.00", data.CostOverTime[1].Cost)
	}
	if data.CostOverTime[2].Cost != 0.75 {
		t.Errorf("day3 cost = %f, want 0.75", data.CostOverTime[2].Cost)
	}
}

func TestFormatReport_WasteByType(t *testing.T) {
	signals := makeTestSignals()
	waste := computeWasteByType(signals)

	if len(waste) == 0 {
		t.Error("expected waste projects")
	}
	var found bool
	for _, wp := range waste {
		if wp.Project == "test-project" {
			found = true
			if wp.Total <= 0 {
				t.Error("expected non-zero total cost for test-project")
			}
		}
	}
	if !found {
		t.Error("test-project not found in waste breakdown")
	}
}

func TestFormatReport_TopFiles(t *testing.T) {
	signals := []analyze.WasteSignal{
		{SessionID: "s1", Reason: "file_reread", Detail: "config/settings.json read 5 times, 0 cache hits between reads", SessionCost: 1.0},
	}
	files := computeTopFiles(signals)

	if len(files) < 1 {
		t.Fatal("expected top files")
	}
	if files[0].Path != "config/settings.json" {
		t.Errorf("path = %q, want config/settings.json", files[0].Path)
	}
	if files[0].ReadCount != 5 {
		t.Errorf("readCount = %d, want 5", files[0].ReadCount)
	}
}

func TestFormatReport_SignalTimeline(t *testing.T) {
	events := loadScenarioJSONL(t, "explain_loop.jsonl")
	assignEventIndex(events)
	cfg := v3Cfg()
	cfg.Thresholds.ToolLoopMaxRepeats = 5

	baselines := analyze.ComputeBaselines(events, config.Defaults())
	signals := analyze.DetectWaste(events, baselines, nil, cfg)

	data := computeReportData(events, baselines, signals, nil)

	if len(data.SignalTimelines) == 0 {
		t.Fatal("expected signal timelines")
	}
	if data.SignalTimelines["ses_explain_loop"] == nil {
		t.Error("expected timeline for ses_explain_loop")
	}
	var hasAnnotation bool
	for _, te := range data.SignalTimelines["ses_explain_loop"] {
		if len(te.Annotations) > 0 {
			hasAnnotation = true
			break
		}
	}
	if !hasAnnotation {
		t.Error("expected LOOP REPEAT annotations in timeline")
	}
}

func TestFormatReport_SignalTimelineReRead(t *testing.T) {
	events := loadScenarioJSONL(t, "explain_reread.jsonl")
	assignEventIndex(events)
	cfg := v3Cfg()
	cfg.Thresholds.FileRereadMinCount = 3

	baselines := analyze.ComputeBaselines(events, config.Defaults())
	signals := analyze.DetectWaste(events, baselines, nil, cfg)

	data := computeReportData(events, baselines, signals, nil)

	if data.SignalTimelines["ses_explain_reread"] == nil {
		t.Fatal("expected timeline for ses_explain_reread")
	}
	var hasAnnotation bool
	for _, te := range data.SignalTimelines["ses_explain_reread"] {
		if len(te.Annotations) > 0 {
			hasAnnotation = true
			break
		}
	}
	if !hasAnnotation {
		t.Error("expected RE-READ annotations in timeline")
	}
}

func TestFormatReport_ModelBreakdown(t *testing.T) {
	events := []source.TokenEvent{
		{SessionID: "s1", Model: "claude-sonnet", Project: "test", Harness: "claude-code", Timestamp: time.Now(), InputTokens: 1000, OutputTokens: 100, CostUSD: 5.0},
		{SessionID: "s1", Model: "claude-sonnet", Project: "test", Harness: "claude-code", Timestamp: time.Now(), InputTokens: 1000, OutputTokens: 100, CostUSD: 3.0},
		{SessionID: "s2", Model: "claude-opus", Project: "test", Harness: "claude-code", Timestamp: time.Now(), InputTokens: 1000, OutputTokens: 100, CostUSD: 2.0},
	}
	models := computeModelBreakdown(events)

	if len(models) != 2 {
		t.Fatalf("model count = %d, want 2", len(models))
	}
	var sonnet, opus *modelBreakdown
	for i := range models {
		if models[i].Model == "claude-sonnet" {
			sonnet = &models[i]
		}
		if models[i].Model == "claude-opus" {
			opus = &models[i]
		}
	}
	if sonnet == nil {
		t.Error("sonnet model not found")
	} else if sonnet.Cost != 8.0 {
		t.Errorf("sonnet cost = %f, want 8.0", sonnet.Cost)
	}
	if opus == nil {
		t.Error("opus model not found")
	} else if opus.Cost != 2.0 {
		t.Errorf("opus cost = %f, want 2.0", opus.Cost)
	}
}

func TestFormatReport_SubagentTree(t *testing.T) {
	trees := []analyze.SubagentTree{
		{
			SessionID: "parent", SubagentCost: 5.0, TotalCost: 10.0,
			Subagents: []analyze.SubagentNode{
				{SessionID: "sub1", AgentType: "explore", Cost: 3.0},
				{SessionID: "sub2", AgentType: "coder", Cost: 2.0},
			},
		},
	}
	root := computeSubagentTreeData(trees)

	if root.Cost <= 0 {
		t.Error("root cost should be > 0")
	}
	if len(root.Children) != 1 {
		t.Fatalf("root children = %d, want 1", len(root.Children))
	}
	if len(root.Children[0].Children) != 2 {
		t.Fatalf("session children = %d, want 2", len(root.Children[0].Children))
	}
}

func TestFormatReport_SignalsSorting(t *testing.T) {
	signals := []analyze.WasteSignal{
		{SessionID: "s1", Severity: "medium", SessionCost: 5.0, Reason: "tool_call_loop", Detail: "detail"},
		{SessionID: "s2", Severity: "high", SessionCost: 2.0, Reason: "cost_outlier", Detail: "detail"},
		{SessionID: "s3", Severity: "high", SessionCost: 10.0, Reason: "subagent_overlap", Detail: "detail"},
	}
	result := computeReportSignals(signals)

	if len(result) != 3 {
		t.Fatalf("length = %d, want 3", len(result))
	}
	if result[0].Severity != "high" {
		t.Error("first should be high severity")
	}
	if result[0].Cost != 10.0 && result[1].Cost != 10.0 {
		t.Error("highest cost within high should come first")
	}
}

func TestFormatReport_ReportJSON(t *testing.T) {
	events := makeTestEvents()
	signals := makeTestSignals()
	jsonData, err := FormatReportJSON(events, nil, signals, nil, "dev", time.Now())
	if err != nil {
		t.Fatalf("FormatReportJSON: %v", err)
	}
	if len(jsonData) == 0 {
		t.Error("JSON output is empty")
	}

	var data reportData
	if err := json.Unmarshal(jsonData, &data); err != nil {
		t.Fatalf("unmarshal JSON: %v", err)
	}
	if data.Summary.TotalSignals == 0 {
		t.Error("expected signals in JSON output")
	}
}

func TestComputeReportSummary(t *testing.T) {
	events := []source.TokenEvent{
		{SessionID: "s1", Project: "p1", CostUSD: 1.0, Timestamp: time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)},
		{SessionID: "s1", Project: "p1", CostUSD: 2.0, Timestamp: time.Date(2026, 5, 3, 10, 0, 0, 0, time.UTC)},
		{SessionID: "s2", Project: "p2", CostUSD: 3.0, Timestamp: time.Date(2026, 5, 2, 10, 0, 0, 0, time.UTC)},
	}
	signals := []analyze.WasteSignal{
		{SessionID: "s1", Reason: "tool_call_loop", Detail: "x", Severity: "high"},
		{SessionID: "s2", Reason: "file_reread", Detail: "x", Severity: "medium"},
	}
	s := computeReportSummary(events, signals)

	if s.TotalCost != 6.0 {
		t.Errorf("totalCost = %f, want 6.0", s.TotalCost)
	}
	if s.Sessions != 2 {
		t.Errorf("sessions = %d, want 2", s.Sessions)
	}
	if s.ProjectCount != 2 {
		t.Errorf("projectCount = %d, want 2", s.ProjectCount)
	}
	if s.TotalSignals != 2 {
		t.Errorf("totalSignals = %d, want 2", s.TotalSignals)
	}
	if s.TotalToolLoop != 1 {
		t.Errorf("totalToolLoop = %d, want 1", s.TotalToolLoop)
	}
	if s.TotalReRead != 1 {
		t.Errorf("totalReRead = %d, want 1", s.TotalReRead)
	}
}

func TestRound2(t *testing.T) {
	tests := []struct {
		in  float64
		out float64
	}{
		{1.234, 1.23},
		{1.235, 1.24},
		{1.0, 1.0},
		{0.0, 0.0},
	}
	for _, tt := range tests {
		got := round2(tt.in)
		if got != tt.out {
			t.Errorf("round2(%.3f) = %.2f, want %.2f", tt.in, got, tt.out)
		}
	}
}

func makeTestEvents() []source.TokenEvent {
	now := time.Date(2026, 5, 3, 12, 0, 0, 0, time.UTC)
	return []source.TokenEvent{
		{
			SessionID: "ses_test_001", Model: "claude-sonnet-4-20250514", Project: "test-project",
			Harness: "claude-code", Timestamp: now, Provider: "test",
			InputTokens: 10000, OutputTokens: 1000, CostUSD: 2.50, EventIndex: 1,
		},
		{
			SessionID: "ses_test_001", Model: "claude-sonnet-4-20250514", Project: "test-project",
			Harness: "claude-code", Timestamp: now.Add(10 * time.Minute), Provider: "test",
			InputTokens: 5000, OutputTokens: 800, CostUSD: 1.25, EventIndex: 2,
		},
		{
			SessionID: "ses_test_002", Model: "claude-opus-4-20250514", Project: "test-project",
			Harness: "claude-code", Timestamp: now.Add(-24 * time.Hour), Provider: "test",
			InputTokens: 20000, OutputTokens: 5000, CostUSD: 8.00, EventIndex: 1,
		},
	}
}

func makeTestSignals() []analyze.WasteSignal {
	return []analyze.WasteSignal{
		{
			SessionID: "ses_test_001", Project: "test-project", Severity: "high",
			Reason: "tool_call_loop", Detail: `read("src/main.go") called 6 times consecutively`,
			Metric: 6, Threshold: 5, SessionCost: 3.75, Model: "claude-sonnet-4-20250514",
			InputTokens: 15000, OutputTokens: 1800,
		},
		{
			SessionID: "ses_test_002", Project: "test-project", Severity: "medium",
			Reason: "file_reread", Detail: "config/settings.json read 4 times, 0 cache hits between reads",
			Metric: 4, Threshold: 3, SessionCost: 8.00, Model: "claude-opus-4-20250514",
			InputTokens: 20000, OutputTokens: 5000,
		},
	}
}
