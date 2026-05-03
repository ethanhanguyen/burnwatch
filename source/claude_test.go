package source

import (
	"encoding/json"
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

	if len(first.ToolCalls) != 1 {
		t.Errorf("first event ToolCalls count = %d, want 1", len(first.ToolCalls))
	}
	if len(first.ToolCalls) > 0 {
		if first.ToolCalls[0].Name != "read" {
			t.Errorf("first event ToolCall[0].Name = %q, want read", first.ToolCalls[0].Name)
		}
	}
	if len(first.FileOps) != 1 {
		t.Errorf("first event FileOps count = %d, want 1", len(first.FileOps))
	}
	if len(first.FileOps) > 0 {
		if first.FileOps[0].Path != "src/main.go" {
			t.Errorf("first event FileOp[0].Path = %q, want src/main.go", first.FileOps[0].Path)
		}
		if first.FileOps[0].Operation != "read" {
			t.Errorf("first event FileOp[0].Operation = %q, want read", first.FileOps[0].Operation)
		}
	}
	if first.EventIndex != 1 {
		t.Errorf("first event EventIndex = %d, want 1", first.EventIndex)
	}
	if first.MessageRole != "assistant" {
		t.Errorf("first event MessageRole = %q, want assistant", first.MessageRole)
	}

	for _, ev := range got {
		if ev.IsSubagent {
			if len(ev.ToolCalls) == 0 {
				t.Error("subagent event has no ToolCalls")
			}
			if len(ev.FileOps) == 0 {
				t.Error("subagent event has no FileOps")
			}
			if ev.ParentSessionID == "" {
				t.Error("subagent event has empty ParentSessionID")
			}
		}
	}

	if len(got) >= 3 {
		third := got[2]
		if len(third.ToolCalls) != 2 {
			t.Errorf("third event (Edit+Glob) ToolCalls count = %d, want 2", len(third.ToolCalls))
		}
		if len(third.FileOps) != 1 {
			t.Errorf("third event FileOps count = %d, want 1 (Edit only, Glob skipped)", len(third.FileOps))
		}
		if len(third.FileOps) > 0 && third.FileOps[0].Operation != "edit" {
			t.Errorf("third event FileOp[0].Operation = %q, want edit", third.FileOps[0].Operation)
		}
	}

	if len(got) >= 4 {
		textOnly := got[3]
		if len(textOnly.ToolCalls) != 0 {
			t.Errorf("text-only event (index 3) ToolCalls count = %d, want 0", len(textOnly.ToolCalls))
		}
		if len(textOnly.FileOps) != 0 {
			t.Errorf("text-only event (index 3) FileOps count = %d, want 0", len(textOnly.FileOps))
		}
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
				if ev.EventIndex != 1 {
					t.Errorf("EventIndex = %d, want 1", ev.EventIndex)
				}
				if ev.MessageRole != "assistant" {
					t.Errorf("MessageRole = %q, want assistant", ev.MessageRole)
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
			ei := 0
			ev, ok := s.parseLine(tt.line, "test-project", "s1", "", false, &errs, "/test", &ei)

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

func TestClaudeSource_ToolCallParsing(t *testing.T) {
	s := &ClaudeSource{projectsDir: "/nope"}
	line := `{"type":"assistant","sessionId":"s1","message":{"model":"claude-sonnet-4-5-20250929","role":"assistant","content":[{"type":"thinking","text":"hmm"},{"type":"tool_use","id":"t1","name":"Read","input":{"file_path":"/Users/hoang/proj/main.go"}},{"type":"tool_use","id":"t2","name":"Write","input":{"file_path":"/Users/hoang/proj/app.js"}}],"usage":{"input_tokens":10,"cache_creation_input_tokens":0,"cache_read_input_tokens":0,"output_tokens":50}},"timestamp":"2026-01-01T00:00:00Z"}`
	var errs []error
	ei := 0
	ev, ok := s.parseLine(line, "test", "s1", "", false, &errs, "/Users/hoang/proj", &ei)
	if !ok {
		t.Fatal("expected event")
	}

	if len(ev.ToolCalls) != 2 {
		t.Fatalf("expected 2 ToolCalls, got %d", len(ev.ToolCalls))
	}
	if ev.ToolCalls[0].Name != "read" {
		t.Errorf("ToolCall[0].Name = %q, want read", ev.ToolCalls[0].Name)
	}
	if ev.ToolCalls[1].Name != "write" {
		t.Errorf("ToolCall[1].Name = %q, want write", ev.ToolCalls[1].Name)
	}

	if len(ev.FileOps) != 2 {
		t.Fatalf("expected 2 FileOps, got %d", len(ev.FileOps))
	}
	if ev.FileOps[0].Path != "main.go" {
		t.Errorf("FileOps[0].Path = %q, want main.go", ev.FileOps[0].Path)
	}
	if ev.FileOps[0].Operation != "read" {
		t.Errorf("FileOps[0].Operation = %q, want read", ev.FileOps[0].Operation)
	}
	if ev.FileOps[1].Path != "app.js" {
		t.Errorf("FileOps[1].Path = %q, want app.js", ev.FileOps[1].Path)
	}
	if ev.FileOps[1].Operation != "write" {
		t.Errorf("FileOps[1].Operation = %q, want write", ev.FileOps[1].Operation)
	}
}

func TestClaudeSource_ArgumentsTruncation(t *testing.T) {
	s := &ClaudeSource{projectsDir: "/nope"}
	longArg := ""
	for i := 0; i < 2000; i++ {
		longArg += "x"
	}
	line := `{"type":"assistant","sessionId":"s1","message":{"model":"claude-sonnet-4-5-20250929","role":"assistant","content":[{"type":"tool_use","id":"t1","name":"Read","input":{"file_path":"/Users/hoang/proj/main.go","extra":"` + longArg + `"}}],"usage":{"input_tokens":10,"cache_creation_input_tokens":0,"cache_read_input_tokens":0,"output_tokens":50}},"timestamp":"2026-01-01T00:00:00Z"}`
	var errs []error
	ei := 0
	ev, ok := s.parseLine(line, "test", "s1", "", false, &errs, "/Users/hoang/proj", &ei)
	if !ok {
		t.Fatal("expected event")
	}
	if len(ev.ToolCalls) != 1 {
		t.Fatal("expected 1 ToolCall")
	}
	if len(ev.ToolCalls[0].Arguments) > 1024 {
		t.Errorf("Arguments should be truncated to 1024, got %d", len(ev.ToolCalls[0].Arguments))
	}
}

func TestClaudeSource_FileOpMapping(t *testing.T) {
	tests := []struct {
		toolName    string
		input       string
		wantOp      string
		wantPath    string
	}{
		{"Read", `{"file_path":"/Users/hoang/proj/src/main.go"}`, "read", "src/main.go"},
		{"Write", `{"file_path":"/Users/hoang/proj/output.txt"}`, "write", "output.txt"},
		{"Edit", `{"file_path":"/Users/hoang/proj/app.js"}`, "edit", "app.js"},
		{"Glob", `{"pattern":"*.go"}`, "", ""},
		{"Bash", `{"command":"ls"}`, "", ""},
		{"Skill", `{"skill":"foo"}`, "", ""},
	}

	projRoot := "/Users/hoang/proj"
	for _, tt := range tests {
		t.Run(tt.toolName, func(t *testing.T) {
			fo := fileOpFromClaudeTool(tt.toolName, json.RawMessage(tt.input), projRoot)
			if tt.wantOp == "" {
				if fo != nil {
					t.Errorf("expected nil FileOp for %s, got %+v", tt.toolName, *fo)
				}
				return
			}
			if fo == nil {
				t.Fatalf("expected FileOp for %s, got nil", tt.toolName)
			}
			if fo.Operation != tt.wantOp {
				t.Errorf("Operation = %q, want %q", fo.Operation, tt.wantOp)
			}
			if fo.Path != tt.wantPath {
				t.Errorf("Path = %q, want %q", fo.Path, tt.wantPath)
			}
		})
	}
}

func TestClaudeSource_PathNormalization(t *testing.T) {
	tests := []struct {
		path        string
		projectRoot string
		want        string
	}{
		{"/Users/hoang/proj/src/main.go", "/Users/hoang/proj", "src/main.go"},
		{"src/main.go", "", "src/main.go"},
		{"./src/main.go", "", "src/main.go"},
		{"/src/main.go", "", "src/main.go"},
		{`src\app.js`, "", "src/app.js"},
		{"/Users/hoang/proj", "/Users/hoang/proj", ""},
	}

	for _, tt := range tests {
		got := NormalizePath(tt.path, tt.projectRoot)
		if got != tt.want {
			t.Errorf("NormalizePath(%q, %q) = %q, want %q", tt.path, tt.projectRoot, got, tt.want)
		}
	}
}

func TestClaudeSource_CanonicalizeToolName(t *testing.T) {
	if got := canonicalizeToolName("Read"); got != "read" {
		t.Errorf("canonicalizeToolName(Read) = %q, want read", got)
	}
	if got := canonicalizeToolName("WRITE"); got != "write" {
		t.Errorf("canonicalizeToolName(WRITE) = %q, want write", got)
	}
	if got := canonicalizeToolName("edit"); got != "edit" {
		t.Errorf("canonicalizeToolName(edit) = %q, want edit", got)
	}
}

func TestClaudeSource_TruncateString(t *testing.T) {
	s := truncateString("hello", 10)
	if s != "hello" {
		t.Errorf("truncateString(hello, 10) = %q, want hello", s)
	}
	s = truncateString("hello world", 5)
	if s != "hello" {
		t.Errorf("truncateString(hello world, 5) = %q, want hello", s)
	}
}

func TestClaudeSource_ClaudeProjectRoot(t *testing.T) {
	got := claudeProjectRoot("-Users-hoang-burnwatch")
	want := "/Users/hoang/burnwatch"
	if got != want {
		t.Errorf("claudeProjectRoot = %q, want %q", got, want)
	}
}
