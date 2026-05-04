package cmd

import (
	"bytes"
	"flag"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ethanhanguyen/burnwatch/analyze"
	"github.com/ethanhanguyen/burnwatch/config"
	"github.com/ethanhanguyen/burnwatch/report"
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

	events := report.CollectEvents(sources)
	if len(events) == 0 {
		t.Fatal("expected events from test data, got none")
	}

	baselines := analyze.ComputeBaselines(events, config.Defaults())
	cfg := config.Defaults()
	signals := analyze.DetectWaste(events, baselines, nil, cfg)

	if len(signals) == 0 {
		t.Log("no waste signals found from test data (may be expected for clean data)")
	}

	_ = analyze.GenerateRecommendations(signals, baselines)

	text := report.FormatText(events, baselines, signals, nil, false, config.Config{})
	if text == "" {
		t.Error("expected non-empty text output")
	}

	trees := analyze.BuildSubagentTree(events)
	jsonBytes, err := report.FormatJSON(events, baselines, signals, nil, trees)
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
	baselines := analyze.ComputeBaselines(events, config.Defaults())
	cfg := config.Defaults()
	signals := analyze.DetectWaste(events, baselines, nil, cfg)

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

	events := report.CollectEvents(sources)
	baselines := analyze.ComputeBaselines(events, config.Defaults())

	allOff := config.Defaults()
	allOff.Signals = config.Signals{}
	signals := analyze.DetectWaste(events, baselines, nil, allOff)
	if len(signals) != 0 {
		t.Errorf("expected 0 signals with all toggles off, got %d", len(signals))
	}

	noFragCfg := config.Defaults()
	noFragCfg.Signals.FragmentationIndex = false
	signalsNoFrag := analyze.DetectWaste(events, baselines, nil, noFragCfg)
	for _, s := range signalsNoFrag {
		if s.Reason == "fragmentation_index" {
			t.Error("found fragmentation_index signal with toggle off")
		}
	}

	noCostCfg := config.Defaults()
	noCostCfg.Signals.CostOutlier = false
	signalsNoCost := analyze.DetectWaste(events, baselines, nil, noCostCfg)
	for _, s := range signalsNoCost {
		if s.Reason == "cost_outlier" {
			t.Error("found cost_outlier signal with toggle off")
		}
	}
}

func runExecute(args []string) (string, string) {
	origArgs := os.Args
	os.Args = append([]string{"burnwatch"}, args...)

	origStderr := os.Stderr
	origStdout := os.Stdout

	rOut, wOut, _ := os.Pipe()
	rErr, wErr, _ := os.Pipe()
	os.Stdout = wOut
	os.Stderr = wErr

	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ContinueOnError)

	Execute()

	_ = wOut.Close()
	_ = wErr.Close()

	var stdoutBuf, stderrBuf bytes.Buffer
	_, _ = io.Copy(&stdoutBuf, rOut)
	_, _ = io.Copy(&stderrBuf, rErr)

	os.Stdout = origStdout
	os.Stderr = origStderr
	os.Args = origArgs

	return stdoutBuf.String(), stderrBuf.String()
}

func TestExecute_Version(t *testing.T) {
	stdout, _ := runExecute([]string{"--version"})
	if !strings.Contains(stdout, "burnwatch") {
		t.Errorf("expected version output, got: %s", stdout)
	}
}

func TestExecute_Init(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	_ = os.Chdir(tmpDir)
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	stdout, _ := runExecute([]string{"--init"})
	if !strings.Contains(stdout, "Wrote .burnwatch.toml") {
		t.Errorf("expected init confirmation, got: %s", stdout)
	}

	data, err := os.ReadFile(".burnwatch.toml")
	if err != nil {
		t.Fatalf("expected .burnwatch.toml to exist: %v", err)
	}
	if !strings.Contains(string(data), "fragmentation_min_cost") {
		t.Error("generated config should contain fragmentation_min_cost")
	}
}

func TestExecute_PrintConfig(t *testing.T) {
	stdout, _ := runExecute([]string{"--print-config"})
	if !strings.Contains(stdout, "fragmentation_min_cost") {
		t.Errorf("expected config output, got: %s", stdout)
	}
}

func TestExecute_Calibrate(t *testing.T) {
	setupTestEnv(t)

	stdout, _ := runExecute([]string{"--calibrate", "--no-fetch-pricing"})
	if !strings.Contains(stdout, "Session costs ($):") {
		t.Errorf("expected calibration cost section, got: %s", stdout)
	}
	if !strings.Contains(stdout, "Suggested thresholds") {
		t.Errorf("expected suggested thresholds section, got: %s", stdout)
	}
}

func TestExecute_CalibrateJSON(t *testing.T) {
	setupTestEnv(t)

	stdout, _ := runExecute([]string{"--calibrate", "--json", "--no-fetch-pricing"})
	if !strings.Contains(stdout, `"total_sessions"`) {
		t.Errorf("expected JSON calibration output, got: %s", stdout)
	}
}

func TestTrendsOutput(t *testing.T) {
	events := []source.TokenEvent{
		{SessionID: "s1", CostUSD: 100, InputTokens: 1000, OutputTokens: 100, Timestamp: time.Date(2026, 5, 4, 10, 0, 0, 0, time.UTC)},
		{SessionID: "s2", CostUSD: 80, InputTokens: 800, OutputTokens: 80, Timestamp: time.Date(2026, 5, 11, 10, 0, 0, 0, time.UTC)},
	}

	baselines := analyze.ComputeBaselines(events, config.Defaults())
	cfg := config.Defaults()
	signals := analyze.DetectWaste(events, baselines, nil, cfg)
	recommendations := analyze.GenerateRecommendations(signals, baselines)

	cfg = config.Config{}
	cfg.Output.ShowTrends = true
	text := report.FormatText(events, baselines, signals, recommendations, false, cfg)

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
