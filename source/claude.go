package source

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type ClaudeSource struct {
	projectsDir string
}

type claudeEntry struct {
	Type      string        `json:"type"`
	SessionID string        `json:"sessionId"`
	AgentID   string        `json:"agentId"`
	Message   claudeMessage `json:"message"`
	Timestamp string        `json:"timestamp"`
}

type claudeMessage struct {
	Model   string               `json:"model"`
	Role    string               `json:"role"`
	Usage   *claudeUsage         `json:"usage"`
	Content []claudeContentBlock `json:"content"`
}

type claudeContentBlock struct {
	Type  string          `json:"type"`
	Name  string          `json:"name"`
	Input json.RawMessage `json:"input"`
}

type claudeUsage struct {
	InputTokens              int64 `json:"input_tokens"`
	OutputTokens             int64 `json:"output_tokens"`
	CacheCreationInputTokens int64 `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     int64 `json:"cache_read_input_tokens"`
}

func defaultProjectDir() string {
	if d := os.Getenv("BURNWATCH_CLAUDE_PROJECTS"); d != "" {
		return d
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".claude", "projects")
}

func (s *ClaudeSource) Name() string {
	return "claude-code"
}

func (s *ClaudeSource) Events() (<-chan TokenEvent, <-chan error) {
	events := make(chan TokenEvent)
	errs := make(chan error, 10)

	go func() {
		defer close(errs)
		defer close(events)

		entries, err := os.ReadDir(s.projectsDir)
		if err != nil {
			errs <- fmt.Errorf("read projects dir: %w", err)
			return
		}

		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			s.processProject(filepath.Join(s.projectsDir, entry.Name()), entry.Name(), events, errs)
		}
	}()

	return events, errs
}

func (s *ClaudeSource) processProject(projDir, projName string, events chan<- TokenEvent, errs chan<- error) {
	projectDisplay := projectNameToDisplay(projName)
	projectRoot := claudeProjectRoot(projName)

	sessionFiles, err := filepath.Glob(filepath.Join(projDir, "*.jsonl"))
	if err != nil {
		errs <- fmt.Errorf("glob session files: %w", err)
		return
	}

	for _, sf := range sessionFiles {
		sessionID := strings.TrimSuffix(filepath.Base(sf), ".jsonl")

		f, err := os.Open(sf)
		if err != nil {
			errs <- fmt.Errorf("open session file %s: %w", sf, err)
			continue
		}

		eventIndex := 0
		scanner := bufio.NewScanner(f)
		scanner.Buffer(make([]byte, 0, 1024*1024), 10*1024*1024)
		var parsedErrs []error
		for scanner.Scan() {
			line := scanner.Text()
			if line == "" {
				continue
			}
			ev, ok := s.parseLine(line, projectDisplay, sessionID, "", false, &parsedErrs, projectRoot, &eventIndex)
			if ok {
				events <- ev
			}
		}
		if err := scanner.Err(); err != nil {
			errs <- fmt.Errorf("scanner error in %s: %w", sf, err)
		}
		_ = f.Close()

		for _, e := range parsedErrs {
			errs <- e
		}

		subagentsPath := filepath.Join(projDir, sessionID, "subagents")
		if fi, err := os.Stat(subagentsPath); err == nil && fi.IsDir() {
			s.processSubagents(subagentsPath, projectDisplay, sessionID, events, errs, projectRoot)
		}
	}
}

func (s *ClaudeSource) processSubagents(subagentsPath, projectDisplay, parentSessionID string, events chan<- TokenEvent, errs chan<- error, projectRoot string) {
	subagentFiles, err := filepath.Glob(filepath.Join(subagentsPath, "agent-*.jsonl"))
	if err != nil {
		errs <- fmt.Errorf("glob subagent files: %w", err)
		return
	}

	for _, saf := range subagentFiles {
		fname := filepath.Base(saf)
		agentID := strings.TrimSuffix(fname, ".jsonl")

		f, err := os.Open(saf)
		if err != nil {
			errs <- fmt.Errorf("open subagent file %s: %w", saf, err)
			continue
		}

		eventIndex := 0
		scanner := bufio.NewScanner(f)
		scanner.Buffer(make([]byte, 0, 1024*1024), 10*1024*1024)
		var parsedErrs []error
		for scanner.Scan() {
			line := scanner.Text()
			if line == "" {
				continue
			}
			ev, ok := s.parseLine(line, projectDisplay, parentSessionID, agentID, true, &parsedErrs, projectRoot, &eventIndex)
			if ok {
				events <- ev
			}
		}
		if err := scanner.Err(); err != nil {
			errs <- fmt.Errorf("scanner error in %s: %w", saf, err)
		}
		_ = f.Close()

		for _, e := range parsedErrs {
			errs <- e
		}
	}
}

func (s *ClaudeSource) parseLine(line, project, sessionID, agentID string, isSubagent bool, errs *[]error, projectRoot string, eventIndex *int) (TokenEvent, bool) {
	var entry claudeEntry
	if err := json.Unmarshal([]byte(line), &entry); err != nil {
		*errs = append(*errs, fmt.Errorf("json unmarshal: %w", err))
		return TokenEvent{}, false
	}

	if entry.Type != "assistant" {
		return TokenEvent{}, false
	}

	if entry.Message.Usage == nil {
		*errs = append(*errs, fmt.Errorf("entry has no usage: session %s", entry.SessionID))
		return TokenEvent{}, false
	}

	if entry.Message.Model == "" {
		*errs = append(*errs, fmt.Errorf("entry has no model: session %s", entry.SessionID))
		return TokenEvent{}, false
	}

	ts, err := parseTimestamp(entry.Timestamp)
	if err != nil {
		*errs = append(*errs, fmt.Errorf("parse timestamp %q: %w", entry.Timestamp, err))
	}

	uid := entry.SessionID
	parentUID := ""
	at := ""

	if isSubagent {
		parentUID = sessionID
		if entry.AgentID != "" {
			at = entry.AgentID
		} else {
			at = agentID
		}
	}

	var toolCalls []ToolCall
	var fileOps []FileOp
	for _, block := range entry.Message.Content {
		if block.Type != "tool_use" {
			continue
		}
		args := truncateString(string(block.Input), 1024)
		normalizedName := canonicalizeToolName(block.Name)
		toolCalls = append(toolCalls, ToolCall{
			Name:      normalizedName,
			Arguments: args,
		})
		fo := fileOpFromClaudeTool(block.Name, block.Input, projectRoot)
		if fo != nil {
			fileOps = append(fileOps, *fo)
		}
	}

	messageRole := entry.Message.Role
	if messageRole == "" {
		messageRole = "assistant"
	}

	*eventIndex++
	idx := *eventIndex

	cost, approx, costUnknown := CostForModel(
		entry.Message.Model,
		entry.Message.Usage.InputTokens,
		entry.Message.Usage.OutputTokens,
		entry.Message.Usage.CacheReadInputTokens,
		entry.Message.Usage.CacheCreationInputTokens,
	)

	return TokenEvent{
		SessionID:       uid,
		ParentSessionID: parentUID,
		AgentType:       at,
		Model:           entry.Message.Model,
		Provider:        "anthropic",
		Timestamp:       ts,
		InputTokens:     entry.Message.Usage.InputTokens,
		OutputTokens:    entry.Message.Usage.OutputTokens,
		CacheRead:       entry.Message.Usage.CacheReadInputTokens,
		CacheWrite:      entry.Message.Usage.CacheCreationInputTokens,
		ReasoningTokens: 0,
		CostUSD:         cost,
		CostApproximate: approx,
		CostUnknown:     costUnknown,
		Project:         project,
		Harness:         "claude-code",
		IsSubagent:      isSubagent,
		ToolCalls:       toolCalls,
		FileOps:         fileOps,
		MessageRole:     messageRole,
		EventIndex:      idx,
	}, true
}

func projectNameToDisplay(name string) string {
	s := strings.TrimPrefix(name, "-")
	return strings.ReplaceAll(s, "-", "/")
}

func parseTimestamp(ts string) (time.Time, error) {
	formats := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05.000Z",
		"2006-01-02T15:04:05Z",
	}
	for _, f := range formats {
		t, err := time.Parse(f, ts)
		if err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("unrecognized timestamp format: %q", ts)
}

func canonicalizeToolName(name string) string {
	return strings.ToLower(name)
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}

func claudeProjectRoot(projName string) string {
	s := strings.TrimPrefix(projName, "-")
	s = strings.ReplaceAll(s, "-", "/")
	return "/" + s
}

func NormalizePath(path, projectRoot string) string {
	path = strings.ReplaceAll(path, "\\", "/")
	if projectRoot != "" {
		path = strings.TrimPrefix(path, projectRoot)
		path = strings.TrimPrefix(path, "/")
	}
	path = strings.TrimPrefix(path, "./")
	path = strings.TrimPrefix(path, "/")
	path = filepath.Clean(path)
	if path == "." {
		return ""
	}
	return path
}

type claudeFileInput struct {
	FilePath string `json:"file_path"`
}

func fileOpFromClaudeTool(toolName string, input json.RawMessage, projectRoot string) *FileOp {
	canon := canonicalizeToolName(toolName)
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

	var fi claudeFileInput
	if err := json.Unmarshal(input, &fi); err != nil || fi.FilePath == "" {
		return nil
	}

	return &FileOp{
		Path:      NormalizePath(fi.FilePath, projectRoot),
		Operation: op,
	}
}
