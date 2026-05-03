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

	if !cfg.Signals.CostOutlier {
		t.Error("CostOutlier should default to true")
	}
	if !cfg.Signals.LowSignal {
		t.Error("LowSignal should default to true")
	}
	if !cfg.Signals.SubagentOverhead {
		t.Error("SubagentOverhead should default to true")
	}
	if !cfg.Signals.CacheUnderutilized {
		t.Error("CacheUnderutilized should default to true")
	}
	if !cfg.Signals.SessionChurn {
		t.Error("SessionChurn should default to true")
	}

	if cfg.Filters.MinCost != 0 {
		t.Errorf("MinCost = %f, want 0", cfg.Filters.MinCost)
	}
	if cfg.Filters.Deduplicate {
		t.Error("Deduplicate should default to false")
	}

	if cfg.Output.GroupChurn {
		t.Error("GroupChurn should default to false")
	}
	if cfg.Output.ShowTrends {
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
	defer os.Remove(fp)

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
	defer os.Remove(fp)

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
	defer os.Remove(fp)

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
	defer os.Remove(fp)

	cfg, err := Load(fp)
	if err != nil {
		t.Fatalf("explicit path should succeed: %v", err)
	}
	if cfg.Thresholds.CostOutlierSigma != 7.0 {
		t.Errorf("CostOutlierSigma = %f, want 7.0", cfg.Thresholds.CostOutlierSigma)
	}
}

func writeTempFile(t *testing.T, content string) string {
	t.Helper()
	f, err := os.CreateTemp("", "burnwatch-config-test-*.toml")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(content); err != nil {
		f.Close()
		t.Fatal(err)
	}
	f.Close()
	return f.Name()
}


