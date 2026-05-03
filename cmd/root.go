package cmd

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/ethanhanguyen/burnwatch/analyze"
	"github.com/ethanhanguyen/burnwatch/config"
	"github.com/ethanhanguyen/burnwatch/output"
	"github.com/ethanhanguyen/burnwatch/source"
)

var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

func Execute() {
	var flags struct {
		DBPath      string
		Harness     string
		Project     string
		JSON        bool
		Days        int
		Verbose     bool
		ConfigPath  string
		PrintConfig bool
		MinCost     float64

		NoCostOutlier       bool
		NoLowSignal         bool
		NoSubagentOverhead  bool
		NoCacheUnderutil    bool
		NoChurn             bool
		ShowTrends          bool

		RefreshPricing bool
		NoFetchPricing bool
	}

	flag.StringVar(&flags.DBPath, "db", "", "OpenCode database path")
	flag.StringVar(&flags.Harness, "harness", "all", "Filter to harness: all, opencode, claude-code")
	flag.StringVar(&flags.Project, "project", "", "Filter to project")
	flag.BoolVar(&flags.JSON, "json", false, "Output as JSON instead of text")
	flag.IntVar(&flags.Days, "days", 0, "Lookback window in days (default: 1 for today, 7 for week, 30 for month)")
	flag.BoolVar(&flags.Verbose, "verbose", false, "Show all events, not just waste signals")
	flag.StringVar(&flags.ConfigPath, "config", "", "Config file path (default: ./.burnwatch.toml, ~/.config/burnwatch/config.toml)")
	flag.BoolVar(&flags.PrintConfig, "print-config", false, "Print effective config and exit")
	flag.Float64Var(&flags.MinCost, "min-cost", 0, "Hide waste signals below this dollar amount")

	flag.BoolVar(&flags.NoCostOutlier, "no-cost-outlier", false, "Disable cost outlier detection")
	flag.BoolVar(&flags.NoLowSignal, "no-low-signal", false, "Disable low output/input ratio detection")
	flag.BoolVar(&flags.NoSubagentOverhead, "no-subagent-overhead", false, "Disable subagent overhead detection")
	flag.BoolVar(&flags.NoCacheUnderutil, "no-cache-underutil", false, "Disable cache underutilization detection")
	flag.BoolVar(&flags.NoChurn, "no-churn", false, "Disable session churn detection")
	flag.BoolVar(&flags.ShowTrends, "show-trends", false, "Show time-trend summary")
	flag.BoolVar(&flags.RefreshPricing, "refresh-pricing", false, "Force re-fetch pricing from OpenRouter")
	flag.BoolVar(&flags.NoFetchPricing, "no-fetch-pricing", false, "Skip network fetch, use embedded pricing only")

	versionFlag := flag.Bool("version", false, "Print version and exit")
	flag.Parse()

	if *versionFlag {
		fmt.Printf("burnwatch %s (%s) built %s\n", version, commit, date)
		return
	}

	cfg, err := config.Load(flags.ConfigPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Config error: %v\n", err)
		os.Exit(1)
	}

	if err := config.Validate(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "Config error: %v\n", err)
		os.Exit(1)
	}

	if flags.PrintConfig {
		enc := toml.NewEncoder(os.Stdout)
		if err := enc.Encode(cfg); err != nil {
			fmt.Fprintf(os.Stderr, "Error encoding config: %v\n", err)
			os.Exit(1)
		}
		return
	}

	if flags.DBPath != "" {
		_ = os.Setenv("BURNWATCH_OPENCODE_DB", flags.DBPath)
	}

	if !flags.NoFetchPricing {
		httpClient := &http.Client{Timeout: 10 * time.Second}
		if flags.RefreshPricing {
			_ = source.RefreshPricing(httpClient)
		} else {
			_ = source.InitPricing(httpClient)
		}
	}

	sources := source.Discover()
	if len(sources) == 0 {
		fmt.Fprintln(os.Stderr, "No data sources found.")
		os.Exit(1)
	}

	events := output.CollectEvents(sources)
	if len(events) == 0 {
		fmt.Fprintln(os.Stderr, "No data found.")
		os.Exit(1)
	}

	if flags.Harness != "" && flags.Harness != "all" {
		events = filterByHarness(events, flags.Harness)
	}

	if flags.Project != "" {
		events = filterByProject(events, flags.Project)
	}

	if flags.Days > 0 {
		events = filterByDays(events, flags.Days)
	}

	if len(events) == 0 {
		fmt.Fprintln(os.Stderr, "No events after filtering.")
		os.Exit(1)
	}

	baselines := analyze.ComputeBaselines(events)
	costSigma := cfg.Thresholds.CostOutlierSigma
	if flags.NoCostOutlier {
		cfg.Signals.CostOutlier = false
	}
	if flags.NoLowSignal {
		cfg.Signals.LowSignal = false
	}
	if flags.NoSubagentOverhead {
		cfg.Signals.SubagentOverhead = false
	}
	if flags.NoCacheUnderutil {
		cfg.Signals.CacheUnderutilized = false
	}
	if flags.NoChurn {
		cfg.Signals.SessionChurn = false
	}
	if flags.ShowTrends {
		cfg.Output.ShowTrends = true
	}
	toggles := analyze.SignalToggles{
		CostOutlier:        cfg.Signals.CostOutlier,
		LowSignal:          cfg.Signals.LowSignal,
		SubagentOverhead:   cfg.Signals.SubagentOverhead,
		CacheUnderutilized: cfg.Signals.CacheUnderutilized,
		SessionChurn:       cfg.Signals.SessionChurn,
	}
	signals := analyze.DetectWaste(events, baselines, costSigma, toggles)

	minCost := cfg.Filters.MinCost
	if flags.MinCost > 0 {
		minCost = flags.MinCost
	}
	if minCost > 0 {
		signals = analyze.FilterByMinCost(signals, minCost)
	}
	if cfg.Filters.Deduplicate {
		signals = analyze.Deduplicate(signals)
	}
	recommendations := analyze.GenerateRecommendations(signals, baselines)

	if flags.JSON {
		trees := analyze.BuildSubagentTree(events)
		jsonBytes, err := output.FormatJSON(events, baselines, signals, recommendations, trees)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error formatting JSON: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(string(jsonBytes))
	} else {
		text := output.FormatText(events, baselines, signals, recommendations, flags.Verbose, cfg)
		fmt.Print(text)
	}
}

func filterByHarness(events []source.TokenEvent, harness string) []source.TokenEvent {
	var filtered []source.TokenEvent
	for _, e := range events {
		if e.Harness == harness {
			filtered = append(filtered, e)
		}
	}
	return filtered
}

func filterByProject(events []source.TokenEvent, project string) []source.TokenEvent {
	var filtered []source.TokenEvent
	for _, e := range events {
		if e.Project == project {
			filtered = append(filtered, e)
		}
	}
	return filtered
}

func filterByDays(events []source.TokenEvent, days int) []source.TokenEvent {
	if days <= 0 {
		return events
	}
	since := time.Now().AddDate(0, 0, -days)
	var filtered []source.TokenEvent
	for _, e := range events {
		if !e.Timestamp.Before(since) {
			filtered = append(filtered, e)
		}
	}
	return filtered
}
