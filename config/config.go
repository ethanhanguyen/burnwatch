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
	CostOutlierSigma    float64 `toml:"cost_outlier_sigma"`
	LowSignalPercentile float64 `toml:"low_signal_percentile"`
}

type Signals struct {
	CostOutlier        bool `toml:"cost_outlier"`
	LowSignal          bool `toml:"low_signal"`
	SubagentOverhead   bool `toml:"subagent_overhead"`
	CacheUnderutilized bool `toml:"cache_underutilized"`
	SessionChurn       bool `toml:"session_churn"`
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
			CostOutlierSigma:    2.0,
			LowSignalPercentile: 10.0,
		},
		Signals: Signals{
			CostOutlier:        true,
			LowSignal:          true,
			SubagentOverhead:   true,
			CacheUnderutilized: true,
			SessionChurn:       true,
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
	if cfg.Filters.MinCost < 0 {
		return fmt.Errorf("min_cost must be >= 0, got %f", cfg.Filters.MinCost)
	}
	return nil
}
