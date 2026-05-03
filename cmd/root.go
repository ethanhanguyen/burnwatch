package cmd

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/ethanhanguyen/burnwatch/analyze"
	"github.com/ethanhanguyen/burnwatch/config"
	"github.com/ethanhanguyen/burnwatch/output"
	"github.com/ethanhanguyen/burnwatch/source"
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
	flag.Parse()

	cfg, err := config.Load(flags.ConfigPath)
	if err != nil {
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
	signals := analyze.DetectWaste(events, baselines, costSigma)

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
		text := output.FormatText(events, baselines, signals, recommendations, flags.Verbose)
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
