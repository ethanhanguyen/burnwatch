package output

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ethanhanguyen/burnwatch/analyze"
	"github.com/ethanhanguyen/burnwatch/config"
	"github.com/ethanhanguyen/burnwatch/source"
)

var v1Cfg = func() config.Config {
	cfg := config.Config{Signals: config.Signals{
		CostOutlier:        true,
		LowSignal:          true,
		SubagentOverhead:   true,
		CacheUnderutilized: true,
	}}
	useDefaults(&cfg)
	return cfg
}()

func loadScenarioJSONL(tb testing.TB, name string) []source.TokenEvent {
	tb.Helper()
	path := filepath.Join("..", "testdata", "scenarios", name)
	f, err := os.Open(path)
	if err != nil {
		tb.Fatalf("open scenario %s: %v", path, err)
	}
	defer func() { _ = f.Close() }()

	var events []source.TokenEvent
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 1024*1024), 10*1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		ev := parseScenarioLine(tb, line)
		events = append(events, ev)
	}
	if err := scanner.Err(); err != nil {
		tb.Fatalf("scan scenario %s: %v", path, err)
	}
	return events
}

type scenarioEntry struct {
	Type            string                `json:"type"`
	SessionID       string                `json:"sessionId"`
	ParentSessionID string                `json:"parentSessionId"`
	Message         scenarioMessage       `json:"message"`
	Timestamp       string                `json:"timestamp"`
}

type scenarioMessage struct {
	Model   string                 `json:"model"`
	Role    string                 `json:"role"`
	Usage   *scenarioUsage         `json:"usage"`
	Content []scenarioContentBlock `json:"content"`
}

type scenarioUsage struct {
	InputTokens              int64 `json:"input_tokens"`
	OutputTokens             int64 `json:"output_tokens"`
	CacheCreationInputTokens int64 `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     int64 `json:"cache_read_input_tokens"`
}

type scenarioContentBlock struct {
	Type  string          `json:"type"`
	Name  string          `json:"name"`
	Input json.RawMessage `json:"input"`
}

func parseScenarioLine(tb testing.TB, line string) source.TokenEvent {
	tb.Helper()
	var entry scenarioEntry
	if err := json.Unmarshal([]byte(line), &entry); err != nil {
		tb.Fatalf("unmarshal scenario line: %v", err)
	}
	if entry.Message.Usage == nil {
		tb.Fatal("scenario entry has no usage")
	}
	ts, _ := time.Parse(time.RFC3339, entry.Timestamp)
	if ts.IsZero() {
		ts = time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)
	}

	var toolCalls []source.ToolCall
	var fileOps []source.FileOp
	for _, block := range entry.Message.Content {
		if block.Type != "tool_use" {
			continue
		}
		args := string(block.Input)
		if len(args) > 1024 {
			args = args[:1024]
		}
		toolCalls = append(toolCalls, source.ToolCall{
			Name:      strings.ToLower(block.Name),
			Arguments: args,
		})
		fo := scenarioFileOpFromTool(block.Name, block.Input)
		if fo != nil {
			fileOps = append(fileOps, *fo)
		}
	}

	messageRole := entry.Message.Role
	if messageRole == "" {
		messageRole = "assistant"
	}

	isSubagent := entry.ParentSessionID != ""

	cost, approx, _ := source.CostForModel(
		entry.Message.Model,
		entry.Message.Usage.InputTokens,
		entry.Message.Usage.OutputTokens,
		entry.Message.Usage.CacheReadInputTokens,
		entry.Message.Usage.CacheCreationInputTokens,
	)
	return source.TokenEvent{
		SessionID:       entry.SessionID,
		ParentSessionID: entry.ParentSessionID,
		Model:           entry.Message.Model,
		Provider:        "test",
		Timestamp:       ts,
		InputTokens:     entry.Message.Usage.InputTokens,
		OutputTokens:    entry.Message.Usage.OutputTokens,
		CacheRead:       entry.Message.Usage.CacheReadInputTokens,
		CacheWrite:      entry.Message.Usage.CacheCreationInputTokens,
		CostUSD:         cost,
		CostApproximate: approx,
		Project:         "scenario-test",
		Harness:         "claude-code",
		IsSubagent:      isSubagent,
		ToolCalls:       toolCalls,
		FileOps:         fileOps,
		MessageRole:     messageRole,
	}
}

type scenarioFileInput struct {
	FilePath string `json:"file_path"`
}

func scenarioFileOpFromTool(toolName string, input json.RawMessage) *source.FileOp {
	canon := strings.ToLower(toolName)
	var op string
	switch canon {
	case "read":
		op = "read"
	case "write":
		op = "write"
	case "edit":
		op = "edit"
	default:
		return nil
	}

	var fi scenarioFileInput
	if err := json.Unmarshal(input, &fi); err != nil || fi.FilePath == "" {
		return nil
	}

	return &source.FileOp{
		Path:      source.NormalizePath(fi.FilePath, ""),
		Operation: op,
	}
}

func findSignalByID(signals []analyze.WasteSignal, sessionID string) *analyze.WasteSignal {
	for i := range signals {
		if signals[i].SessionID == sessionID {
			return &signals[i]
		}
	}
	return nil
}

func assignEventIndex(events []source.TokenEvent) {
	idxs := make(map[string]int)
	for i := range events {
		idxs[events[i].SessionID]++
		events[i].EventIndex = idxs[events[i].SessionID]
	}
}

func runScenarioPipeline(t *testing.T, events []source.TokenEvent) ([]analyze.WasteSignal, map[string]analyze.Baseline) {
	assignEventIndex(events)
	t.Helper()
	baselines := analyze.ComputeBaselines(events, config.Defaults())
	if len(baselines) == 0 {
		t.Fatal("no baselines computed")
	}
	signals := analyze.DetectWaste(events, baselines, nil, allCfg)
	return signals, baselines
}

func runPipelineWithSignals(t *testing.T, events []source.TokenEvent, cfg config.Config) []analyze.WasteSignal {
	assignEventIndex(events)
	t.Helper()
	baselines := analyze.ComputeBaselines(events, config.Defaults())
	if len(baselines) == 0 {
		t.Fatal("no baselines computed")
	}
	return analyze.DetectWaste(events, baselines, nil, cfg)
}

func TestScenario_CostOutlier(t *testing.T) {
	events := loadScenarioJSONL(t, "cost_outlier.jsonl")
	baselines := analyze.ComputeBaselines(events, config.Defaults())
	signals := analyze.DetectWaste(events, baselines, nil, v1Cfg)

	var costSig, lowSig *analyze.WasteSignal
	for i := range signals {
		if signals[i].SessionID != "ses_cost_waste" {
			continue
		}
		switch signals[i].Reason {
		case "cost_outlier":
			cp := signals[i]
			costSig = &cp
		case "low_signal":
			lp := signals[i]
			lowSig = &lp
		}
	}
	if costSig == nil {
		t.Fatal("expected ses_cost_waste to be flagged as cost_outlier")
	}
	if costSig.Severity != "high" {
		t.Errorf("expected severity high, got %s", costSig.Severity)
	}
	if costSig.Metric <= costSig.Threshold {
		t.Errorf("expected metric %.6f > threshold %.6f", costSig.Metric, costSig.Threshold)
	}
	_ = lowSig

	for _, id := range []string{"ses_cost_normal_01", "ses_cost_normal_02", "ses_cost_normal_03", "ses_cost_normal_04", "ses_cost_normal_05"} {
		if s := findSignalByID(signals, id); s != nil {
			t.Errorf("normal session %s was flagged unexpectedly (reason=%s)", id, s.Reason)
		}
	}
}

func TestScenario_InputOverconsumption(t *testing.T) {
	events := loadScenarioJSONL(t, "input_overconsumption.jsonl")
	signals, _ := runScenarioPipeline(t, events)

	waste := findSignalByID(signals, "ses_input_waste")
	if waste == nil {
		t.Fatal("expected ses_input_waste to be flagged as costly or anomalous")
	}
	if waste.InputTokens < 100000 {
		t.Errorf("expected input tokens > 100K, got %d", waste.InputTokens)
	}
}

func TestScenario_OutputExplosion(t *testing.T) {
	events := loadScenarioJSONL(t, "output_explosion.jsonl")
	signals, _ := runScenarioPipeline(t, events)

	// v1 flags this as cost_outlier (200K output tokens = massive cost)
	// v2 (PR13) will add dedicated output_explosion heuristic with output token baseline
	waste := findSignalByID(signals, "ses_output_waste")
	if waste == nil {
		// List all reasons for debugging
		reasons := map[string][]string{}
		for _, s := range signals {
			reasons[s.Reason] = append(reasons[s.Reason], s.SessionID)
		}
		t.Fatalf("expected ses_output_waste to be flagged, got signals: %v", reasons)
	}
	if waste.OutputTokens < 100000 {
		t.Errorf("expected output tokens > 100K, got %d", waste.OutputTokens)
	}
}

func TestScenario_LowTokenEfficiency(t *testing.T) {
	events := loadScenarioJSONL(t, "low_token_efficiency.jsonl")
	cfg := allCfg
	cfg.Signals.LowSignal = true
	signals := runPipelineWithSignals(t, events, cfg)

	found := false
	for _, s := range signals {
		if s.SessionID == "ses_ter_waste" && s.Reason == "low_signal" {
			found = true
			break
		}
	}
	if !found {
		reasons := map[string]int{}
		for _, s := range signals {
			if s.SessionID == "ses_ter_waste" {
				reasons[s.Reason]++
			}
		}
		t.Fatalf("expected ses_ter_waste to be flagged as low_signal, got: %v", reasons)
	}
}

func TestScenario_Fragmentation(t *testing.T) {
	events := loadScenarioJSONL(t, "fragmentation.jsonl")
	signals, _ := runScenarioPipeline(t, events)

	day1IDs := map[string]bool{
		"ses_frag_day1_01": true,
		"ses_frag_day1_02": true,
		"ses_frag_day1_03": true,
		"ses_frag_day1_04": true,
	}
	day1Count := 0
	for _, s := range signals {
		if day1IDs[s.SessionID] {
			day1Count++
		}
	}
	if day1Count < 2 {
		t.Logf("fragmentation: day1 only produced %d signals (may need tuning)", day1Count)
	}

	day2IDs := map[string]bool{
		"ses_frag_day2_01": true,
		"ses_frag_day2_02": true,
	}
	for _, s := range signals {
		if day2IDs[s.SessionID] {
			t.Errorf("day 2 session %s flagged unexpectedly (day 2 sessions have high ratio)", s.SessionID)
		}
	}
}

func TestScenario_SubagentOverhead(t *testing.T) {
	parentID := "ses_sub_parent"
	subID := "ses_sub_agent"

	parentEvents := loadScenarioJSONL(t, "subagent_overhead.jsonl")

	totalParentCost := float64(0)
	for _, e := range parentEvents {
		if e.SessionID == parentID {
			totalParentCost += e.CostUSD
		}
	}
	subCost := totalParentCost * 3.0
	subEvent := source.TokenEvent{
		SessionID:       subID,
		ParentSessionID: parentID,
		Model:           "claude-sonnet-4-5-20250929",
		Provider:        "test",
		Timestamp:       time.Date(2026, 5, 1, 10, 2, 0, 0, time.UTC),
		InputTokens:     5000,
		OutputTokens:    500,
		CostUSD:         subCost,
		Project:         "scenario-test",
		Harness:         "claude-code",
		IsSubagent:      true,
	}
	events := append(parentEvents, subEvent)

	trees := analyze.BuildSubagentTree(events)
	var tree *analyze.SubagentTree
	for i := range trees {
		if trees[i].SessionID == parentID {
			tree = &trees[i]
			break
		}
	}
	if tree == nil {
		t.Fatalf("expected subagent tree for parent session %s, got trees: %v", parentID, func() []string {
			var ids []string
			for _, tr := range trees {
				ids = append(ids, tr.SessionID)
			}
			return ids
		}())
	}
	if tree.OverheadPct < 50 {
		t.Fatalf("expected overhead > 50%%, got %.1f%%", tree.OverheadPct)
	}

	cfg := allCfg
	cfg.Signals.SubagentOverhead = true
	baselines := analyze.ComputeBaselines(events, config.Defaults())
	signals := analyze.DetectWaste(events, baselines, trees, cfg)

	foundSubagent := false
	for _, s := range signals {
		if s.SessionID == parentID && s.Reason == "subagent_overhead" {
			foundSubagent = true
			break
		}
	}
	if !foundSubagent {
		reasons := map[string]int{}
		for _, s := range signals {
			if s.SessionID == parentID {
				reasons[s.Reason]++
			}
		}
		t.Fatalf("expected parent session to be flagged for subagent_overhead, got: %v", reasons)
	}
}

func TestScenario_CacheUnderutilized(t *testing.T) {
	events := loadScenarioJSONL(t, "cache_underutilized.jsonl")
	cfg := allCfg
	cfg.Signals.CacheUnderutilized = true
	signals := runPipelineWithSignals(t, events, cfg)

	found := false
	for _, s := range signals {
		if s.SessionID == "ses_cache_waste" && s.Reason == "cache_underutilized" {
			found = true
			break
		}
	}
	if !found {
		reasons := map[string]int{}
		for _, s := range signals {
			if s.SessionID == "ses_cache_waste" {
				reasons[s.Reason]++
			}
		}
		t.Fatalf("expected ses_cache_waste to be flagged for cache_underutilized, got: %v", reasons)
	}
}

func TestScenario_MultiSignal(t *testing.T) {
	events := loadScenarioJSONL(t, "multi_signal.jsonl")
	signals, _ := runScenarioPipeline(t, events)

	waste := findSignalByID(signals, "ses_multi_waste")
	if waste == nil {
		t.Fatal("expected ses_multi_waste to be flagged (cost outlier + low ratio)")
	}

	reasons := map[string]int{}
	for _, s := range signals {
		if s.SessionID == "ses_multi_waste" {
			reasons[s.Reason]++
		}
	}
	if len(reasons) < 1 {
		t.Error("expected at least 1 signal reason for multi-signal session")
	}
}

func TestScenario_AllClean(t *testing.T) {
	events := loadScenarioJSONL(t, "all_clean.jsonl")
	assignEventIndex(events)
	baselines := analyze.ComputeBaselines(events, config.Defaults())
	signals := analyze.DetectWaste(events, baselines, nil, v1Cfg)

	for _, s := range signals {
		if strings.HasPrefix(s.SessionID, "ses_clean_") {
			t.Errorf("clean session %s flagged with reason=%s", s.SessionID, s.Reason)
		}
	}
}

func v3Cfg() config.Config {
	cfg := config.Config{Signals: config.Signals{
		ToolLoop:   true,
		FileReread: true,
	}}
	useDefaults(&cfg)
	return cfg
}

func TestScenario_ToolLoop(t *testing.T) {
	events := loadScenarioJSONL(t, "tool_loop_edge.jsonl")
	assignEventIndex(events)
	cfg := v3Cfg()
	cfg.Thresholds.ToolLoopMaxRepeats = 5

	baselines := analyze.ComputeBaselines(events, config.Defaults())
	signals := analyze.DetectWaste(events, baselines, nil, cfg)

	sig := findSignalByID(signals, "ses_loop_edge_5")
	if sig == nil {
		var reasons []string
		for _, s := range signals {
			reasons = append(reasons, s.Reason)
		}
		t.Fatalf("expected ses_loop_edge_5 to be flagged as tool_call_loop, got signals: %v", reasons)
	}
	if sig.Reason != "tool_call_loop" {
		t.Errorf("expected reason tool_call_loop, got %s", sig.Reason)
	}
	if sig.Severity != "high" {
		t.Errorf("expected severity high, got %s", sig.Severity)
	}

	if s := findSignalByID(signals, "ses_loop_no_repeat"); s != nil && s.Reason == "tool_call_loop" {
		t.Errorf("ses_loop_no_repeat should not be flagged as tool_call_loop")
	}
	if s := findSignalByID(signals, "ses_loop_normal_03"); s != nil && s.Reason == "tool_call_loop" {
		t.Errorf("ses_loop_normal_03 should not be flagged as tool_call_loop")
	}
}

func TestScenario_FileReRead(t *testing.T) {
	events := loadScenarioJSONL(t, "file_reread.jsonl")
	assignEventIndex(events)
	cfg := v3Cfg()
	cfg.Thresholds.FileRereadMinCount = 3

	baselines := analyze.ComputeBaselines(events, config.Defaults())
	signals := analyze.DetectWaste(events, baselines, nil, cfg)

	sig := findSignalByID(signals, "ses_reread_waste")
	if sig == nil {
		var reasons []string
		for _, s := range signals {
			reasons = append(reasons, s.Reason)
		}
		t.Fatalf("expected ses_reread_waste to be flagged as file_reread, got signals: %v", reasons)
	}
	if sig.Reason != "file_reread" {
		t.Errorf("expected reason file_reread, got %s", sig.Reason)
	}
	if sig.Severity != "medium" {
		t.Errorf("expected severity medium, got %s", sig.Severity)
	}
	if !strings.Contains(sig.Detail, "config/settings.json") {
		t.Errorf("expected Detail to mention config/settings.json, got %s", sig.Detail)
	}

	for _, id := range []string{"ses_reread_normal_01", "ses_reread_normal_02"} {
		if s := findSignalByID(signals, id); s != nil && s.Reason == "file_reread" {
			t.Errorf("normal session %s was flagged unexpectedly (reason=%s)", id, s.Reason)
		}
	}
}

func TestScenario_FileReReadMixed(t *testing.T) {
	events := loadScenarioJSONL(t, "file_reread_mixed.jsonl")
	assignEventIndex(events)
	cfg := v3Cfg()
	cfg.Thresholds.FileRereadMinCount = 3

	baselines := analyze.ComputeBaselines(events, config.Defaults())
	signals := analyze.DetectWaste(events, baselines, nil, cfg)

	sig := findSignalByID(signals, "ses_reread_mixed_w")
	if sig == nil {
		var reasons []string
		for _, s := range signals {
			reasons = append(reasons, s.Reason)
		}
		t.Fatalf("expected ses_reread_mixed_w to be flagged as file_reread, got signals: %v", reasons)
	}
	if sig.Reason != "file_reread" {
		t.Errorf("expected reason file_reread, got %s", sig.Reason)
	}
	if !strings.Contains(sig.Detail, "config/base.json") {
		t.Errorf("expected Detail to mention config/base.json, got %s", sig.Detail)
	}

	if s := findSignalByID(signals, "ses_reread_below_threshold"); s != nil && s.Reason == "file_reread" {
		t.Errorf("ses_reread_below_threshold (2 reads) should not be flagged")
	}
	if s := findSignalByID(signals, "ses_reread_cached"); s != nil && s.Reason == "file_reread" {
		t.Errorf("ses_reread_cached (cached reads) should not be flagged")
	}
}

func v3FullCfg() config.Config {
	cfg := config.Config{Signals: config.Signals{
		ToolLoop:        true,
		FileReread:      true,
		SubagentOverlap: true,
		SessionRestart:  true,
	}}
	useDefaults(&cfg)
	return cfg
}

func TestScenario_SubagentOverlap(t *testing.T) {
	events := loadScenarioJSONL(t, "subagent_overlap.jsonl")
	assignEventIndex(events)
	trees := analyze.BuildSubagentTree(events)
	cfg := v3FullCfg()
	cfg.Thresholds.SubagentOverlapPct = 50.0

	baselines := analyze.ComputeBaselines(events, config.Defaults())
	signals := analyze.DetectWaste(events, baselines, trees, cfg)

	sig := findSignalByID(signals, "ses_overlap_parent")
	if sig == nil {
		var reasons []string
		for _, s := range signals {
			reasons = append(reasons, s.Reason)
		}
		t.Fatalf("expected ses_overlap_parent to be flagged as subagent_overlap, got signals: %v", reasons)
	}
	if sig.Reason != "subagent_overlap" {
		t.Errorf("expected reason subagent_overlap, got %s", sig.Reason)
	}
	if sig.Severity != "high" {
		t.Errorf("expected severity high, got %s", sig.Severity)
	}

	for _, id := range []string{"ses_overlap_normal_01", "ses_overlap_normal_02"} {
		if s := findSignalByID(signals, id); s != nil && s.Reason == "subagent_overlap" {
			t.Errorf("normal session %s was flagged as subagent_overlap", id)
		}
	}
}

func TestScenario_SessionRestart(t *testing.T) {
	events := loadScenarioJSONL(t, "session_restart.jsonl")
	assignEventIndex(events)
	cfg := v3FullCfg()
	cfg.Thresholds.SessionRestartPct = 80.0
	cfg.Thresholds.SessionRestartInitialOps = 10

	baselines := analyze.ComputeBaselines(events, config.Defaults())
	signals := analyze.DetectWaste(events, baselines, nil, cfg)

	sig := findSignalByID(signals, "ses_restart_b")
	if sig == nil {
		var reasons []string
		for _, s := range signals {
			reasons = append(reasons, s.Reason)
		}
		t.Fatalf("expected ses_restart_b to be flagged as session_restart, got signals: %v", reasons)
	}
	if sig.Reason != "session_restart" {
		t.Errorf("expected reason session_restart, got %s", sig.Reason)
	}
	if sig.Severity != "medium" {
		t.Errorf("expected severity medium, got %s", sig.Severity)
	}

	if s := findSignalByID(signals, "ses_restart_continued"); s != nil && s.Reason == "session_restart" {
		t.Errorf("ses_restart_continued was flagged unexpectedly as session_restart")
	}
}
