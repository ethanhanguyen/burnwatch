package cmd

import (
	"os"
	"path/filepath"
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
	signals := analyze.DetectWaste(events, baselines)

	if len(signals) == 0 {
		t.Log("no waste signals found from test data (may be expected for clean data)")
	}

	_ = analyze.GenerateRecommendations(signals, baselines)

	text := output.FormatText(events, baselines, signals, nil, false, config.Config{})
	if text == "" {
		t.Error("expected non-empty text output")
	}

	// No panic with JSON flag equivalent
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
	// Filter by days uses time.Now().AddDate(0, 0, -days) as the cutoff.
	// Events with timestamps older than the cutoff are excluded.
	// Use sufficiently old events that they would be filtered out by a 1-day window.
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
	signals := analyze.DetectWaste(events, baselines)

	if len(signals) != 0 {
		t.Errorf("expected 0 signals for empty events, got %d", len(signals))
	}
}
