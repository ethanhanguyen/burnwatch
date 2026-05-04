package cmd

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/ethanhanguyen/burnwatch/analyze"
	"github.com/ethanhanguyen/burnwatch/config"
	"github.com/ethanhanguyen/burnwatch/report"
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

		NoCostOutlier          bool
		NoLowSignal            bool
		NoSubagentOverhead     bool
		NoCacheUnderutil       bool
		NoFragmentationIndex   bool
		NoInputOverconsumption   bool
		NoOutputExplosion        bool
		NoTokenEfficiency        bool
		NoToolLoop               bool
		NoFileReread             bool
		NoSubagentOverlap        bool
		NoSessionRestart         bool
		ShowTrends               bool

		RefreshPricing bool
		NoFetchPricing bool
		Init           bool
		Calibrate      bool

		InputOverconsumptionSigma float64
		OutputExplosionSigma      float64
		TokenEfficiencyPercentile float64
		FragmentationThreshold    float64
		SubagentOverheadPct       float64
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
	flag.BoolVar(&flags.NoFragmentationIndex, "no-fragmentation-index", false, "Disable fragmentation index detection")
	flag.BoolVar(&flags.NoInputOverconsumption, "no-input-overconsumption", false, "Disable input overconsumption detection")
	flag.BoolVar(&flags.NoOutputExplosion, "no-output-explosion", false, "Disable output explosion detection")
	flag.BoolVar(&flags.NoTokenEfficiency, "no-token-efficiency", false, "Disable token efficiency detection")
	flag.BoolVar(&flags.NoToolLoop, "no-tool-loop", false, "Disable tool loop detection")
	flag.BoolVar(&flags.NoFileReread, "no-file-reread", false, "Disable file re-read detection")
	flag.BoolVar(&flags.NoSubagentOverlap, "no-subagent-overlap", false, "Disable subagent overlap detection")
	flag.BoolVar(&flags.NoSessionRestart, "no-session-restart", false, "Disable session restart detection")
	flag.BoolVar(&flags.ShowTrends, "show-trends", false, "Show time-trend summary")
	flag.BoolVar(&flags.RefreshPricing, "refresh-pricing", false, "Force re-fetch pricing from OpenRouter")
	flag.BoolVar(&flags.NoFetchPricing, "no-fetch-pricing", false, "Skip network fetch, use embedded pricing only")
	flag.BoolVar(&flags.Init, "init", false, "Write default .burnwatch.toml and exit")
	flag.BoolVar(&flags.Calibrate, "calibrate", false, "Show data distribution and suggest thresholds")

	explainID := flag.String("explain", "", "Show annotated timeline for session ID")

	flag.Float64Var(&flags.InputOverconsumptionSigma, "input-sigma", 0, "Sigma for input overconsumption detection (0 = use config)")
	flag.Float64Var(&flags.OutputExplosionSigma, "output-sigma", 0, "Sigma for output explosion detection (0 = use config)")
	flag.Float64Var(&flags.TokenEfficiencyPercentile, "ter-percentile", 0, "Percentile for token efficiency threshold (0 = use config)")
	flag.Float64Var(&flags.FragmentationThreshold, "fragmentation-threshold", 0, "Threshold for fragmentation index (0 = use config)")
	flag.Float64Var(&flags.SubagentOverheadPct, "subagent-overhead", 0, "Subagent overhead percentage threshold (0 = use config)")

	versionFlag := flag.Bool("version", false, "Print version and exit")
	flag.Parse()

	if *versionFlag {
		fmt.Printf("burnwatch %s (%s) built %s\n", version, commit, date)
		return
	}

	if flags.Init {
		if _, err := os.Stat(".burnwatch.toml"); err == nil {
			fmt.Fprintln(os.Stderr, "Error: .burnwatch.toml already exists. Delete it first to regenerate.")
			os.Exit(1)
		}
		if err := config.WriteDefault(".burnwatch.toml"); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing config: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Wrote .burnwatch.toml with default settings and comments.")
		return
	}

	if flags.Calibrate {
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

		events := report.CollectEvents(sources)
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

		data := analyze.ComputeCalibration(events)

		if flags.JSON {
			jsonBytes, err := report.FormatCalibrationJSON(data)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error formatting JSON: %v\n", err)
				os.Exit(1)
			}
			fmt.Println(string(jsonBytes))
		} else {
			text := report.FormatCalibrationText(data)
			fmt.Print(text)
		}
		return
	}

	if *explainID != "" {
		sources := source.Discover()
		if len(sources) == 0 {
			fmt.Fprintln(os.Stderr, "No data sources found.")
			os.Exit(1)
		}
		events := report.CollectEvents(sources)
		var sessionEvents []source.TokenEvent
		for _, e := range events {
			if e.SessionID == *explainID {
				sessionEvents = append(sessionEvents, e)
			}
		}
		if len(sessionEvents) == 0 {
			fmt.Fprintf(os.Stderr, "Session %q not found.\n", *explainID)
			os.Exit(1)
		}

		baselines := analyze.ComputeBaselines(events, config.Defaults())
		trees := analyze.BuildSubagentTree(events)
		cfg := config.Defaults()
		signals := analyze.DetectWaste(events, baselines, trees, cfg)

		var sessionSignals []analyze.WasteSignal
		for _, s := range signals {
			if s.SessionID == *explainID {
				sessionSignals = append(sessionSignals, s)
			}
		}

		var sessionTrees []analyze.SubagentTree
		for _, t := range trees {
			if t.SessionID == *explainID {
				sessionTrees = append(sessionTrees, t)
			} else {
				for _, n := range t.Subagents {
					if hasSubagentForSession(n, *explainID) {
						sessionTrees = append(sessionTrees, t)
						break
					}
				}
			}
		}

		text := report.FormatExplain(*explainID, sessionEvents, sessionSignals, sessionTrees)
		fmt.Print(text)
		return
	}

	if len(flag.Args()) > 0 && flag.Args()[0] == "report" {
		handleReport(flag.Args()[1:], flags)
		return
	}

	if len(flag.Args()) > 0 && flag.Args()[0] == "watch" {
		handleWatchCmd(flag.Args()[1:], flags)
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

	events := report.CollectEvents(sources)
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

	baselines := analyze.ComputeBaselines(events, cfg)
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
	if flags.NoFragmentationIndex {
		cfg.Signals.FragmentationIndex = false
	}
	if flags.NoInputOverconsumption {
		cfg.Signals.InputOverconsumption = false
	}
	if flags.NoOutputExplosion {
		cfg.Signals.OutputExplosion = false
	}
	if flags.NoTokenEfficiency {
		cfg.Signals.TokenEfficiency = false
	}
	if flags.NoToolLoop {
		cfg.Signals.ToolLoop = false
	}
	if flags.NoFileReread {
		cfg.Signals.FileReread = false
	}
	if flags.NoSubagentOverlap {
		cfg.Signals.SubagentOverlap = false
	}
	if flags.NoSessionRestart {
		cfg.Signals.SessionRestart = false
	}
	if flags.ShowTrends {
		cfg.Output.ShowTrends = true
	}

	if flags.InputOverconsumptionSigma > 0 {
		cfg.Thresholds.InputOverconsumptionSigma = flags.InputOverconsumptionSigma
	}
	if flags.OutputExplosionSigma > 0 {
		cfg.Thresholds.OutputExplosionSigma = flags.OutputExplosionSigma
	}
	if flags.TokenEfficiencyPercentile > 0 {
		cfg.Thresholds.TokenEfficiencyPercentile = flags.TokenEfficiencyPercentile
	}
	if flags.FragmentationThreshold > 0 {
		cfg.Thresholds.FragmentationIndexThreshold = flags.FragmentationThreshold
	}
	if flags.SubagentOverheadPct > 0 {
		cfg.Thresholds.SubagentOverheadPct = flags.SubagentOverheadPct
	}

	trees := analyze.BuildSubagentTree(events)
	signals := analyze.DetectWaste(events, baselines, trees, cfg)

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
		jsonBytes, err := report.FormatJSON(events, baselines, signals, recommendations, trees)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error formatting JSON: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(string(jsonBytes))
	} else {
		text := report.FormatText(events, baselines, signals, recommendations, flags.Verbose, cfg)
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

func hasSubagentForSession(n analyze.SubagentNode, sessionID string) bool {
	if n.SessionID == sessionID {
		return true
	}
	for _, c := range n.Children {
		if hasSubagentForSession(c, sessionID) {
			return true
		}
	}
	return false
}

type reportFlags struct {
	outputPath string
	open       bool
	days       int
	reportJSON bool
}

func parseReportFlags(args []string) (*reportFlags, []string) {
	fs := flag.NewFlagSet("report", flag.ExitOnError)
	rf := &reportFlags{}
	fs.StringVar(&rf.outputPath, "output", "", "Report output file path")
	fs.BoolVar(&rf.open, "open", false, "Open report in browser after generation")
	fs.IntVar(&rf.days, "days", 30, "Days of data for report")
	fs.BoolVar(&rf.reportJSON, "report-json", false, "Output report JSON data (no HTML)")
	_ = fs.Parse(args)
	return rf, fs.Args()
}

func handleReport(extraArgs []string, flags struct {
	DBPath      string
	Harness     string
	Project     string
	JSON        bool
	Days        int
	Verbose     bool
	ConfigPath  string
	PrintConfig bool
	MinCost     float64

	NoCostOutlier          bool
	NoLowSignal            bool
	NoSubagentOverhead     bool
	NoCacheUnderutil       bool
	NoFragmentationIndex   bool
	NoInputOverconsumption   bool
	NoOutputExplosion        bool
	NoTokenEfficiency        bool
	NoToolLoop               bool
	NoFileReread             bool
	NoSubagentOverlap        bool
	NoSessionRestart         bool
	ShowTrends               bool

	RefreshPricing bool
	NoFetchPricing bool
	Init           bool
	Calibrate      bool

	InputOverconsumptionSigma float64
	OutputExplosionSigma      float64
	TokenEfficiencyPercentile float64
	FragmentationThreshold    float64
	SubagentOverheadPct       float64
}) {
	rf, _ := parseReportFlags(extraArgs)

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

	events := report.CollectEvents(sources)
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

	days := rf.days
	if flags.Days > 0 {
		days = flags.Days
	}
	if days > 0 {
		events = filterByDays(events, days)
	}

	if len(events) == 0 {
		fmt.Fprintln(os.Stderr, "No events after filtering.")
		os.Exit(1)
	}

	baselines := analyze.ComputeBaselines(events, config.Defaults())
	trees := analyze.BuildSubagentTree(events)
	cfg, err := config.Load(flags.ConfigPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Config error: %v\n", err)
		os.Exit(1)
	}
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
	if flags.NoFragmentationIndex {
		cfg.Signals.FragmentationIndex = false
	}
	if flags.NoInputOverconsumption {
		cfg.Signals.InputOverconsumption = false
	}
	if flags.NoOutputExplosion {
		cfg.Signals.OutputExplosion = false
	}
	if flags.NoTokenEfficiency {
		cfg.Signals.TokenEfficiency = false
	}
	if flags.NoToolLoop {
		cfg.Signals.ToolLoop = false
	}
	if flags.NoFileReread {
		cfg.Signals.FileReread = false
	}
	if flags.NoSubagentOverlap {
		cfg.Signals.SubagentOverlap = false
	}
	if flags.NoSessionRestart {
		cfg.Signals.SessionRestart = false
	}
	if flags.ShowTrends {
		cfg.Output.ShowTrends = true
	}
	if flags.InputOverconsumptionSigma > 0 {
		cfg.Thresholds.InputOverconsumptionSigma = flags.InputOverconsumptionSigma
	}
	if flags.OutputExplosionSigma > 0 {
		cfg.Thresholds.OutputExplosionSigma = flags.OutputExplosionSigma
	}
	if flags.TokenEfficiencyPercentile > 0 {
		cfg.Thresholds.TokenEfficiencyPercentile = flags.TokenEfficiencyPercentile
	}
	if flags.FragmentationThreshold > 0 {
		cfg.Thresholds.FragmentationIndexThreshold = flags.FragmentationThreshold
	}
	if flags.SubagentOverheadPct > 0 {
		cfg.Thresholds.SubagentOverheadPct = flags.SubagentOverheadPct
	}
	signals := analyze.DetectWaste(events, baselines, trees, cfg)
	recs := analyze.GenerateRecommendations(signals, baselines)

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

	if rf.reportJSON {
		jsonData, err := report.FormatReportJSON(events, baselines, signals, trees, version, time.Now())
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error formatting JSON: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(string(jsonData))
		return
	}

	htmlReport := report.FormatReport(events, baselines, signals, recs, trees, version, time.Now())

	outputPath := rf.outputPath
	if outputPath == "" {
		outputPath = fmt.Sprintf("reports/burnwatch-report-%s.html", time.Now().Format("2006-01-02"))
	}
	_ = os.MkdirAll("reports", 0755)

	if err := os.WriteFile(outputPath, []byte(htmlReport), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing report: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Report written to %s (%d KiB)\n", outputPath, len(htmlReport)/1024)

	if rf.open {
		if err := openBrowser(outputPath); err != nil {
			fmt.Fprintf(os.Stderr, "Error opening browser: %v\n", err)
		}
	}
}

func openBrowser(path string) error {
	var cmd string
	var args []string
	switch runtime.GOOS {
	case "darwin":
		cmd = "open"
	case "linux":
		cmd = "xdg-open"
	case "windows":
		cmd = "cmd"
		args = []string{"/c", "start"}
	default:
		return fmt.Errorf("unsupported platform %s", runtime.GOOS)
	}
	args = append(args, path)
	return exec.Command(cmd, args...).Start()
}

type watchCmdFlags struct {
	interval int
}

func parseWatchFlags(args []string) (*watchCmdFlags, []string) {
	fs := flag.NewFlagSet("watch", flag.ExitOnError)
	wf := &watchCmdFlags{}
	fs.IntVar(&wf.interval, "interval", 5, "Poll interval in seconds")
	_ = fs.Parse(args)
	return wf, fs.Args()
}

func handleWatchCmd(extraArgs []string, flags struct {
	DBPath      string
	Harness     string
	Project     string
	JSON        bool
	Days        int
	Verbose     bool
	ConfigPath  string
	PrintConfig bool
	MinCost     float64

	NoCostOutlier          bool
	NoLowSignal            bool
	NoSubagentOverhead     bool
	NoCacheUnderutil       bool
	NoFragmentationIndex   bool
	NoInputOverconsumption   bool
	NoOutputExplosion        bool
	NoTokenEfficiency        bool
	NoToolLoop               bool
	NoFileReread             bool
	NoSubagentOverlap        bool
	NoSessionRestart         bool
	ShowTrends               bool

	RefreshPricing bool
	NoFetchPricing bool
	Init           bool
	Calibrate      bool

	InputOverconsumptionSigma float64
	OutputExplosionSigma      float64
	TokenEfficiencyPercentile float64
	FragmentationThreshold    float64
	SubagentOverheadPct       float64
}) {
	wf, _ := parseWatchFlags(extraArgs)

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

	if err := handleWatch(sources, wf.interval); err != nil {
		fmt.Fprintf(os.Stderr, "Watch error: %v\n", err)
		os.Exit(1)
	}
}
