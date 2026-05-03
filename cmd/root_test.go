package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ethanhanguyen/burnwatch/analyze"
	"github.com/ethanhanguyen/burnwatch/config"
	"github.com/ethanhanguyen/burnwatch/output"
	"github.com/ethanhanguyen/burnwatch/source"
)

func today() time.Time {
	return time.Now().Truncate(24 * time.Hour)
}

func setupTestEnv(t *testing.T) {
	t.Helper()
	dbPath, err := filepath.Abs(filepath.Join("..", "testdata", "opencode_sample.db"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Skipf("sample DB not found at %s", dbPath)
	}
	t.Setenv("BURNWATCH_OPENCODE_DB", dbPath)
}

func TestEndToEnd(t *testing.T) {
	setupTestEnv(t)

	sources := source.Discover()
	if len(sources) == 0 {
		t.Fatal("no sources discovered")
	}

	events := output.CollectEvents(sources)
	if len(events) == 0 {
		t.Fatal("expected events from test data, got none")
	}

	baselines := analyze.ComputeBaselines(events)
	toggles := analyze.SignalToggles{
		CostOutlier:          true,
		LowSignal:            true,
		SubagentOverhead:     true,
		CacheUnderutilized:   true,
		FragmentationIndex:   true,
		InputOverconsumption: true,
		OutputExplosion:      true,
		TokenEfficiency:      true,
	}
	signals := analyze.DetectWaste(events, baselines, 2.0, 2.0, 2.0, 3.0, 3, toggles)

	if len(signals) == 0 {
		t.Log("no waste signals found from test data (may be expected for clean data)")
	}

	_ = analyze.GenerateRecommendations(signals, baselines)

	text := output.FormatText(events, baselines, signals, nil, false, config.Config{})
	if text == "" {
		t.Error("expected non-empty text output")
	}

	trees := analyze.BuildSubagentTree(events)
	jsonBytes, err := output.FormatJSON(events, baselines, signals, nil, trees)
	if err != nil {
		t.Fatal(err)
	}
	if len(jsonBytes) == 0 {
		t.Error("expected non-empty JSON output")
	}
}

func TestFilterByHarness(t *testing.T) {
	events := []source.TokenEvent{
		{SessionID: "s1", Harness: "opencode"},
		{SessionID: "s2", Harness: "claude-code"},
		{SessionID: "s3", Harness: "opencode"},
	}

	filtered := filterByHarness(events, "opencode")
	if len(filtered) != 2 {
		t.Errorf("expected 2 opencode events, got %d", len(filtered))
	}

	filtered = filterByHarness(events, "claude-code")
	if len(filtered) != 1 {
		t.Errorf("expected 1 claude-code event, got %d", len(filtered))
	}

	filtered = filterByHarness(events, "nonexistent")
	if len(filtered) != 0 {
		t.Errorf("expected 0 events for nonexistent harness, got %d", len(filtered))
	}
}

func TestFilterByProject(t *testing.T) {
	events := []source.TokenEvent{
		{SessionID: "s1", Project: "proj-a"},
		{SessionID: "s2", Project: "proj-b"},
		{SessionID: "s3", Project: "proj-a"},
	}

	filtered := filterByProject(events, "proj-a")
	if len(filtered) != 2 {
		t.Errorf("expected 2 proj-a events, got %d", len(filtered))
	}

	filtered = filterByProject(events, "proj-b")
	if len(filtered) != 1 {
		t.Errorf("expected 1 proj-b event, got %d", len(filtered))
	}

	filtered = filterByProject(events, "nonexistent")
	if len(filtered) != 0 {
		t.Errorf("expected 0 events for nonexistent project, got %d", len(filtered))
	}
}

func TestFilterByDays(t *testing.T) {
	events := []source.TokenEvent{
		{SessionID: "s1", Timestamp: today()},
		{SessionID: "s2", Timestamp: today().AddDate(0, 0, -1)},
	}

	filtered := filterByDays(events, 2)
	if len(filtered) != 2 {
		t.Errorf("expected 2 events within 2 days, got %d", len(filtered))
	}

	filtered = filterByDays(events, 0)
	if len(filtered) != 2 {
		t.Errorf("expected 2 events when days==0 (no filter), got %d", len(filtered))
	}
}

func TestZeroEvents(t *testing.T) {
	events := []source.TokenEvent{}
	baselines := analyze.ComputeBaselines(events)
	toggles := analyze.SignalToggles{
		CostOutlier:          true,
		LowSignal:            true,
		SubagentOverhead:     true,
		CacheUnderutilized:   true,
		FragmentationIndex:   true,
		InputOverconsumption: true,
		OutputExplosion:      true,
		TokenEfficiency:      true,
	}
	signals := analyze.DetectWaste(events, baselines, 2.0, 2.0, 2.0, 3.0, 3, toggles)

	if len(signals) != 0 {
		t.Errorf("expected 0 signals for empty events, got %d", len(signals))
	}
}

func TestTogglesSuppressOutput(t *testing.T) {
	setupTestEnv(t)

	sources := source.Discover()
	if len(sources) == 0 {
		t.Fatal("no sources discovered")
	}

	events := output.CollectEvents(sources)
	baselines := analyze.ComputeBaselines(events)

	allOff := analyze.SignalToggles{
		CostOutlier:          false,
		LowSignal:            false,
		SubagentOverhead:     false,
		CacheUnderutilized:   false,
		FragmentationIndex:   false,
		InputOverconsumption: false,
		OutputExplosion:      false,
		TokenEfficiency:      false,
	}
	signals := analyze.DetectWaste(events, baselines, 2.0, 2.0, 2.0, 3.0, 3, allOff)
	if len(signals) != 0 {
		t.Errorf("expected 0 signals with all toggles off, got %d", len(signals))
	}

	noFrag := analyze.SignalToggles{
		CostOutlier:          true,
		LowSignal:            true,
		SubagentOverhead:     true,
		CacheUnderutilized:   true,
		FragmentationIndex:   false,
		InputOverconsumption: true,
		OutputExplosion:      true,
		TokenEfficiency:      true,
	}
	signalsNoFrag := analyze.DetectWaste(events, baselines, 2.0, 2.0, 2.0, 3.0, 3, noFrag)
	for _, s := range signalsNoFrag {
		if s.Reason == "fragmentation_index" {
			t.Error("found fragmentation_index signal with toggle off")
		}
	}

	noCost := analyze.SignalToggles{
		CostOutlier:          false,
		LowSignal:            true,
		SubagentOverhead:     true,
		CacheUnderutilized:   true,
		FragmentationIndex:   true,
		InputOverconsumption: true,
		OutputExplosion:      true,
		TokenEfficiency:      true,
	}
	signalsNoCost := analyze.DetectWaste(events, baselines, 2.0, 2.0, 2.0, 3.0, 3, noCost)
	for _, s := range signalsNoCost {
		if s.Reason == "cost_outlier" {
			t.Error("found cost_outlier signal with toggle off")
		}
	}
}

func TestTrendsOutput(t *testing.T) {
	events := []source.TokenEvent{
		{SessionID: "s1", CostUSD: 100, InputTokens: 1000, OutputTokens: 100, Timestamp: time.Date(2026, 5, 4, 10, 0, 0, 0, time.UTC)},
		{SessionID: "s2", CostUSD: 80, InputTokens: 800, OutputTokens: 80, Timestamp: time.Date(2026, 5, 11, 10, 0, 0, 0, time.UTC)},
	}

	baselines := analyze.ComputeBaselines(events)
	toggles := analyze.SignalToggles{
		CostOutlier:          true,
		LowSignal:            true,
		SubagentOverhead:     true,
		CacheUnderutilized:   true,
		FragmentationIndex:   true,
		InputOverconsumption: true,
		OutputExplosion:      true,
		TokenEfficiency:      true,
	}
	signals := analyze.DetectWaste(events, baselines, 2.0, 2.0, 2.0, 3.0, 3, toggles)
	recommendations := analyze.GenerateRecommendations(signals, baselines)

	cfg := config.Config{}
	cfg.Output.ShowTrends = true
	text := output.FormatText(events, baselines, signals, recommendations, false, cfg)

	if !strings.Contains(text, "Trends:") {
		t.Error("expected trends section, got none")
	}
	if !strings.Contains(text, "Cost:") {
		t.Error("expected cost trend line")
	}
	if !strings.Contains(text, "Sessions:") {
		t.Error("expected sessions trend line")
	}
	if !strings.Contains(text, "Output/input ratio:") {
		t.Error("expected ratio trend line")
	}
}
