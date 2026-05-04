package config

import (
	"os"
	"strings"
	"testing"
)

func TestDefaults(t *testing.T) {
	cfg := Defaults()

	if cfg.Thresholds.CostOutlierSigma != 2.0 {
		t.Errorf("CostOutlierSigma = %f, want 2.0", cfg.Thresholds.CostOutlierSigma)
	}
	if cfg.Thresholds.LowSignalPercentile != 10.0 {
		t.Errorf("LowSignalPercentile = %f, want 10.0", cfg.Thresholds.LowSignalPercentile)
	}
	if cfg.Thresholds.CachePercentile != 10.0 {
		t.Errorf("CachePercentile = %f, want 10.0", cfg.Thresholds.CachePercentile)
	}
	if cfg.Thresholds.SubagentOverheadPct != 50.0 {
		t.Errorf("SubagentOverheadPct = %f, want 50.0", cfg.Thresholds.SubagentOverheadPct)
	}
	if cfg.Thresholds.ChurnMinSessions != 3 {
		t.Errorf("ChurnMinSessions = %d, want 3", cfg.Thresholds.ChurnMinSessions)
	}
	if cfg.Thresholds.FragmentationIndexThreshold != 3.0 {
		t.Errorf("FragmentationIndexThreshold = %f, want 3.0", cfg.Thresholds.FragmentationIndexThreshold)
	}
	if cfg.Thresholds.InputOverconsumptionSigma != 2.0 {
		t.Errorf("InputOverconsumptionSigma = %f, want 2.0", cfg.Thresholds.InputOverconsumptionSigma)
	}
	if cfg.Thresholds.OutputExplosionSigma != 2.0 {
		t.Errorf("OutputExplosionSigma = %f, want 2.0", cfg.Thresholds.OutputExplosionSigma)
	}
	if cfg.Thresholds.TokenEfficiencyPercentile != 10.0 {
		t.Errorf("TokenEfficiencyPercentile = %f, want 10.0", cfg.Thresholds.TokenEfficiencyPercentile)
	}
	if cfg.Thresholds.FragmentationMinCost != 0.50 {
		t.Errorf("FragmentationMinCost = %f, want 0.50", cfg.Thresholds.FragmentationMinCost)
	}

	checkSignals(t, cfg.Signals)
	checkFilters(t, cfg.Filters)
	checkOutput(t, cfg.Output)
}

func checkSignals(t *testing.T, s Signals) {
	t.Helper()
	if !s.CostOutlier {
		t.Error("CostOutlier should default to true")
	}
	if !s.LowSignal {
		t.Error("LowSignal should default to true")
	}
	if !s.SubagentOverhead {
		t.Error("SubagentOverhead should default to true")
	}
	if !s.CacheUnderutilized {
		t.Error("CacheUnderutilized should default to true")
	}
	if !s.FragmentationIndex {
		t.Error("FragmentationIndex should default to true")
	}
	if !s.InputOverconsumption {
		t.Error("InputOverconsumption should default to true")
	}
	if !s.OutputExplosion {
		t.Error("OutputExplosion should default to true")
	}
	if !s.TokenEfficiency {
		t.Error("TokenEfficiency should default to true")
	}
	if !s.ToolLoop {
		t.Error("ToolLoop should default to true")
	}
	if !s.FileReread {
		t.Error("FileReread should default to true")
	}
	if !s.SubagentOverlap {
		t.Error("SubagentOverlap should default to true")
	}
	if !s.SessionRestart {
		t.Error("SessionRestart should default to true")
	}
}

func checkFilters(t *testing.T, f Filters) {
	t.Helper()
	if f.MinCost != 0 {
		t.Errorf("MinCost = %f, want 0", f.MinCost)
	}
	if f.Deduplicate {
		t.Error("Deduplicate should default to false")
	}
}

func checkOutput(t *testing.T, o Output) {
	t.Helper()
	if o.GroupChurn {
		t.Error("GroupChurn should default to false")
	}
	if o.ShowTrends {
		t.Error("ShowTrends should default to false")
	}
}

func TestLoad_NoFile(t *testing.T) {
	cfg, err := Load("")
	if err != nil {
		t.Fatalf("expected no error with empty path, got: %v", err)
	}

	def := Defaults()
	if cfg.Thresholds.CostOutlierSigma != def.Thresholds.CostOutlierSigma {
		t.Error("config should match defaults when no file found")
	}
}

func TestLoad_FileNotFound(t *testing.T) {
	_, err := Load("/nonexistent/path/burnwatch.toml")
	if err == nil {
		t.Fatal("expected error for nonexistent file with explicit path")
	}
}

func TestLoad_ValidTOML(t *testing.T) {
	content := `
[thresholds]
cost_outlier_sigma = 3.0
low_signal_percentile = 15.0
cache_percentile = 20.0
subagent_overhead_pct = 75.0
churn_min_sessions = 5
fragmentation_index_threshold = 4.0
input_overconsumption_sigma = 3.0
output_explosion_sigma = 2.5
token_efficiency_percentile = 25.0

[signals]
cost_outlier = false
cache_underutilized = false

[filters]
min_cost = 0.01
deduplicate = true

[output]
group_churn = true
`
	fp := writeTempFile(t, content)
	defer func() { _ = os.Remove(fp) }()

	cfg, err := Load(fp)
	if err != nil {
		t.Fatalf("expected valid TOML to load, got: %v", err)
	}

	if cfg.Thresholds.CostOutlierSigma != 3.0 {
		t.Errorf("CostOutlierSigma = %f, want 3.0", cfg.Thresholds.CostOutlierSigma)
	}
	if cfg.Thresholds.LowSignalPercentile != 15.0 {
		t.Errorf("LowSignalPercentile = %f, want 15.0", cfg.Thresholds.LowSignalPercentile)
	}
	if cfg.Thresholds.CachePercentile != 20.0 {
		t.Errorf("CachePercentile = %f, want 20.0", cfg.Thresholds.CachePercentile)
	}
	if cfg.Thresholds.SubagentOverheadPct != 75.0 {
		t.Errorf("SubagentOverheadPct = %f, want 75.0", cfg.Thresholds.SubagentOverheadPct)
	}
	if cfg.Thresholds.ChurnMinSessions != 5 {
		t.Errorf("ChurnMinSessions = %d, want 5", cfg.Thresholds.ChurnMinSessions)
	}
	if cfg.Thresholds.FragmentationIndexThreshold != 4.0 {
		t.Errorf("FragmentationIndexThreshold = %f, want 4.0", cfg.Thresholds.FragmentationIndexThreshold)
	}
	if cfg.Thresholds.InputOverconsumptionSigma != 3.0 {
		t.Errorf("InputOverconsumptionSigma = %f, want 3.0", cfg.Thresholds.InputOverconsumptionSigma)
	}
	if cfg.Thresholds.OutputExplosionSigma != 2.5 {
		t.Errorf("OutputExplosionSigma = %f, want 2.5", cfg.Thresholds.OutputExplosionSigma)
	}
	if cfg.Thresholds.TokenEfficiencyPercentile != 25.0 {
		t.Errorf("TokenEfficiencyPercentile = %f, want 25.0", cfg.Thresholds.TokenEfficiencyPercentile)
	}
	if cfg.Signals.CostOutlier {
		t.Error("CostOutlier should be false")
	}
	if cfg.Signals.CacheUnderutilized {
		t.Error("CacheUnderutilized should be false")
	}
	if !cfg.Signals.LowSignal {
		t.Error("LowSignal should remain default true")
	}
	if cfg.Filters.MinCost != 0.01 {
		t.Errorf("MinCost = %f, want 0.01", cfg.Filters.MinCost)
	}
	if !cfg.Filters.Deduplicate {
		t.Error("Deduplicate should be true")
	}
	if !cfg.Output.GroupChurn {
		t.Error("GroupChurn should be true")
	}
}

func TestLoad_InvalidTOML(t *testing.T) {
	fp := writeTempFile(t, "this = is not valid toml [[[")
	defer func() { _ = os.Remove(fp) }()

	_, err := Load(fp)
	if err == nil {
		t.Fatal("expected parse error for invalid TOML")
	}
}

func TestLoad_PartialOverride(t *testing.T) {
	content := `
[thresholds]
cost_outlier_sigma = 5.0
`
	fp := writeTempFile(t, content)
	defer func() { _ = os.Remove(fp) }()

	cfg, err := Load(fp)
	if err != nil {
		t.Fatalf("expected load to succeed, got: %v", err)
	}

	if cfg.Thresholds.CostOutlierSigma != 5.0 {
		t.Errorf("CostOutlierSigma = %f, want 5.0", cfg.Thresholds.CostOutlierSigma)
	}

	def := Defaults()
	if cfg.Thresholds.LowSignalPercentile != def.Thresholds.LowSignalPercentile {
		t.Error("unmodified fields should retain defaults")
	}
	if !cfg.Signals.CostOutlier {
		t.Error("unmodified signal should retain default")
	}
}

func TestValidate_NegativeSigma(t *testing.T) {
	cfg := Defaults()
	cfg.Thresholds.CostOutlierSigma = -1.0

	if err := Validate(cfg); err == nil {
		t.Fatal("expected error for negative sigma")
	} else if !strings.Contains(err.Error(), "sigma") {
		t.Errorf("error should mention sigma, got: %v", err)
	}
}

func TestValidate_ZeroSigma(t *testing.T) {
	cfg := Defaults()
	cfg.Thresholds.CostOutlierSigma = 0

	if err := Validate(cfg); err == nil {
		t.Fatal("expected error for zero sigma")
	}
}

func TestValidate_BadPercentile(t *testing.T) {
	tests := []struct {
		name string
		val  float64
	}{
		{"zero", 0},
		{"hundred", 100},
		{"negative", -1},
		{"above_hundred", 101},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Defaults()
			cfg.Thresholds.LowSignalPercentile = tt.val

			if err := Validate(cfg); err == nil {
				t.Errorf("expected error for percentile %f", tt.val)
			} else if !strings.Contains(err.Error(), "percentile") {
				t.Errorf("error should mention percentile, got: %v", err)
			}
		})
	}
}

func TestValidate_NegativeMinCost(t *testing.T) {
	cfg := Defaults()
	cfg.Filters.MinCost = -0.01

	if err := Validate(cfg); err == nil {
		t.Fatal("expected error for negative min_cost")
	} else if !strings.Contains(err.Error(), "min_cost") {
		t.Errorf("error should mention min_cost, got: %v", err)
	}
}

func TestValidate_CachePercentile(t *testing.T) {
	tests := []struct {
		name string
		val  float64
	}{
		{"zero", 0},
		{"hundred", 100},
		{"above_hundred", 101},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Defaults()
			cfg.Thresholds.CachePercentile = tt.val
			if err := Validate(cfg); err == nil {
				t.Errorf("expected error for cache_percentile %f", tt.val)
			} else if !strings.Contains(err.Error(), "cache_percentile") {
				t.Errorf("error should mention cache_percentile, got: %v", err)
			}
		})
	}
}

func TestValidate_SubagentOverheadPct(t *testing.T) {
	tests := []struct {
		name string
		val  float64
	}{
		{"zero", 0},
		{"hundred", 100},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Defaults()
			cfg.Thresholds.SubagentOverheadPct = tt.val
			if err := Validate(cfg); err == nil {
				t.Errorf("expected error for subagent_overhead_pct %f", tt.val)
			} else if !strings.Contains(err.Error(), "subagent_overhead_pct") {
				t.Errorf("error should mention subagent_overhead_pct, got: %v", err)
			}
		})
	}
}

func TestValidate_ChurnMinSessions(t *testing.T) {
	cfg := Defaults()
	cfg.Thresholds.ChurnMinSessions = 0
	if err := Validate(cfg); err == nil {
		t.Fatal("expected error for churn_min_sessions = 0")
	} else if !strings.Contains(err.Error(), "churn_min_sessions") {
		t.Errorf("error should mention churn_min_sessions, got: %v", err)
	}
}

func TestValidate_FragmentationIndexThreshold(t *testing.T) {
	cfg := Defaults()
	cfg.Thresholds.FragmentationIndexThreshold = 0
	if err := Validate(cfg); err == nil {
		t.Fatal("expected error for fragmentation_index_threshold = 0")
	} else if !strings.Contains(err.Error(), "fragmentation_index_threshold") {
		t.Errorf("error should mention fragmentation_index_threshold, got: %v", err)
	}
}

func TestValidate_InputOverconsumptionSigma(t *testing.T) {
	cfg := Defaults()
	cfg.Thresholds.InputOverconsumptionSigma = 0
	if err := Validate(cfg); err == nil {
		t.Fatal("expected error for input_overconsumption_sigma = 0")
	} else if !strings.Contains(err.Error(), "input_overconsumption_sigma") {
		t.Errorf("error should mention input_overconsumption_sigma, got: %v", err)
	}
}

func TestValidate_OutputExplosionSigma(t *testing.T) {
	cfg := Defaults()
	cfg.Thresholds.OutputExplosionSigma = 0
	if err := Validate(cfg); err == nil {
		t.Fatal("expected error for output_explosion_sigma = 0")
	} else if !strings.Contains(err.Error(), "output_explosion_sigma") {
		t.Errorf("error should mention output_explosion_sigma, got: %v", err)
	}
}

func TestValidate_TokenEfficiencyPercentile(t *testing.T) {
	tests := []struct {
		name string
		val  float64
	}{
		{"zero", 0},
		{"hundred", 100},
		{"above_hundred", 101},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Defaults()
			cfg.Thresholds.TokenEfficiencyPercentile = tt.val
			if err := Validate(cfg); err == nil {
				t.Errorf("expected error for token_efficiency_percentile %f", tt.val)
			} else if !strings.Contains(err.Error(), "token_efficiency_percentile") {
				t.Errorf("error should mention token_efficiency_percentile, got: %v", err)
			}
		})
	}
}

func TestLoad_SearchOrder(t *testing.T) {
	cfgWithPath, err := Load("./testdata/sample.toml")
	if err != nil {
		t.Skipf("testdata not set up, skipping: %v", err)
	}
	_ = cfgWithPath
}

func TestLoad_SearchOrderExplicit(t *testing.T) {
	content := `
[thresholds]
cost_outlier_sigma = 7.0
`
	fp := writeTempFile(t, content)
	defer func() { _ = os.Remove(fp) }()

	cfg, err := Load(fp)
	if err != nil {
		t.Fatalf("explicit path should succeed: %v", err)
	}
	if cfg.Thresholds.CostOutlierSigma != 7.0 {
		t.Errorf("CostOutlierSigma = %f, want 7.0", cfg.Thresholds.CostOutlierSigma)
	}
}

func TestWriteDefault(t *testing.T) {
	tmpDir := t.TempDir()
	path := tmpDir + "/.burnwatch.toml"

	if err := WriteDefault(path); err != nil {
		t.Fatalf("WriteDefault failed: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read written config: %v", err)
	}

	if !strings.Contains(string(data), "cost_outlier_sigma") {
		t.Error("written config should contain cost_outlier_sigma")
	}
	if !strings.Contains(string(data), "fragmentation_min_cost") {
		t.Error("written config should contain fragmentation_min_cost")
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load written config: %v", err)
	}
	if cfg.Thresholds.FragmentationMinCost != 0.50 {
		t.Errorf("FragmentationMinCost = %f, want 0.50", cfg.Thresholds.FragmentationMinCost)
	}
}

func TestWriteDefault_DirNotFound(t *testing.T) {
	err := WriteDefault("/nonexistent/dir/config.toml")
	if err == nil {
		t.Fatal("expected error for nonexistent directory")
	}
}

func TestLoad_EmptyStringResolves(t *testing.T) {
	cfg, err := Load("")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if cfg.Thresholds.FragmentationMinCost != 0.50 {
		t.Errorf("expected default FragmentationMinCost, got %f", cfg.Thresholds.FragmentationMinCost)
	}
}

func TestValidate_ToolLoopMaxRepeats(t *testing.T) {
	cfg := Defaults()
	cfg.Thresholds.ToolLoopMaxRepeats = 1
	if err := Validate(cfg); err == nil {
		t.Error("expected error for tool_loop_max_repeats < 2")
	}
	cfg.Thresholds.ToolLoopMaxRepeats = 2
	if err := Validate(cfg); err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
}

func TestValidate_FileRereadMinCount(t *testing.T) {
	cfg := Defaults()
	cfg.Thresholds.FileRereadMinCount = 1
	if err := Validate(cfg); err == nil {
		t.Error("expected error for file_reread_min_count < 2")
	}
	cfg.Thresholds.FileRereadMinCount = 2
	if err := Validate(cfg); err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
}

func TestValidate_DefaultBehavioralThresholds(t *testing.T) {
	cfg := Defaults()
	if cfg.Thresholds.ToolLoopMaxRepeats != 5 {
		t.Errorf("expected ToolLoopMaxRepeats=5, got %d", cfg.Thresholds.ToolLoopMaxRepeats)
	}
	if cfg.Thresholds.FileRereadMinCount != 3 {
		t.Errorf("expected FileRereadMinCount=3, got %d", cfg.Thresholds.FileRereadMinCount)
	}
	if !cfg.Signals.ToolLoop {
		t.Error("expected ToolLoop default true")
	}
	if !cfg.Signals.FileReread {
		t.Error("expected FileReread default true")
	}
}

func TestLoad_BehavioralThresholds(t *testing.T) {
	path := writeTempFile(t, `
[thresholds]
tool_loop_max_repeats = 7
file_reread_min_count = 5

[signals]
tool_loop = true
file_reread = true
`)
	defer func() { _ = os.Remove(path) }()

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if cfg.Thresholds.ToolLoopMaxRepeats != 7 {
		t.Errorf("expected ToolLoopMaxRepeats=7, got %d", cfg.Thresholds.ToolLoopMaxRepeats)
	}
	if cfg.Thresholds.FileRereadMinCount != 5 {
		t.Errorf("expected FileRereadMinCount=5, got %d", cfg.Thresholds.FileRereadMinCount)
	}
	if !cfg.Signals.ToolLoop {
		t.Error("expected ToolLoop=true")
	}
	if !cfg.Signals.FileReread {
		t.Error("expected FileReread=true")
	}
}

func writeTempFile(t *testing.T, content string) string {
	t.Helper()
	f, err := os.CreateTemp("", "burnwatch-config-test-*.toml")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(content); err != nil {
		_ = f.Close()
		t.Fatal(err)
	}
	_ = f.Close()
	return f.Name()
}


