package output

import (
	"strings"
	"testing"
	"time"

	"github.com/ethanhanguyen/burnwatch/analyze"
	"github.com/ethanhanguyen/burnwatch/config"
	"github.com/ethanhanguyen/burnwatch/source"
)

func TestFormatExplain_Loop(t *testing.T) {
	events := loadScenarioJSONL(t, "explain_loop.jsonl")
	assignEventIndex(events)

	cfg := v3Cfg()
	cfg.Thresholds.ToolLoopMaxRepeats = 5

	baselines := analyze.ComputeBaselines(events, config.Defaults())
	signals := analyze.DetectWaste(events, baselines, nil, cfg)

	var sessionEvents []source.TokenEvent
	for _, e := range events {
		if e.SessionID == "ses_explain_loop" {
			sessionEvents = append(sessionEvents, e)
		}
	}

	var sessionSignals []analyze.WasteSignal
	for _, s := range signals {
		if s.SessionID == "ses_explain_loop" {
			sessionSignals = append(sessionSignals, s)
		}
	}

	output := FormatExplain("ses_explain_loop", sessionEvents, sessionSignals, nil)

	mustContain(t, output, "ses_explain_loop")
	mustContain(t, output, "scenario-test")
	mustContain(t, output, "claude-code")
	mustContain(t, output, "[LOOP REPEAT 1/6]")
	mustContain(t, output, "[LOOP REPEAT 6/6]")
	mustContain(t, output, "tool_call_loop")

	if !strings.Contains(output, "LOOP REPEAT") {
		t.Fatal("expected LOOP REPEAT annotations in output")
	}
}

func TestFormatExplain_ReRead(t *testing.T) {
	events := loadScenarioJSONL(t, "explain_reread.jsonl")
	assignEventIndex(events)

	cfg := v3Cfg()
	cfg.Thresholds.FileRereadMinCount = 3

	baselines := analyze.ComputeBaselines(events, config.Defaults())
	signals := analyze.DetectWaste(events, baselines, nil, cfg)

	var sessionEvents []source.TokenEvent
	for _, e := range events {
		if e.SessionID == "ses_explain_reread" {
			sessionEvents = append(sessionEvents, e)
		}
	}

	var sessionSignals []analyze.WasteSignal
	for _, s := range signals {
		if s.SessionID == "ses_explain_reread" {
			sessionSignals = append(sessionSignals, s)
		}
	}

	output := FormatExplain("ses_explain_reread", sessionEvents, sessionSignals, nil)

	mustContain(t, output, "ses_explain_reread")
	mustContain(t, output, "file_reread")
	mustContain(t, output, "[RE-READ")
	mustContain(t, output, "config/settings.json")
	mustContain(t, output, "RE-READ 4/4")
	mustContain(t, output, "File Re-Read Breakdown")
}

func TestFormatExplain_Mixed(t *testing.T) {
	events := loadScenarioJSONL(t, "explain_mixed.jsonl")
	assignEventIndex(events)

	cfg := config.Config{Signals: config.Signals{
		ToolLoop:   true,
		FileReread: true,
	}}
	useDefaults(&cfg)
	cfg.Thresholds.ToolLoopMaxRepeats = 5
	cfg.Thresholds.FileRereadMinCount = 3

	baselines := analyze.ComputeBaselines(events, config.Defaults())
	signals := analyze.DetectWaste(events, baselines, nil, cfg)

	var sessionEvents []source.TokenEvent
	for _, e := range events {
		if e.SessionID == "ses_explain_mixed" {
			sessionEvents = append(sessionEvents, e)
		}
	}

	var sessionSignals []analyze.WasteSignal
	for _, s := range signals {
		if s.SessionID == "ses_explain_mixed" {
			sessionSignals = append(sessionSignals, s)
		}
	}

	output := FormatExplain("ses_explain_mixed", sessionEvents, sessionSignals, nil)

	mustContain(t, output, "ses_explain_mixed")
	mustContain(t, output, "tool_call_loop")
	mustContain(t, output, "file_reread")
	mustContain(t, output, "[LOOP REPEAT")
	mustContain(t, output, "[RE-READ")
}

func TestFormatExplain_Clean(t *testing.T) {
	events := loadScenarioJSONL(t, "explain_clean.jsonl")
	assignEventIndex(events)

	cfg := v3Cfg()
	cfg.Thresholds.ToolLoopMaxRepeats = 5
	cfg.Thresholds.FileRereadMinCount = 3

	baselines := analyze.ComputeBaselines(events, config.Defaults())
	signals := analyze.DetectWaste(events, baselines, nil, cfg)

	var sessionEvents []source.TokenEvent
	for _, e := range events {
		if e.SessionID == "ses_explain_clean" {
			sessionEvents = append(sessionEvents, e)
		}
	}

	var sessionSignals []analyze.WasteSignal
	for _, s := range signals {
		if s.SessionID == "ses_explain_clean" {
			sessionSignals = append(sessionSignals, s)
		}
	}

	output := FormatExplain("ses_explain_clean", sessionEvents, sessionSignals, nil)

	mustContain(t, output, "ses_explain_clean")
	mustNotContain(t, output, "Waste Signals")
}

func TestFormatExplain_Empty(t *testing.T) {
	output := FormatExplain("ses_nonexistent", nil, nil, nil)
	if !strings.Contains(output, "No events found") {
		t.Errorf("expected 'No events found', got %q", output)
	}
}

func TestFormatExplain_Duration(t *testing.T) {
	tests := []struct {
		name     string
		start    string
		end      string
		expected string
	}{
		{"seconds", "2026-01-01T00:00:00Z", "2026-01-01T00:00:45Z", "45s"},
		{"minutes", "2026-01-01T00:00:00Z", "2026-01-01T00:05:00Z", "5m"},
		{"hour_min", "2026-01-01T00:00:00Z", "2026-01-01T01:30:00Z", "1h 30m"},
		{"hours_mins", "2026-01-01T00:00:00Z", "2026-01-01T03:02:00Z", "3h 2m"},
		{"round_sec_up", "2026-01-01T00:00:00Z", "2026-01-01T00:00:44.6Z", "45s"},
		{"round_min_up", "2026-01-01T00:00:00Z", "2026-01-01T00:05:30Z", "6m"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start := mustParseTime(t, tt.start)
			end := mustParseTime(t, tt.end)
			got := formatDuration(start, end)
			if got != tt.expected {
				t.Errorf("formatDuration = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestParseLoopDetail(t *testing.T) {
	tests := []struct {
		detail       string
		wantTool     string
		wantFilePath string
	}{
		{`read("src/main.go") called 12 times consecutively in session`, "read", "src/main.go"},
		{`Bash called 5 times consecutively in session`, "Bash", ""},
		{`write("config/settings.json") called 3 times consecutively in session`, "write", "config/settings.json"},
		{"malformed string without called", "", ""},
		{"tool() called 1 time", "tool", ""},
	}
	for _, tt := range tests {
		t.Run(tt.detail, func(t *testing.T) {
			gotTool, gotPath := parseLoopDetail(tt.detail)
			if gotTool != tt.wantTool {
				t.Errorf("parseLoopDetail toolName = %q, want %q", gotTool, tt.wantTool)
			}
			if gotPath != tt.wantFilePath {
				t.Errorf("parseLoopDetail filePath = %q, want %q", gotPath, tt.wantFilePath)
			}
		})
	}
}

func TestParseRereadDetail(t *testing.T) {
	tests := []struct {
		detail    string
		wantPath  string
		wantCount int
	}{
		{"config/settings.json read 4 times, 0 cache hits between reads", "config/settings.json", 4},
		{"src/main.go read 12 times, 0 cache hits between reads", "src/main.go", 12},
		{"path/with/slashes.ts read 7 times, 0 cache hits between reads", "path/with/slashes.ts", 7},
		{"no count times", "", 0},
	}
	for _, tt := range tests {
		t.Run(tt.detail, func(t *testing.T) {
			gotPath, gotCount := parseRereadDetail(tt.detail)
			if gotPath != tt.wantPath {
				t.Errorf("parseRereadDetail path = %q, want %q", gotPath, tt.wantPath)
			}
			if gotCount != tt.wantCount {
				t.Errorf("parseRereadDetail count = %d, want %d", gotCount, tt.wantCount)
			}
		})
	}
}

func mustContain(t *testing.T, haystack, needle string) {
	t.Helper()
	if !strings.Contains(haystack, needle) {
		t.Errorf("expected output to contain %q", needle)
	}
}

func mustNotContain(t *testing.T, haystack, needle string) {
	t.Helper()
	if strings.Contains(haystack, needle) {
		t.Errorf("expected output to NOT contain %q", needle)
	}
}

func mustParseTime(t *testing.T, s string) time.Time {
	t.Helper()
	ts, err := time.Parse(time.RFC3339, s)
	if err != nil {
		var err2 error
		ts, err2 = time.Parse("2006-01-02T15:04:05.0Z", s)
		if err2 != nil {
			ts, _ = time.Parse("2006-01-02T15:04:05Z", s)
		}
	}
	if ts.IsZero() {
		t.Fatalf("parse time %q: %v", s, err)
	}
	return ts
}
