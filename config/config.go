package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

type Config struct {
	Thresholds Thresholds
	Signals    Signals
	Filters    Filters
	Output     Output
}

type Thresholds struct {
	CostOutlierSigma             float64 `toml:"cost_outlier_sigma"`
	LowSignalPercentile          float64 `toml:"low_signal_percentile"`
	CachePercentile              float64 `toml:"cache_percentile"`
	SubagentOverheadPct          float64 `toml:"subagent_overhead_pct"`
	ChurnMinSessions             int     `toml:"churn_min_sessions"`
	FragmentationIndexThreshold  float64 `toml:"fragmentation_index_threshold"`
	InputOverconsumptionSigma    float64 `toml:"input_overconsumption_sigma"`
	OutputExplosionSigma         float64 `toml:"output_explosion_sigma"`
	TokenEfficiencyPercentile    float64 `toml:"token_efficiency_percentile"`
	FragmentationMinCost         float64 `toml:"fragmentation_min_cost"`
	ToolLoopMaxRepeats           int     `toml:"tool_loop_max_repeats"`
	FileRereadMinCount           int     `toml:"file_reread_min_count"`
}

type Signals struct {
	CostOutlier          bool `toml:"cost_outlier"`
	LowSignal            bool `toml:"low_signal"`
	SubagentOverhead     bool `toml:"subagent_overhead"`
	CacheUnderutilized   bool `toml:"cache_underutilized"`
	FragmentationIndex   bool `toml:"fragmentation_index"`
	InputOverconsumption bool `toml:"input_overconsumption"`
	OutputExplosion      bool `toml:"output_explosion"`
	TokenEfficiency      bool `toml:"token_efficiency"`
	ToolLoop             bool `toml:"tool_loop"`
	FileReread           bool `toml:"file_reread"`
	SubagentOverlap      bool `toml:"subagent_overlap"`
	SessionRestart       bool `toml:"session_restart"`
}

type Filters struct {
	MinCost     float64 `toml:"min_cost"`
	Deduplicate bool    `toml:"deduplicate"`
}

type Output struct {
	GroupChurn bool `toml:"group_churn"`
	ShowTrends bool `toml:"show_trends"`
}

func Defaults() Config {
	return Config{
		Thresholds: Thresholds{
			CostOutlierSigma:             2.0,
			LowSignalPercentile:          10.0,
			CachePercentile:              10.0,
			SubagentOverheadPct:          50.0,
			ChurnMinSessions:             3,
			FragmentationIndexThreshold:  3.0,
			InputOverconsumptionSigma:    2.0,
			OutputExplosionSigma:         2.0,
			TokenEfficiencyPercentile:    10.0,
			FragmentationMinCost:         0.50,
			ToolLoopMaxRepeats:           5,
			FileRereadMinCount:           3,
		},
		Signals: Signals{
			CostOutlier:          true,
			LowSignal:            true,
			SubagentOverhead:     true,
			CacheUnderutilized:   true,
			FragmentationIndex:   true,
			InputOverconsumption: true,
			OutputExplosion:      true,
			TokenEfficiency:      true,
		},
		Filters: Filters{
			MinCost:     0,
			Deduplicate: false,
		},
		Output: Output{
			GroupChurn: false,
			ShowTrends: false,
		},
	}
}

func Load(path string) (Config, error) {
	cfg := Defaults()

	if path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			return cfg, fmt.Errorf("config file %q: %w", path, err)
		}
		if err := toml.Unmarshal(data, &cfg); err != nil {
			return cfg, fmt.Errorf("config file %q: %w", path, err)
		}
		return cfg, nil
	}

	var resolved string
	home, err := os.UserHomeDir()
	if err == nil {
		candidates := []string{
			".burnwatch.toml",
			filepath.Join(home, ".config", "burnwatch", "config.toml"),
		}
		for _, cand := range candidates {
			if _, statErr := os.Stat(cand); statErr == nil {
				resolved = cand
				break
			}
		}
	}

	if resolved == "" {
		return cfg, nil
	}

	data, err := os.ReadFile(resolved)
	if err != nil {
		return cfg, nil
	}
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("config file %q: %w", resolved, err)
	}

	return cfg, nil
}

func WriteDefault(path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	_, err = f.WriteString(defaultTOML)
	return err
}

const defaultTOML = `# Burnwatch configuration
# All thresholds can be overridden here. See docs/quickstart.md for details.

[thresholds]
# Sensitivity for cost outlier detection (sigma from population mean).
# Higher = fewer flagged sessions. Lower = more false positives.
# Range: 1.0–4.0. Default 2.0.
cost_outlier_sigma = 2.0

# Low signal: flag sessions in bottom N% by total token volume.
# Range: 5–25. Default 10.
low_signal_percentile = 10.0

# Cache underutilization: flag sessions in bottom N% by cache-hit ratio.
# Range: 5–25. Default 10.
cache_percentile = 10.0

# Subagent overhead: flag if subagent cost exceeds N% of parent session.
# Range: 20–80. Default 50.
subagent_overhead_pct = 50.0

# Churn: minimum sessions required before churn analysis triggers.
# Range: 2–10. Default 3.
churn_min_sessions = 3

# Fragmentation index: flag sessions where subagent count / cost >= N.
# Higher = only severe fragmentation. Lower = fine-grained detection.
# Range: 0.5–10.0. Default 3.0.
fragmentation_index_threshold = 3.0

# Input overconsumption: flag sessions exceeding mean + N * stddev of input tokens.
# Same semantics as cost_outlier_sigma. Default 2.0.
input_overconsumption_sigma = 2.0

# Output explosion: flag sessions exceeding mean + N * stddev of output tokens.
# Default 2.0.
output_explosion_sigma = 2.0

# Token efficiency: flag sessions in bottom N% by TER (output/input ratio).
# Range: 5–25. Default 10.
token_efficiency_percentile = 10.0

# Fragmentation minimum cost (USD). Excludes trivially cheap sessions.
# Set to 0 to include all. Default 0.50.
fragmentation_min_cost = 0.50

# Tool loop: flag sessions with >= N consecutive identical tool calls.
# Range: 2–20. Default 5.
tool_loop_max_repeats = 5

# File re-read: flag files read >= N times without cache hits between reads.
# Range: 2–10. Default 3.
file_reread_min_count = 3

[signals]
# Enable/disable individual waste signals.
cost_outlier = true
low_signal = true
subagent_overhead = true
cache_underutilized = true
fragmentation_index = true
input_overconsumption = true
output_explosion = true
token_efficiency = true

# v3 behavioral signals (default off — opt in)
tool_loop = false
file_reread = false
subagent_overlap = false
session_restart = false

[filters]
# Exclude sessions costing less than this (USD). 0 = include all.
min_cost = 0
# Merge duplicate sessions from multiple harness runs.
deduplicate = false

[output]
# Group sessions by project/churn pattern in text output.
group_churn = false
# Show trend direction (↑↓→) in text output.
show_trends = false
`

func Validate(cfg Config) error {
	if cfg.Thresholds.CostOutlierSigma <= 0 {
		return fmt.Errorf("cost_outlier_sigma must be > 0, got %f", cfg.Thresholds.CostOutlierSigma)
	}
	if cfg.Thresholds.LowSignalPercentile <= 0 || cfg.Thresholds.LowSignalPercentile >= 100 {
		return fmt.Errorf("low_signal_percentile must be in (0, 100), got %f", cfg.Thresholds.LowSignalPercentile)
	}
	if cfg.Thresholds.CachePercentile <= 0 || cfg.Thresholds.CachePercentile >= 100 {
		return fmt.Errorf("cache_percentile must be in (0, 100), got %f", cfg.Thresholds.CachePercentile)
	}
	if cfg.Thresholds.SubagentOverheadPct <= 0 || cfg.Thresholds.SubagentOverheadPct >= 100 {
		return fmt.Errorf("subagent_overhead_pct must be in (0, 100), got %f", cfg.Thresholds.SubagentOverheadPct)
	}
	if cfg.Thresholds.ChurnMinSessions < 1 {
		return fmt.Errorf("churn_min_sessions must be >= 1, got %d", cfg.Thresholds.ChurnMinSessions)
	}
	if cfg.Thresholds.FragmentationIndexThreshold <= 0 {
		return fmt.Errorf("fragmentation_index_threshold must be > 0, got %f", cfg.Thresholds.FragmentationIndexThreshold)
	}
	if cfg.Thresholds.InputOverconsumptionSigma <= 0 {
		return fmt.Errorf("input_overconsumption_sigma must be > 0, got %f", cfg.Thresholds.InputOverconsumptionSigma)
	}
	if cfg.Thresholds.OutputExplosionSigma <= 0 {
		return fmt.Errorf("output_explosion_sigma must be > 0, got %f", cfg.Thresholds.OutputExplosionSigma)
	}
	if cfg.Thresholds.TokenEfficiencyPercentile <= 0 || cfg.Thresholds.TokenEfficiencyPercentile >= 100 {
		return fmt.Errorf("token_efficiency_percentile must be in (0, 100), got %f", cfg.Thresholds.TokenEfficiencyPercentile)
	}
	if cfg.Filters.MinCost < 0 {
		return fmt.Errorf("min_cost must be >= 0, got %f", cfg.Filters.MinCost)
	}
	if cfg.Thresholds.ToolLoopMaxRepeats < 2 {
		return fmt.Errorf("tool_loop_max_repeats must be >= 2, got %d", cfg.Thresholds.ToolLoopMaxRepeats)
	}
	if cfg.Thresholds.FileRereadMinCount < 2 {
		return fmt.Errorf("file_reread_min_count must be >= 2, got %d", cfg.Thresholds.FileRereadMinCount)
	}
	return nil
}
