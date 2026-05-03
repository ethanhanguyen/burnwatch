package source

import (
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const delta = 0.0001

func TestClaudeSource_Name(t *testing.T) {
	s := &ClaudeSource{projectsDir: "/tmp"}
	if s.Name() != "claude-code" {
		t.Errorf("Name() = %q, want %q", s.Name(), "claude-code")
	}
}

func TestClaudeSource_Events(t *testing.T) {
	tmpDir := t.TempDir()
	projDir := filepath.Join(tmpDir, "-Users-hoang-burnwatch")
	sessionFile := filepath.Join(projDir, "20834312-69fa-4496-8627-64c1865e9bcf.jsonl")
	subagentsDir := filepath.Join(projDir, "20834312-69fa-4496-8627-64c1865e9bcf", "subagents")

	if err := os.MkdirAll(subagentsDir, 0755); err != nil {
		t.Fatal(err)
	}

	src, err := os.ReadFile(filepath.Join("..", "testdata", "claude_projects", "sample-project", "20834312-69fa-4496-8627-64c1865e9bcf.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sessionFile, src, 0644); err != nil {
		t.Fatal(err)
	}

	subSrc, err := os.ReadFile(filepath.Join("..", "testdata", "claude_projects", "sample-project", "20834312-69fa-4496-8627-64c1865e9bcf", "subagents", "agent-aa46fa6e6b4c8fb82.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(subagentsDir, "agent-aa46fa6e6b4c8fb82.jsonl"), subSrc, 0644); err != nil {
		t.Fatal(err)
	}

	s := &ClaudeSource{projectsDir: tmpDir}
	events, errs := s.Events()

	var got []TokenEvent
	for e := range events {
		got = append(got, e)
	}

	var errors []error
	for e := range errs {
		errors = append(errors, e)
	}
	_ = errors

	if len(got) != 6 {
		t.Errorf("expected 6 events (4 top-level assistant + 2 subagent), got %d", len(got))
	}

	first := got[0]
	if first.Model != "claude-sonnet-4-5-20250929" {
		t.Errorf("first event model = %q, want claude-sonnet-4-5-20250929", first.Model)
	}
	if first.Harness != "claude-code" {
		t.Errorf("first event harness = %q, want claude-code", first.Harness)
	}
	if first.InputTokens != 3 {
		t.Errorf("first event input tokens = %d, want 3", first.InputTokens)
	}
	if first.CostUSD <= 0 {
		t.Errorf("first event cost = %f, want > 0", first.CostUSD)
	}
	if first.Project != "Users/hoang/burnwatch" {
		t.Errorf("first event project = %q, want Users/hoang/burnwatch", first.Project)
	}
	if !first.Timestamp.IsZero() {
		t.Logf("first event timestamp = %v", first.Timestamp)
	}

	foundSubagent := false
	for _, ev := range got {
		if ev.IsSubagent {
			foundSubagent = true
			if ev.AgentType == "" {
				t.Error("subagent event has empty AgentType")
			}
			if ev.ParentSessionID == "" {
				t.Error("subagent event has empty ParentSessionID")
			}
		}
	}
	if !foundSubagent {
		t.Error("no subagent events found")
	}

	opusFound := false
	for _, ev := range got {
		if strings.Contains(ev.Model, "opus") {
			opusFound = true
			break
		}
	}
	if !opusFound {
		t.Error("expected at least one Opus event")
	}

	haikuFound := false
	for _, ev := range got {
		if strings.Contains(ev.Model, "haiku") {
			haikuFound = true
			break
		}
	}
	if !haikuFound {
		t.Error("expected at least one Haiku event (from subagent)")
	}
}

func TestClaudeSource_EmptyDir(t *testing.T) {
	tmpDir := t.TempDir()
	s := &ClaudeSource{projectsDir: tmpDir}
	events, errs := s.Events()

	count := 0
	for range events {
		count++
	}
	var errors []error
	for e := range errs {
		errors = append(errors, e)
	}
	_ = errors

	if count != 0 {
		t.Errorf("expected 0 events from empty dir, got %d", count)
	}
}

func TestClaudeSource_EmptyJSONL(t *testing.T) {
	tmpDir := t.TempDir()
	projDir := filepath.Join(tmpDir, "-Users-hoang-test")
	if err := os.MkdirAll(projDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projDir, "empty.jsonl"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	s := &ClaudeSource{projectsDir: tmpDir}
	events, errs := s.Events()

	count := 0
	for range events {
		count++
	}
	var errors []error
	for e := range errs {
		errors = append(errors, e)
	}
	_ = errors

	if count != 0 {
		t.Errorf("expected 0 events from empty JSONL, got %d", count)
	}
}

func TestClaudeSource_FileOnlyNonAssistant(t *testing.T) {
	tmpDir := t.TempDir()
	projDir := filepath.Join(tmpDir, "-Users-hoang-test")
	if err := os.MkdirAll(projDir, 0755); err != nil {
		t.Fatal(err)
	}
	content := `{"type":"user","sessionId":"abc","message":{"role":"user","content":[{"type":"text","text":"hi"}]},"timestamp":"2026-01-01T00:00:00Z"}
{"type":"attachment","sessionId":"abc","message":{"name":"f.txt"},"timestamp":"2026-01-01T00:00:01Z"}
`
	if err := os.WriteFile(filepath.Join(projDir, "test.jsonl"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	s := &ClaudeSource{projectsDir: tmpDir}
	events, errs := s.Events()

	count := 0
	for range events {
		count++
	}
	var errors []error
	for e := range errs {
		errors = append(errors, e)
	}
	_ = errors

	if count != 0 {
		t.Errorf("expected 0 events from non-assistant-only JSONL, got %d", count)
	}
}

func TestClaudeSource_NoSubagentDir(t *testing.T) {
	tmpDir := t.TempDir()
	projDir := filepath.Join(tmpDir, "-Users-hoang-test")
	if err := os.MkdirAll(projDir, 0755); err != nil {
		t.Fatal(err)
	}
	content := `{"type":"assistant","sessionId":"abc","message":{"model":"claude-sonnet-4-5-20250929","usage":{"input_tokens":10,"cache_creation_input_tokens":0,"cache_read_input_tokens":0,"output_tokens":100}},"timestamp":"2026-01-01T00:00:00Z"}
`
	if err := os.WriteFile(filepath.Join(projDir, "abc.jsonl"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	s := &ClaudeSource{projectsDir: tmpDir}
	events, errs := s.Events()

	var got []TokenEvent
	for e := range events {
		got = append(got, e)
	}
	var errors []error
	for e := range errs {
		errors = append(errors, e)
	}

	if len(got) != 1 {
		t.Errorf("expected 1 event, got %d (errors: %v)", len(got), errors)
	}
}

func TestClaudeSource_ParseEntry(t *testing.T) {
	s := &ClaudeSource{projectsDir: "/nope"}

	tests := []struct {
		name     string
		line     string
		wantSkip bool
		wantErr  bool
		check    func(t *testing.T, ev TokenEvent)
	}{
		{
			name: "well-formed assistant",
			line: `{"type":"assistant","sessionId":"s1","message":{"model":"claude-sonnet-4-5-20250929","usage":{"input_tokens":100,"cache_creation_input_tokens":200,"cache_read_input_tokens":300,"output_tokens":400}},"timestamp":"2026-01-01T00:00:00Z"}`,
			check: func(t *testing.T, ev TokenEvent) {
				if ev.SessionID != "s1" {
					t.Errorf("SessionID = %q, want s1", ev.SessionID)
				}
				if ev.InputTokens != 100 {
					t.Errorf("InputTokens = %d, want 100", ev.InputTokens)
				}
				if ev.CacheWrite != 200 {
					t.Errorf("CacheWrite = %d, want 200", ev.CacheWrite)
				}
				if ev.CacheRead != 300 {
					t.Errorf("CacheRead = %d, want 300", ev.CacheRead)
				}
				if ev.OutputTokens != 400 {
					t.Errorf("OutputTokens = %d, want 400", ev.OutputTokens)
				}
				if ev.ReasoningTokens != 0 {
					t.Errorf("ReasoningTokens = %d, want 0", ev.ReasoningTokens)
				}
			},
		},
		{
			name:     "user entry skipped",
			line:     `{"type":"user","sessionId":"s1","message":{"role":"user","content":[{"type":"text","text":"hi"}]},"timestamp":"2026-01-01T00:00:00Z"}`,
			wantSkip: true,
		},
		{
			name:     "queue-operation skipped",
			line:     `{"type":"queue-operation","sessionId":"s1","message":{"operation":"dispatch"},"timestamp":"2026-01-01T00:00:00Z"}`,
			wantSkip: true,
		},
		{
			name:     "attachment skipped",
			line:     `{"type":"attachment","sessionId":"s1","message":{"name":"f.txt"},"timestamp":"2026-01-01T00:00:00Z"}`,
			wantSkip: true,
		},
		{
			name:    "missing usage skipped",
			line:    `{"type":"assistant","sessionId":"s1","message":{"model":"claude-sonnet-4-5-20250929"},"timestamp":"2026-01-01T00:00:00Z"}`,
			wantErr: true,
		},
		{
			name:    "missing model skipped",
			line:    `{"type":"assistant","sessionId":"s1","message":{"usage":{"input_tokens":10,"cache_creation_input_tokens":0,"cache_read_input_tokens":0,"output_tokens":50}},"timestamp":"2026-01-01T00:00:00Z"}`,
			wantErr: true,
		},
		{
			name:    "malformed JSON line",
			line:    `not json at all`,
			wantErr: true,
		},
		{
			name: "zero tokens valid",
			line: `{"type":"assistant","sessionId":"s1","message":{"model":"claude-sonnet-4-5-20250929","usage":{"input_tokens":0,"cache_creation_input_tokens":0,"cache_read_input_tokens":0,"output_tokens":0}},"timestamp":"2026-01-01T00:00:00Z"}`,
			check: func(t *testing.T, ev TokenEvent) {
				if math.Abs(ev.CostUSD) > delta {
					t.Errorf("zero-token event should have zero cost, got %f", ev.CostUSD)
				}
			},
		},
		{
			name: "bad timestamp uses zero",
			line: `{"type":"assistant","sessionId":"s1","message":{"model":"claude-sonnet-4-5-20250929","usage":{"input_tokens":10,"cache_creation_input_tokens":0,"cache_read_input_tokens":0,"output_tokens":10}},"timestamp":"not-a-time"}`,
			check: func(t *testing.T, ev TokenEvent) {
				if !ev.Timestamp.IsZero() {
					t.Errorf("bad timestamp should result in zero time, got %v", ev.Timestamp)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var errs []error
			ev, ok := s.parseLine(tt.line, "test-project", "s1", "", false, &errs)

			if tt.wantSkip && ok {
				t.Error("line should have been skipped")
			}
			if tt.wantErr {
				if ok {
					t.Error("expected error but got an event")
				}
				if len(errs) == 0 {
					t.Error("expected error in error channel")
				}
				return
			}
			if tt.check != nil {
				if !ok {
					t.Fatal("expected event but line was skipped")
				}
				tt.check(t, ev)
			}
		})
	}
}

func TestCostForModel_FromPricingTable(t *testing.T) {
	tcu := float64(tokensPerCostUnit)
	tests := []struct {
		model      string
		input      int64
		output     int64
		cacheRead  int64
		cacheWrite int64
		wantCost   float64
	}{
		{"claude-sonnet-4-5-20250929", 1000, 0, 0, 0, (1000.0 / tcu) * 3.00},
		{"claude-sonnet-4-5-20250929", 0, 1000, 0, 0, (1000.0 / tcu) * 15.00},
		{"claude-sonnet-4-5-20250929", 0, 0, 1000, 0, (1000.0 / tcu) * 0.30},
		{"claude-sonnet-4-5-20250929", 0, 0, 0, 1000, (1000.0 / tcu) * 3.75},
		{"claude-opus-4-5", 1000, 0, 0, 0, (1000.0 / tcu) * 15.00},
		{"claude-opus-4-5", 0, 1000, 0, 0, (1000.0 / tcu) * 75.00},
		{"claude-haiku-4-5", 1000, 0, 0, 0, (1000.0 / tcu) * 0.80},
		{"claude-haiku-4-5", 0, 1000, 0, 0, (1000.0 / tcu) * 4.00},
	}

	for _, tt := range tests {
		got, _, _ := CostForModel(tt.model, tt.input, tt.output, tt.cacheRead, tt.cacheWrite)
		if math.Abs(got-tt.wantCost) > delta {
			t.Errorf("CostForModel(%q, %d, %d, %d, %d) = %f, want %f",
				tt.model, tt.input, tt.output, tt.cacheRead, tt.cacheWrite, got, tt.wantCost)
		}
	}
}

func TestClaudeSource_Discover(t *testing.T) {
	tmpDir := t.TempDir()
	_ = os.Setenv("BURNWATCH_CLAUDE_PROJECTS", tmpDir)
	defer func() { _ = os.Unsetenv("BURNWATCH_CLAUDE_PROJECTS") }()

	projDir := filepath.Join(tmpDir, "-Users-hoang-test")
	if err := os.MkdirAll(projDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projDir, "sess.jsonl"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	sources := Discover()
	found := false
	for _, s := range sources {
		if s.Name() == "claude-code" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Discover() should include Claude Code source when projects dir exists")
	}
}

func TestClaudeSource_DiscoverEmpty(t *testing.T) {
	tmpDir := t.TempDir()
	_ = os.Setenv("BURNWATCH_CLAUDE_PROJECTS", tmpDir+"/nonexistent")
	defer func() { _ = os.Unsetenv("BURNWATCH_CLAUDE_PROJECTS") }()

	sources := Discover()
	for _, s := range sources {
		if s.Name() == "claude-code" {
			t.Error("Discover() should not include Claude Code source when projects dir does not exist")
		}
	}
}

func TestClaudeSource_ProjectsDirNotFound(t *testing.T) {
	s := &ClaudeSource{projectsDir: filepath.Join(t.TempDir(), "nonexistent")}
	events, errs := s.Events()

	var errors []error
	for e := range errs {
		errors = append(errors, e)
	}
	if len(errors) == 0 {
		t.Error("expected error when projects dir does not exist")
	}

	count := 0
	for range events {
		count++
	}
	if count != 0 {
		t.Errorf("expected 0 events, got %d", count)
	}
}

func TestClaudeSource_NonDirEntriesSkipped(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, "README.txt"), []byte("hello"), 0644); err != nil {
		t.Fatal(err)
	}

	s := &ClaudeSource{projectsDir: tmpDir}
	events, errs := s.Events()

	count := 0
	for range events {
		count++
	}
	var errors []error
	for e := range errs {
		errors = append(errors, e)
	}
	_ = errors

	if count != 0 {
		t.Errorf("expected 0 events from dir with only non-directory files, got %d", count)
	}
}

func TestClaudeSource_FileReadError(t *testing.T) {
	tmpDir := t.TempDir()
	projDir := filepath.Join(tmpDir, "-Test")
	if err := os.MkdirAll(projDir, 0755); err != nil {
		t.Fatal(err)
	}
	unreadableFile := filepath.Join(projDir, "unreadable.jsonl")
	if err := os.WriteFile(unreadableFile, nil, 0000); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chmod(unreadableFile, 0644) }()

	s := &ClaudeSource{projectsDir: tmpDir}
	events, errs := s.Events()

	var errors []error
	for e := range errs {
		errors = append(errors, e)
	}
	if len(errors) == 0 {
		t.Error("expected error when session file cannot be opened")
	}

	for range events {
	}
}

func TestClaudeSource_ProjectNameNoLeadingDash(t *testing.T) {
	name := projectNameToDisplay("project-name")
	if name != "project/name" {
		t.Errorf("projectNameToDisplay = %q, want project/name", name)
	}
}
