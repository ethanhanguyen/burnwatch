package cmd

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/yourname/burnwatch/analyze"
	"github.com/yourname/burnwatch/output"
	"github.com/yourname/burnwatch/source"
)

func Execute() {
	var flags struct {
		DBPath  string
		Harness string
		Project string
		JSON    bool
		Days    int
		Verbose bool
	}

	flag.StringVar(&flags.DBPath, "db", "", "OpenCode database path")
	flag.StringVar(&flags.Harness, "harness", "all", "Filter to harness: all, opencode, claude-code")
	flag.StringVar(&flags.Project, "project", "", "Filter to project")
	flag.BoolVar(&flags.JSON, "json", false, "Output as JSON instead of text")
	flag.IntVar(&flags.Days, "days", 0, "Lookback window in days (default: 1 for today, 7 for week, 30 for month)")
	flag.BoolVar(&flags.Verbose, "verbose", false, "Show all events, not just waste signals")
	flag.Parse()

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
	signals := analyze.DetectWaste(events, baselines)
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
