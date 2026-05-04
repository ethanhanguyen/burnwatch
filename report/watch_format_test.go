package report

import (
	"strings"
	"testing"
	"time"

	"github.com/ethanhanguyen/burnwatch/analyze"
	"github.com/ethanhanguyen/burnwatch/source"
)

func TestRenderSparkline_AllZero(t *testing.T) {
	var freq [10]int
	result := RenderSparkline(freq)
	if strings.TrimSpace(result) != "" {
		t.Errorf("expected empty sparkline for all zeros, got %q", result)
	}
}

func TestRenderSparkline_Uniform(t *testing.T) {
	freq := [10]int{5, 5, 5, 5, 5, 5, 5, 5, 5, 5}
	result := RenderSparkline(freq)
	if len([]rune(result)) < 10 {
		t.Errorf("expected at least 10 chars, got %q", result)
	}
}

func TestRenderSparkline_Varied(t *testing.T) {
	freq := [10]int{1, 2, 5, 10, 8, 6, 4, 2, 1, 0}
	result := RenderSparkline(freq)
	runes := []rune(result)
	if len(runes) < 10 {
		t.Errorf("expected at least 10 chars, got %q", result)
	}
}

func TestRenderSparkline_Single(t *testing.T) {
	var freq [10]int
	freq[5] = 20
	result := RenderSparkline(freq)
	runes := []rune(result)
	if len(runes) < 10 {
		t.Errorf("expected at least 10 chars, got %q", result)
	}
}

func TestRenderSessionRow_Active(t *testing.T) {
	ws := &analyze.WatchSession{
		SessionID: "ses_abc123",
		Project:   "burnwatch",
		Harness:   "Claude Code",
		Model:     "sonnet-4",
		StartedAt: time.Now().Add(-12 * time.Minute),
		Cost:      2.14,
		Events:    make([]source.TokenEvent, 48),
		LastAction: "read src/handler.go",
		EventFreq:  [10]int{5, 5, 5, 5, 5, 5, 5, 5, 5, 5},
	}

	result := RenderSessionRow(ws, 80)
	if !strings.Contains(result, "ses_abc123") {
		t.Error("expected session ID in row")
	}
	if !strings.Contains(result, "burnwatch") {
		t.Error("expected project in row")
	}
	if !strings.Contains(result, "Claude Code") {
		t.Error("expected harness in row")
	}
}

func TestRenderSessionRow_Idle(t *testing.T) {
	ws := &analyze.WatchSession{
		SessionID: "ses_idle",
		Project:   "test",
		Harness:   "Claude Code",
		Model:     "sonnet",
		StartedAt: time.Now().Add(-34 * time.Minute),
		Cost:      1.36,
		Events:    make([]source.TokenEvent, 85),
		Idle:      true,
		LastAction: "bash go test ./...",
		EventFreq:  [10]int{3, 0, 0, 0, 0, 0, 0, 0, 0, 0},
	}

	result := RenderSessionRow(ws, 80)
	if !strings.Contains(result, "ses_idle") {
		t.Error("expected session ID")
	}
	if !strings.Contains(result, "test") {
		t.Error("expected project")
	}
}

func TestRenderAlertRow_High(t *testing.T) {
	alert := &analyze.WatchAlert{
		Time:      time.Now(),
		SessionID: "ses_abc123",
		Project:   "test",
		Severity:  "high",
		Reason:    "tool_call_loop",
		Detail:    `read_file("src/handler.go") called 6 times`,
	}

	result := RenderAlertRow(alert)
	if !strings.Contains(result, "ses_abc123") {
		t.Error("expected session ID")
	}
	if !strings.Contains(result, "tool_call_loop") {
		t.Error("expected reason")
	}
}

func TestRenderAlertRow_Medium(t *testing.T) {
	alert := &analyze.WatchAlert{
		Time:      time.Now(),
		SessionID: "ses_def456",
		Project:   "test",
		Severity:  "medium",
		Reason:    "file_reread",
		Detail:    "config/settings.json read 3 times",
	}

	result := RenderAlertRow(alert)
	if !strings.Contains(result, "ses_def456") {
		t.Error("expected session ID")
	}
	if !strings.Contains(result, "file_reread") {
		t.Error("expected reason")
	}
}

func TestRenderAlertRow_OldAlert(t *testing.T) {
	alert := &analyze.WatchAlert{
		Time:      time.Now().Add(-10 * time.Minute),
		SessionID: "ses_old",
		Project:   "test",
		Severity:  "low",
		Reason:    "subagent_spawn",
		Detail:    "Subagent spawned",
	}

	result := RenderAlertRow(alert)
	if !strings.Contains(result, "ses_old") {
		t.Error("expected session ID")
	}
}

func TestRenderWatchHeader(t *testing.T) {
	state := analyze.NewWatchState()
	state.Sessions["s1"] = &analyze.WatchSession{
		SessionID: "s1",
		Project:   "test",
		Cost:      1.00,
		StartedAt: time.Now().Add(-10 * time.Minute),
		Idle:      false,
	}
	state.Sessions["s2"] = &analyze.WatchSession{
		SessionID: "s2",
		Project:   "test",
		Cost:      2.00,
		StartedAt: time.Now().Add(-5 * time.Minute),
		Idle:      true,
	}

	result := RenderWatchHeader(state, 5)
	if !strings.Contains(result, "Active:") {
		t.Error("expected Active count in header")
	}
	if !strings.Contains(result, "Today:") {
		t.Error("expected Today cost in header")
	}
}

func TestRenderWatchFooter(t *testing.T) {
	result := RenderWatchFooter(0)
	if !strings.Contains(result, "TAB") {
		t.Error("expected TAB hint")
	}
	if !strings.Contains(result, "Q") {
		t.Error("expected Q hint")
	}
	if !strings.Contains(result, "Sessions") {
		t.Error("expected Sessions pane label")
	}

	result = RenderWatchFooter(1)
	if !strings.Contains(result, "Alerts") {
		t.Error("expected Alerts pane label")
	}
}

func TestFormatWatchDuration(t *testing.T) {
	tests := []struct {
		d    time.Duration
		want string
	}{
		{30 * time.Second, "30s"},
		{5 * time.Minute, "5m0s"},
		{2*time.Hour + 30*time.Minute, "2h30m"},
	}

	for _, tt := range tests {
		got := formatWatchDuration(tt.d)
		if got != tt.want {
			t.Errorf("formatWatchDuration(%v) = %q, want %q", tt.d, got, tt.want)
		}
	}
}

func TestLastActionStr(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"", ""},
		{"read src/handler.go", "read src/handler.go"},
		{"this is a very long action string that exceeds thirty characters", "this is a very long action ..."},
	}

	for _, tt := range tests {
		got := lastActionStr(tt.input)
		if got != tt.expected {
			t.Errorf("lastActionStr(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}
