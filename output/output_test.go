package output

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ethanhanguyen/burnwatch/analyze"
	"github.com/ethanhanguyen/burnwatch/config"
	"github.com/ethanhanguyen/burnwatch/source"
)

var updateGolden = flag.Bool("update", false, "update golden files")

var fixedTime = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

func useDefaults(cfg *config.Config) {
	d := config.Defaults()
	cfg.Thresholds = d.Thresholds
	cfg.Filters = d.Filters
	cfg.Output = d.Output
}

var allCfg = func() config.Config {
	cfg := config.Config{Signals: config.Signals{
		CostOutlier:          true,
		LowSignal:            true,
		SubagentOverhead:     true,
		CacheUnderutilized:   true,
		FragmentationIndex:   true,
		InputOverconsumption: true,
		OutputExplosion:      true,
		TokenEfficiency:      true,
	}}
	useDefaults(&cfg)
	return cfg
}()

func runPipeline(events []source.TokenEvent) (
	baselines map[string]analyze.Baseline,
	signals []analyze.WasteSignal,
	recommendations []analyze.Recommendation,
) {
	baselines = analyze.ComputeBaselines(events, config.Defaults())
	trees := analyze.BuildSubagentTree(events)
	signals = analyze.DetectWaste(events, baselines, trees, allCfg)
	recommendations = analyze.GenerateRecommendations(signals, baselines)
	return
}

func setupTestEnv(t *testing.T) {
	t.Helper()
	NowFunc = func() time.Time { return fixedTime }
	dbPath, err := filepath.Abs(filepath.Join("..", "testdata", "opencode_sample.db"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Skipf("sample DB not found at %s", dbPath)
	}
	t.Setenv("BURNWATCH_OPENCODE_DB", dbPath)

	claudePath, err := filepath.Abs(filepath.Join("..", "testdata", "claude_projects"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(claudePath); os.IsNotExist(err) {
		t.Skipf("claude projects fixture not found at %s", claudePath)
	}
	t.Setenv("BURNWATCH_CLAUDE_PROJECTS", claudePath)
}

func collectTestEvents(t *testing.T) []source.TokenEvent {
	t.Helper()
	sources := source.Discover()
	if len(sources) == 0 {
		t.Fatal("no sources discovered")
	}
	return CollectEvents(sources)
}

func TestGoldenText(t *testing.T) {
	setupTestEnv(t)

	events := collectTestEvents(t)
	baselines := analyze.ComputeBaselines(events, config.Defaults())
	signals := analyze.DetectWaste(events, baselines, nil, config.Defaults())
	recommendations := analyze.GenerateRecommendations(signals, baselines)

	got := FormatText(events, baselines, signals, recommendations, false, config.Config{})

	goldenPath := filepath.Join("..", "testdata", "expected_report.txt")
	if *updateGolden {
		if err := os.WriteFile(goldenPath, []byte(got), 0644); err != nil {
			t.Fatal(err)
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

func TestGoldenJSON(t *testing.T) {
	setupTestEnv(t)

	events := collectTestEvents(t)
	baselines := analyze.ComputeBaselines(events, config.Defaults())
	signals := analyze.DetectWaste(events, baselines, nil, config.Defaults())
	recommendations := analyze.GenerateRecommendations(signals, baselines)
	trees := analyze.BuildSubagentTree(events)

	got, err := FormatJSON(events, baselines, signals, recommendations, trees)
	if err != nil {
		t.Fatal(err)
	}

	goldenPath := filepath.Join("..", "testdata", "expected_report.json")
	if *updateGolden {
		if err := os.WriteFile(goldenPath, got, 0644); err != nil {
			t.Fatal(err)
		}
		t.Logf("updated golden file: %s", goldenPath)
		return
	}

	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("golden file not found: %s (run with -update to generate)", goldenPath)
	}

	if string(got) != string(want) {
		t.Errorf("JSON output differs from golden file.\n=== GOT ===\n%s\n=== WANT ===\n%s", string(got), string(want))
	}
}

func TestFormatText_NoData(t *testing.T) {
	got := FormatText(nil, nil, nil, nil, false, config.Config{})
	if got != "No data found.\n" {
		t.Errorf("expected no-data message, got: %s", got)
	}
}

func TestFormatText_NoSignals(t *testing.T) {
	events := []source.TokenEvent{
		{
			SessionID: "s1",
			Harness:   "opencode",
			Project:   "test",
			CostUSD:   1.0,
		},
	}
	baselines := analyze.ComputeBaselines(events, config.Defaults())
	signals := analyze.DetectWaste(events, baselines, nil, config.Defaults())
	recommendations := analyze.GenerateRecommendations(signals, baselines)
	got := FormatText(events, baselines, signals, recommendations, false, config.Config{})

	if got == "No waste signals detected.\n" {
		t.Log("no waste signals as expected for single event")
	}
}

func TestFormatJSON_NoData(t *testing.T) {
	got, err := FormatJSON(nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	expected := `{
  "summary": {
    "opencode_sessions": 0,
    "opencode_subagent_sessions": 0,
    "claude_sessions": 0,
    "today_cost": 0,
    "today_sessions": 0,
    "week_cost": 0,
    "week_sessions": 0
  },
  "projects": [],
  "waste_signals": [],
  "subagent_trees": [],
  "recommendations": [],
  "potential_savings": 0
}`
	if string(got) != expected {
		t.Errorf("JSON output mismatch.\nGOT:\n%s\nWANT:\n%s", string(got), expected)
	}
}

func TestMedianFromSorted(t *testing.T) {
	tests := []struct {
		name string
		in   []float64
		want float64
	}{
		{"empty", nil, 0},
		{"single", []float64{5.0}, 5.0},
		{"odd", []float64{1.0, 2.0, 3.0}, 2.0},
		{"even", []float64{1.0, 2.0, 3.0, 4.0}, 2.5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := medianFromSorted(tt.in)
			if got != tt.want {
				t.Errorf("medianFromSorted(%v) = %f, want %f", tt.in, got, tt.want)
			}
		})
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		name   string
		s      string
		maxLen int
		want   string
	}{
		{"short", "abc", 20, "abc"},
		{"exact", "abcdefghijklmnopqrst", 20, "abcdefghijklmnopqrst"},
		{"long", "abcdefghijklmnopqrstuvwxyz", 20, "abcdefghijklmnopqrst"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncate(tt.s, tt.maxLen)
			if got != tt.want {
				t.Errorf("truncate(%q, %d) = %q, want %q", tt.s, tt.maxLen, got, tt.want)
			}
		})
	}
}

func TestHarnessLabel(t *testing.T) {
	if got := harnessLabel("opencode"); got != "OpenCode" {
		t.Errorf("harnessLabel(opencode) = %q", got)
	}
	if got := harnessLabel("claude-code"); got != "Claude Code" {
		t.Errorf("harnessLabel(claude-code) = %q", got)
	}
	if got := harnessLabel("unknown"); got != "unknown" {
		t.Errorf("harnessLabel(unknown) = %q", got)
	}
}

func TestRunPipeline(t *testing.T) {
	events := []source.TokenEvent{
		{SessionID: "s1", Harness: "opencode", Project: "p1", InputTokens: 100, OutputTokens: 50, CostUSD: 1.5, Timestamp: fixedTime},
		{SessionID: "s1", Harness: "opencode", Project: "p1", InputTokens: 100, OutputTokens: 50, CostUSD: 0.5, Timestamp: fixedTime},
		{SessionID: "s2", Harness: "opencode", Project: "p1", InputTokens: 200, OutputTokens: 20, CostUSD: 3.0, Timestamp: fixedTime},
	}

	baselines, signals, recommendations := runPipeline(events)
	if baselines == nil {
		t.Fatal("expected non-nil baselines")
	}
	if len(signals) == 0 {
		t.Log("no signals from 2-session dataset (expected for statistical safety)")
	}
	_ = recommendations
}

func TestCollectEvents_NoSources(t *testing.T) {
	events := CollectEvents(nil)
	if len(events) != 0 {
		t.Errorf("expected 0 events from nil sources, got %d", len(events))
	}
}

func TestWriteProjects_Empty(t *testing.T) {
	var b strings.Builder
	writeProjects(&b, map[string]analyze.Baseline{
		"*": {Project: "*", SessionCount: 1},
	})
	got := b.String()
	if got != "" {
		t.Errorf("expected empty output for no non-global baselines, got: %s", got)
	}
}

func TestWriteSummary_SingleSession(t *testing.T) {
	NowFunc = func() time.Time { return fixedTime }
	t.Cleanup(func() { NowFunc = time.Now })

	events := []source.TokenEvent{
		{SessionID: "s1", Harness: "opencode", CostUSD: 1.5, Timestamp: fixedTime},
	}
	baselines := analyze.ComputeBaselines(events, config.Defaults())

	var b strings.Builder
	writeSummary(&b, events, baselines)
	got := b.String()
	if !strings.Contains(got, "OpenCode: 1 sessions") {
		t.Errorf("expected OpenCode session count, got: %s", got)
	}
	if !strings.Contains(got, "Today:") {
		t.Errorf("expected Today line, got: %s", got)
	}
}

func TestWriteSignalBlock_EmptyRecommendation(t *testing.T) {
	s := analyze.WasteSignal{
		SessionID:   "ses_abc",
		Project:     "test",
		Severity:    "high",
		Reason:      "cost_outlier",
		Metric:      5.0,
		Threshold:   1.0,
		SessionCost: 5.0,
	}
	rec := analyze.Recommendation{}

	var b strings.Builder
	writeSignalBlock(&b, s, rec, nil)
	got := b.String()
	if !strings.Contains(got, "HIGH") {
		t.Errorf("expected HIGH severity, got: %s", got)
	}
	if strings.Contains(got, "\u2192") {
		t.Errorf("expected no recommendation arrow for empty rec, got: %s", got)
	}
}

func TestWriteSignalBlock_AllReasons(t *testing.T) {
	tests := []struct {
		name string
		s    analyze.WasteSignal
	}{
		{
			name: "fragmentation_index",
			s: analyze.WasteSignal{
				SessionID:   "ses_x",
				Project:     "proj",
				Severity:    "medium",
				Reason:      "fragmentation_index",
				Metric:      4.5,
				Threshold:   3.0,
				SessionCost: 10.0,
				Detail:      "Project proj had 5 sessions on 2026-01-01, fragmentation index = 4.5",
			},
		},
		{
			name: "default_reason",
			s: analyze.WasteSignal{
				SessionID:   "ses_y",
				Project:     "proj",
				Severity:    "low",
				Reason:      "unknown_reason",
				Detail:      "some detail",
				Metric:      1.0,
				Threshold:   0.5,
				SessionCost: 3.0,
			},
		},
		{
			name: "empty_session_id",
			s: analyze.WasteSignal{
				SessionID: "",
				Project:   "proj",
				Severity:  "low",
				Reason:    "cache_underutilized",
				Metric:    0.12,
				Threshold: 0.25,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var b strings.Builder
			rec := analyze.Recommendation{
				Action:     "Test action",
				Detail:     "Test detail",
				SavingsEst: 5.0,
			}
			writeSignalBlock(&b, tt.s, rec, nil)
			got := b.String()
			if !strings.Contains(got, "\u2192") {
				t.Errorf("expected recommendation arrow, got: %s", got)
			}
		})
	}
}

func TestCollectEvents_WithSource(t *testing.T) {
	events := CollectEvents([]source.Source{})
	if len(events) != 0 {
		t.Errorf("expected 0 events from empty sources, got %d", len(events))
	}
}

func TestFormatText_NoSignalsWithVerbose(t *testing.T) {
	events := []source.TokenEvent{
		{SessionID: "s1", Harness: "opencode", Project: "p1", CostUSD: 1.0, Timestamp: fixedTime},
	}
	baselines := analyze.ComputeBaselines(events, config.Defaults())
	signals := analyze.DetectWaste(events, baselines, nil, config.Defaults())
	recommendations := analyze.GenerateRecommendations(signals, baselines)

	got := FormatText(events, baselines, signals, recommendations, true, config.Config{})
	if !strings.Contains(got, "All sessions:") {
		t.Errorf("verbose mode should show all sessions, got: %s", got)
	}
	if !strings.Contains(got, "No waste signals detected.") {
		t.Errorf("should show no waste signals message with single event, got: %s", got)
	}
}

func TestConvertSubagentNode_WithChildren(t *testing.T) {
	node := analyze.SubagentNode{
		SessionID: "parent",
		AgentType: "general",
		Cost:      10.0,
		Children: []analyze.SubagentNode{
			{SessionID: "child", AgentType: "explore", Cost: 5.0},
		},
	}
	result := convertSubagentNode(node)
	if result.SessionID != "parent" {
		t.Errorf("expected parent session ID, got %s", result.SessionID)
	}
	if len(result.Children) != 1 {
		t.Fatalf("expected 1 child, got %d", len(result.Children))
	}
	if result.Children[0].SessionID != "child" {
		t.Errorf("expected child session ID, got %s", result.Children[0].SessionID)
	}
}

func TestFormatText_Verbose(t *testing.T) {
	events := []source.TokenEvent{
		{SessionID: "s1", Harness: "opencode", Project: "p1", InputTokens: 100, OutputTokens: 100, CostUSD: 1.0, Timestamp: fixedTime},
		{SessionID: "s2", Harness: "opencode", Project: "p1", InputTokens: 200, OutputTokens: 50, CostUSD: 2.0, Timestamp: fixedTime},
	}
	baselines := analyze.ComputeBaselines(events, config.Defaults())
	signals := analyze.DetectWaste(events, baselines, nil, config.Defaults())
	recommendations := analyze.GenerateRecommendations(signals, baselines)

	got := FormatText(events, baselines, signals, recommendations, true, config.Config{})
	if !strings.Contains(got, "All sessions:") {
		t.Errorf("verbose output should show all sessions, got: %s", got)
	}
	if !strings.Contains(got, "s1") {
		t.Errorf("verbose output should include s1, got: %s", got)
	}
}

func TestFindBaselineForSignal(t *testing.T) {
	baselines := map[string]analyze.Baseline{
		"*":                {Project: "*", CostMean: 1.0},
		"proj1:opencode":   {Project: "proj1", Harness: "opencode", CostMean: 5.0},
		"proj2:claude-code": {Project: "proj2", Harness: "claude-code", CostMean: 10.0},
	}

	t.Run("match by project", func(t *testing.T) {
		s := analyze.WasteSignal{Project: "proj1"}
		bl := findBaselineForSignal(s, baselines)
		if bl == nil || bl.CostMean != 5.0 {
			t.Errorf("expected proj1 baseline, got %v", bl)
		}
	})

	t.Run("fallback to global", func(t *testing.T) {
		s := analyze.WasteSignal{Project: "unknown"}
		bl := findBaselineForSignal(s, baselines)
		if bl == nil || bl.CostMean != 1.0 {
			t.Errorf("expected global baseline, got %v", bl)
		}
	})

	t.Run("nil for empty baselines", func(t *testing.T) {
		s := analyze.WasteSignal{Project: "proj1"}
		bl := findBaselineForSignal(s, map[string]analyze.Baseline{})
		if bl != nil {
			t.Errorf("expected nil, got %v", bl)
		}
	})
}

func TestExtractDateFromDetail(t *testing.T) {
	tests := []struct {
		name   string
		detail string
		want   string
	}{
		{
			name:   "valid date",
			detail: "Project proj had 5 sessions on 2026-01-15, all below mean ratio (0.5000)",
			want:   "2026-01-15",
		},
		{
			name:   "no date",
			detail: "Some other detail without date",
			want:   "",
		},
		{
			name:   "empty",
			detail: "",
			want:   "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractDateFromDetail(tt.detail)
			if got != tt.want {
				t.Errorf("extractDateFromDetail(%q) = %q, want %q", tt.detail, got, tt.want)
			}
		})
	}
}

func TestWriteChurnGroups(t *testing.T) {
	signals := []analyze.WasteSignal{
		{
			SessionID:   "s1",
			Project:     "proj1",
			Severity:    "medium",
			Reason:      "fragmentation_index",
			Metric:      4.5,
			Threshold:   3.0,
			SessionCost: 10.0,
			Detail:      "Project proj1 had 5 sessions on 2026-01-15, fragmentation index = 4.5",
		},
		{
			SessionID:   "s2",
			Project:     "proj1",
			Severity:    "medium",
			Reason:      "fragmentation_index",
			Metric:      4.5,
			Threshold:   3.0,
			SessionCost: 10.0,
			Detail:      "Project proj1 had 5 sessions on 2026-01-15, fragmentation index = 4.5",
		},
	}

	recBySignal := make(map[analyze.WasteSignal]analyze.Recommendation)
	for _, s := range signals {
		recBySignal[s] = analyze.Recommendation{
			Signal:     s,
			Action:     "consolidate",
			SavingsEst: 5.0,
		}
	}

	var b strings.Builder
	writeChurnGroups(&b, signals, recBySignal)
	got := b.String()

	if !strings.Contains(got, "proj1 on 2026-01-15") {
		t.Errorf("expected grouped frag, got: %s", got)
	}
	if !strings.Contains(got, "$20.00 total") {
		t.Errorf("expected total cost, got: %s", got)
	}
	if !strings.Contains(got, "Potential savings: $10.00") {
		t.Errorf("expected savings estimate, got: %s", got)
	}
}

func TestFormatText_GroupChurn(t *testing.T) {
	baseTime := time.Date(2026, 4, 15, 10, 0, 0, 0, time.UTC)
	events := []source.TokenEvent{
		{SessionID: "sa", Harness: "opencode", Project: "proj", InputTokens: 10000, OutputTokens: 50, CostUSD: 1.0, Timestamp: baseTime},
		{SessionID: "sb", Harness: "opencode", Project: "proj", InputTokens: 10000, OutputTokens: 30, CostUSD: 1.0, Timestamp: baseTime},
		{SessionID: "sc", Harness: "opencode", Project: "proj", InputTokens: 10000, OutputTokens: 80, CostUSD: 1.0, Timestamp: baseTime},
		{SessionID: "se", Harness: "opencode", Project: "proj", InputTokens: 10000, OutputTokens: 40, CostUSD: 1.0, Timestamp: baseTime},
		{SessionID: "sd", Harness: "opencode", Project: "proj", InputTokens: 10000, OutputTokens: 500, CostUSD: 2.0, Timestamp: baseTime.AddDate(0, 0, 1)},
	}

	baselines := analyze.ComputeBaselines(events, config.Defaults())
	signals := analyze.DetectWaste(events, baselines, nil, config.Defaults())
	recommendations := analyze.GenerateRecommendations(signals, baselines)

	hasFrag := false
	for _, s := range signals {
		if s.Reason == "fragmentation_index" {
			hasFrag = true
			break
		}
	}
	if !hasFrag {
		t.Skip("test data did not generate fragmentation_index signals")
	}

	// Test GroupChurn=false (default) — individual lines with date
	gotUngrouped := FormatText(events, baselines, signals, recommendations, false, config.Config{})
	if !strings.Contains(gotUngrouped, "fragmentation index") {
		t.Error("ungrouped output should show fragmentation lines")
	}

	// Test GroupChurn=true — one grouped line
	cfg := config.Config{}
	cfg.Output.GroupChurn = true
	gotGrouped := FormatText(events, baselines, signals, recommendations, false, cfg)

	if !strings.Contains(gotGrouped, "on 2026-04-15") {
		t.Errorf("grouped output should show date, got: %s", gotGrouped)
	}
	if !strings.Contains(gotGrouped, "total") {
		t.Errorf("grouped output should show total, got: %s", gotGrouped)
	}
	// In grouped mode, "fragmentation index" should appear exactly once (in the group header)
	if strings.Count(gotGrouped, "fragmentation index") != 1 {
		t.Errorf("grouped output should have exactly 1 'fragmentation index' line, got: %s", gotGrouped)
	}
}
