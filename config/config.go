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
	ChurnThreshold               float64 `toml:"churn_threshold"`
	FragmentationIndexThreshold  float64 `toml:"fragmentation_index_threshold"`
	InputOverconsumptionSigma    float64 `toml:"input_overconsumption_sigma"`
	OutputExplosionSigma         float64 `toml:"output_explosion_sigma"`
	TokenEfficiencyPercentile    float64 `toml:"token_efficiency_percentile"`
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
			ChurnThreshold:               2.0,
			FragmentationIndexThreshold:  3.0,
			InputOverconsumptionSigma:    2.0,
			OutputExplosionSigma:         2.0,
			TokenEfficiencyPercentile:    10.0,
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
	return nil
}
