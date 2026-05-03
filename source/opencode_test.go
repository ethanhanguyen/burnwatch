package source

import (
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func ensureSampleDB(t *testing.T) string {
	t.Helper()
	dbPath := filepath.Join("..", "testdata", "opencode_sample.db")
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Skipf("sample DB not found at %s", dbPath)
	}
	return dbPath
}

func TestOpenCodeSource_Events(t *testing.T) {
	dbPath := ensureSampleDB(t)

	s := &OpenCodeSource{dbPath: dbPath}
	events, errs := s.Events()

	var eventList []TokenEvent
	var errList []error
	done := make(chan struct{})
	go func() {
		for e := range events {
			eventList = append(eventList, e)
		}
		close(done)
	}()

	for e := range errs {
		errList = append(errList, e)
	}
	<-done

	if len(errList) > 0 {
		t.Logf("non-fatal errors: %v", errList)
	}

	if len(eventList) == 0 {
		t.Fatal("expected events, got none")
	}

	first := eventList[0]
	if first.Harness != "opencode" {
		t.Errorf("first event Harness = %q, want %q", first.Harness, "opencode")
	}
	if first.SessionID == "" {
		t.Error("first event SessionID is empty")
	}
	if first.CostUSD < 0 {
		t.Errorf("first event CostUSD = %f, want >= 0", first.CostUSD)
	}
	if first.Project == "" {
		t.Error("first event Project is empty")
	}

	last := eventList[len(eventList)-1]
	if last.Harness != "opencode" {
		t.Errorf("last event Harness = %q, want %q", last.Harness, "opencode")
	}

	var subagentCount int
	for _, e := range eventList {
		if e.IsSubagent {
			subagentCount++
			if e.ParentSessionID == "" {
				t.Errorf("subagent event %q has empty ParentSessionID", e.SessionID)
			}
		}
		if e.CostUSD < 0 {
			t.Errorf("event %q: CostUSD = %f, want >= 0", e.SessionID, e.CostUSD)
		}
	}
	if subagentCount == 0 {
		t.Error("expected at least one subagent event")
	}
}

func TestOpenCodeSource_Name(t *testing.T) {
	s := &OpenCodeSource{}
	if name := s.Name(); name != "opencode" {
		t.Errorf("Name() = %q, want %q", name, "opencode")
	}
}

func TestOpenCodeSource_Discover(t *testing.T) {
	sampleDB := ensureSampleDB(t)

	origPath := sampleDB

	sources := Discover()

	var found *OpenCodeSource
	for _, src := range sources {
		if s, ok := src.(*OpenCodeSource); ok {
			found = s
			break
		}
	}

	if found == nil {
		t.Logf("no OpenCode source found via Discover (normal if DB not at default path)")

		t.Setenv("BURNWATCH_OPENCODE_DB", origPath)
		s := &OpenCodeSource{dbPath: defaultDBPath()}
		if s.Name() != "opencode" {
			t.Errorf("OpenCodeSource.Name() = %q, want %q", s.Name(), "opencode")
		}
		return
	}

	if found.Name() != "opencode" {
		t.Errorf("discovered source Name() = %q, want %q", found.Name(), "opencode")
	}
}

func TestOpenCodeSource_EmptyDB(t *testing.T) {
	f, err := os.CreateTemp("", "empty-*.db")
	if err != nil {
		t.Fatal(err)
	}
	emptyPath := f.Name()
	_ = f.Close()
	defer func() { _ = os.Remove(emptyPath) }()

	s := &OpenCodeSource{dbPath: emptyPath}
	events, errs := s.Events()

	var eventList []TokenEvent
	var errList []error
	done := make(chan struct{})
	go func() {
		for e := range events {
			eventList = append(eventList, e)
		}
		close(done)
	}()

	for e := range errs {
		errList = append(errList, e)
	}
	<-done

	if len(eventList) != 0 {
		t.Errorf("expected 0 events from empty DB, got %d", len(eventList))
	}
	if len(errList) == 0 {
		t.Error("expected errors from empty DB")
	}
}

func TestOpenCodeSource_ParseMessage(t *testing.T) {
	tests := []struct {
		name    string
		data    string
		want    TokenEvent
		wantErr bool
	}{
		{
			name: "well-formed message",
			data: `{
				"role": "assistant",
				"agent": "build",
				"modelID": "google/gemini-3-pro-preview",
				"providerID": "vercel",
				"cost": 0.093594,
				"tokens": {
					"total": 45122,
					"input": 44787,
					"output": 22,
					"reasoning": 313,
					"cache": { "write": 0, "read": 1234 }
				},
				"time": { "created": 1775925856369 }
			}`,
			want: TokenEvent{
				AgentType:       "build",
				Model:           "google/gemini-3-pro-preview",
				Provider:        "vercel",
				Timestamp:       mustParseTime(t, 1775925856369),
				InputTokens:     44787,
				OutputTokens:    22,
				CacheRead:       1234,
				CacheWrite:      0,
				ReasoningTokens: 313,
				CostUSD:         0.05609375,
				CostApproximate: false,
				Harness:         "opencode",
			},
		},
		{
			name: "missing tokens field",
			data: `{
				"role": "assistant",
				"agent": "general",
				"modelID": "claude-sonnet-4-5",
				"providerID": "anthropic",
				"cost": 0.50,
				"time": { "created": 1775925856369 }
			}`,
			wantErr: false,
			want: TokenEvent{
				AgentType: "general",
				Model:     "claude-sonnet-4-5",
				Provider:  "anthropic",
				Timestamp: mustParseTime(t, 1775925856369),
				CostUSD:   0.0,
				Harness:   "opencode",
			},
		},
		{
			name:    "corrupt JSON",
			data:    `{not valid json`,
			wantErr: true,
		},
		{
			name: "missing optional fields",
			data: `{
				"role": "assistant",
				"modelID": "some-model",
				"providerID": "some-provider",
				"tokens": {
					"input": 100,
					"output": 50,
					"cache": {}
				},
				"time": { "created": 1775925856369 }
			}`,
			want: TokenEvent{
				Model:        "some-model",
				Provider:     "some-provider",
				Timestamp:    mustParseTime(t, 1775925856369),
				InputTokens:  100,
				OutputTokens: 50,
				Harness:      "opencode",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result tokenData
			err := parseMessageJSON(tt.data, &result)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			src := &OpenCodeSource{}
			event := src.tokenDataToEvent(result, "sess-1", "", "my-project", 1, nil)

			if event.AgentType != tt.want.AgentType {
				t.Errorf("AgentType = %q, want %q", event.AgentType, tt.want.AgentType)
			}
			if event.Model != tt.want.Model {
				t.Errorf("Model = %q, want %q", event.Model, tt.want.Model)
			}
			if event.Provider != tt.want.Provider {
				t.Errorf("Provider = %q, want %q", event.Provider, tt.want.Provider)
			}
			if !event.Timestamp.Equal(tt.want.Timestamp) {
				t.Errorf("Timestamp = %v, want %v", event.Timestamp, tt.want.Timestamp)
			}
			if event.InputTokens != tt.want.InputTokens {
				t.Errorf("InputTokens = %d, want %d", event.InputTokens, tt.want.InputTokens)
			}
			if event.OutputTokens != tt.want.OutputTokens {
				t.Errorf("OutputTokens = %d, want %d", event.OutputTokens, tt.want.OutputTokens)
			}
			if event.CacheRead != tt.want.CacheRead {
				t.Errorf("CacheRead = %d, want %d", event.CacheRead, tt.want.CacheRead)
			}
			if event.CacheWrite != tt.want.CacheWrite {
				t.Errorf("CacheWrite = %d, want %d", event.CacheWrite, tt.want.CacheWrite)
			}
			if event.ReasoningTokens != tt.want.ReasoningTokens {
				t.Errorf("ReasoningTokens = %d, want %d", event.ReasoningTokens, tt.want.ReasoningTokens)
			}
			if event.CostUSD != tt.want.CostUSD {
				t.Errorf("CostUSD = %f, want %f", event.CostUSD, tt.want.CostUSD)
			}
			if event.Harness != tt.want.Harness {
				t.Errorf("Harness = %q, want %q", event.Harness, tt.want.Harness)
			}
		})
	}
}

func mustParseTime(t *testing.T, unixMilli int64) time.Time {
	t.Helper()
	return time.UnixMilli(unixMilli)
}

func TestOpenCodeSource_ParseMessage_CacheDefaults(t *testing.T) {
	data := `{
		"role": "assistant",
		"agent": "explore",
		"modelID": "google/gemini-3-pro-preview",
		"providerID": "vercel",
		"cost": 1.0,
		"tokens": {
			"input": 100,
			"output": 50
		},
		"time": { "created": 1775925856369 }
	}`

	var result tokenData
	if err := parseMessageJSON(data, &result); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	src := &OpenCodeSource{}
	event := src.tokenDataToEvent(result, "sess-1", "", "test", 1, nil)

	if event.CacheRead != 0 {
		t.Errorf("expected cache read to default to 0, got %d", event.CacheRead)
	}
	if event.CacheWrite != 0 {
		t.Errorf("expected cache write to default to 0, got %d", event.CacheWrite)
	}
	if event.ReasoningTokens != 0 {
		t.Errorf("expected reasoning to default to 0, got %d", event.ReasoningTokens)
	}
}

func TestOpenCodeSource_dbPath(t *testing.T) {
	t.Setenv("BURNWATCH_OPENCODE_DB", "/custom/path/db.sqlite")
	if got := defaultDBPath(); got != "/custom/path/db.sqlite" {
		t.Errorf("defaultDBPath with env = %q, want /custom/path/db.sqlite", got)
	}
}

func TestOpenCodeSource_ProjectNameFallback(t *testing.T) {
	f, err := os.CreateTemp("", "test-*.db")
	if err != nil {
		t.Fatal(err)
	}
	dbPath := f.Name()
	_ = f.Close()
	defer func() { _ = os.Remove(dbPath) }()

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()

	for _, s := range []string{
		`CREATE TABLE "session" ("id" text PRIMARY KEY, "project_id" text NOT NULL, "parent_id" text, "slug" text NOT NULL, "directory" text NOT NULL, "title" text NOT NULL, "version" text NOT NULL, "time_created" integer NOT NULL, "time_updated" integer NOT NULL)`,
		`CREATE TABLE "project" ("id" text PRIMARY KEY, "worktree" text NOT NULL, "name" text, "time_created" integer NOT NULL, "time_updated" integer NOT NULL, "sandboxes" text NOT NULL)`,
		`CREATE TABLE "message" ("id" text PRIMARY KEY, "session_id" text NOT NULL, "time_created" integer NOT NULL, "time_updated" integer NOT NULL, "data" text NOT NULL)`,
	} {
		if _, err := db.Exec(s); err != nil {
			t.Fatal(err)
		}
	}

	_, err = db.Exec(`INSERT INTO session (id, project_id, parent_id, slug, directory, title, version, time_created, time_updated) 
		VALUES ('s1', 'p1', NULL, 'slug1', '/dir', 'Test', '1.0', 1700000000000, 1700000000000)`)
	if err != nil {
		t.Fatal(err)
	}

	_, err = db.Exec(`INSERT INTO session (id, project_id, parent_id, slug, directory, title, version, time_created, time_updated) 
		VALUES ('s2', 'p2', 's1', 'slug2', '/dir2', 'Sub', '1.0', 1700000000001, 1700000000001)`)
	if err != nil {
		t.Fatal(err)
	}

	_, err = db.Exec(`INSERT INTO project (id, worktree, name, time_created, time_updated, sandboxes) 
		VALUES ('p1', '/fake', 'SuperProject', 1700000000000, 1700000000000, '{}')`)
	if err != nil {
		t.Fatal(err)
	}

	msgData := `{"role":"assistant","agent":"general","modelID":"test-model","providerID":"test","cost":1.5,"tokens":{"input":100,"output":50,"cache":{}},"time":{"created":1775925856369}}`

	_, err = db.Exec(`INSERT INTO message (id, session_id, time_created, time_updated, data) VALUES ('m1', 's1', 1700000000000, 1700000000000, ?)`, msgData)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`INSERT INTO message (id, session_id, time_created, time_updated, data) VALUES ('m2', 's2', 1700000000001, 1700000000001, ?)`, msgData)
	if err != nil {
		t.Fatal(err)
	}

	s := &OpenCodeSource{dbPath: dbPath}
	events, errs := s.Events()

	var eventList []TokenEvent
	var errList []error
	done := make(chan struct{})
	go func() {
		for e := range events {
			eventList = append(eventList, e)
		}
		close(done)
	}()

	for e := range errs {
		errList = append(errList, e)
	}
	<-done

	if len(errList) > 0 {
		t.Logf("non-fatal errors: %v", errList)
	}

	if len(eventList) != 2 {
		t.Fatalf("expected 2 events, got %d", len(eventList))
	}

	if eventList[0].Project != "SuperProject" {
		t.Errorf("expected project name 'SuperProject', got %q", eventList[0].Project)
	}

	if eventList[1].Project != "p2" {
		t.Errorf("expected project ID fallback 'p2', got %q", eventList[1].Project)
	}

	if !eventList[1].IsSubagent {
		t.Error("expected session s2 to be a subagent")
	}

	if eventList[1].ParentSessionID != "s1" {
		t.Errorf("expected parent session 's1', got %q", eventList[1].ParentSessionID)
	}
}

func TestOpenCodeSource_WithToolParts(t *testing.T) {
	f, err := os.CreateTemp("", "test-*.db")
	if err != nil {
		t.Fatal(err)
	}
	dbPath := f.Name()
	_ = f.Close()
	defer func() { _ = os.Remove(dbPath) }()

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()

	for _, s := range []string{
		`CREATE TABLE "session" ("id" text PRIMARY KEY, "project_id" text NOT NULL, "parent_id" text, "slug" text NOT NULL, "directory" text NOT NULL, "title" text NOT NULL, "version" text NOT NULL, "time_created" integer NOT NULL, "time_updated" integer NOT NULL)`,
		`CREATE TABLE "project" ("id" text PRIMARY KEY, "worktree" text NOT NULL, "name" text, "time_created" integer NOT NULL, "time_updated" integer NOT NULL, "sandboxes" text NOT NULL)`,
		`CREATE TABLE "message" ("id" text PRIMARY KEY, "session_id" text NOT NULL, "time_created" integer NOT NULL, "time_updated" integer NOT NULL, "data" text NOT NULL)`,
		`CREATE TABLE "part" ("id" text PRIMARY KEY, "message_id" text NOT NULL, "time_created" integer NOT NULL, "time_updated" integer NOT NULL, "data" text NOT NULL)`,
	} {
		if _, err := db.Exec(s); err != nil {
			t.Fatal(err)
		}
	}

	_, err = db.Exec(`INSERT INTO session (id, project_id, parent_id, slug, directory, title, version, time_created, time_updated) 
		VALUES ('s1', 'p1', NULL, 'slug1', '/dir', 'Test', '1.0', 1700000000000, 1700000000000)`)
	if err != nil {
		t.Fatal(err)
	}

	_, err = db.Exec(`INSERT INTO project (id, worktree, name, time_created, time_updated, sandboxes) 
		VALUES ('p1', '/fake', 'TestProject', 1700000000000, 1700000000000, '{}')`)
	if err != nil {
		t.Fatal(err)
	}

	msgData := `{"role":"assistant","agent":"general","modelID":"test-model","providerID":"test","tokens":{"input":100,"output":50,"cache":{}},"time":{"created":1775925856369}}`
	_, err = db.Exec(`INSERT INTO message (id, session_id, time_created, time_updated, data) VALUES ('m1', 's1', 1700000000000, 1700000000000, ?)`, msgData)
	if err != nil {
		t.Fatal(err)
	}

	partData1 := `{"type":"tool","tool":"read","state":{"input":{"filePath":"src/main.go"}}}`
	partData2 := `{"type":"tool","tool":"write","state":{"input":{"filePath":"src/app.js"}}}`
	_, err = db.Exec(`INSERT INTO part (id, message_id, time_created, time_updated, data) VALUES ('p1', 'm1', 1700000000001, 1700000000001, ?)`, partData1)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`INSERT INTO part (id, message_id, time_created, time_updated, data) VALUES ('p2', 'm1', 1700000000002, 1700000000002, ?)`, partData2)
	if err != nil {
		t.Fatal(err)
	}

	s := &OpenCodeSource{dbPath: dbPath}
	events, errs := s.Events()

	var eventList []TokenEvent
	var errList []error
	done := make(chan struct{})
	go func() {
		for e := range events {
			eventList = append(eventList, e)
		}
		close(done)
	}()

	for e := range errs {
		errList = append(errList, e)
	}
	<-done

	if len(errList) > 0 {
		t.Fatalf("unexpected errors: %v", errList)
	}
	if len(eventList) != 1 {
		t.Fatalf("expected 1 event, got %d", len(eventList))
	}

	ev := eventList[0]
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
	if ev.FileOps[0].Path != "src/main.go" {
		t.Errorf("FileOps[0].Path = %q, want src/main.go", ev.FileOps[0].Path)
	}
	if ev.FileOps[0].Operation != "read" {
		t.Errorf("FileOps[0].Operation = %q, want read", ev.FileOps[0].Operation)
	}
	if ev.EventIndex != 1 {
		t.Errorf("EventIndex = %d, want 1", ev.EventIndex)
	}
	if ev.MessageRole != "assistant" {
		t.Errorf("MessageRole = %q, want assistant", ev.MessageRole)
	}
}

func TestOpenCodeSource_MissingPartTable(t *testing.T) {
	f, err := os.CreateTemp("", "test-*.db")
	if err != nil {
		t.Fatal(err)
	}
	dbPath := f.Name()
	_ = f.Close()
	defer func() { _ = os.Remove(dbPath) }()

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()

	for _, s := range []string{
		`CREATE TABLE "session" ("id" text PRIMARY KEY, "project_id" text NOT NULL, "parent_id" text, "slug" text NOT NULL, "directory" text NOT NULL, "title" text NOT NULL, "version" text NOT NULL, "time_created" integer NOT NULL, "time_updated" integer NOT NULL)`,
		`CREATE TABLE "project" ("id" text PRIMARY KEY, "worktree" text NOT NULL, "name" text, "time_created" integer NOT NULL, "time_updated" integer NOT NULL, "sandboxes" text NOT NULL)`,
		`CREATE TABLE "message" ("id" text PRIMARY KEY, "session_id" text NOT NULL, "time_created" integer NOT NULL, "time_updated" integer NOT NULL, "data" text NOT NULL)`,
	} {
		if _, err := db.Exec(s); err != nil {
			t.Fatal(err)
		}
	}

	_, err = db.Exec(`INSERT INTO session (id, project_id, parent_id, slug, directory, title, version, time_created, time_updated) 
		VALUES ('s1', 'p1', NULL, 'slug1', '/dir', 'Test', '1.0', 1700000000000, 1700000000000)`)
	if err != nil {
		t.Fatal(err)
	}

	_, err = db.Exec(`INSERT INTO project (id, worktree, name, time_created, time_updated, sandboxes) 
		VALUES ('p1', '/fake', 'TestProject', 1700000000000, 1700000000000, '{}')`)
	if err != nil {
		t.Fatal(err)
	}

	msgData := `{"role":"assistant","agent":"general","modelID":"test-model","providerID":"test","tokens":{"input":100,"output":50,"cache":{}},"time":{"created":1775925856369}}`
	_, err = db.Exec(`INSERT INTO message (id, session_id, time_created, time_updated, data) VALUES ('m1', 's1', 1700000000000, 1700000000000, ?)`, msgData)
	if err != nil {
		t.Fatal(err)
	}

	s := &OpenCodeSource{dbPath: dbPath}
	events, errs := s.Events()

	var eventList []TokenEvent
	var errList []error
	done := make(chan struct{})
	go func() {
		for e := range events {
			eventList = append(eventList, e)
		}
		close(done)
	}()

	for e := range errs {
		errList = append(errList, e)
	}
	<-done

	if len(eventList) != 1 {
		t.Fatalf("expected 1 event without part table, got %d", len(eventList))
	}
	if len(eventList[0].ToolCalls) != 0 {
		t.Errorf("expected 0 ToolCalls without part table, got %d", len(eventList[0].ToolCalls))
	}
	if len(eventList[0].FileOps) != 0 {
		t.Errorf("expected 0 FileOps without part table, got %d", len(eventList[0].FileOps))
	}

	foundPartErr := false
	for _, e := range errList {
		if strings.Contains(e.Error(), "part") {
			foundPartErr = true
			break
		}
	}
	if !foundPartErr {
		t.Errorf("expected part query error, got errors: %v", errList)
	}
}

func TestOpenCodeSource_PartParseFailure(t *testing.T) {
	f, err := os.CreateTemp("", "test-*.db")
	if err != nil {
		t.Fatal(err)
	}
	dbPath := f.Name()
	_ = f.Close()
	defer func() { _ = os.Remove(dbPath) }()

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()

	for _, s := range []string{
		`CREATE TABLE "session" ("id" text PRIMARY KEY, "project_id" text NOT NULL, "parent_id" text, "slug" text NOT NULL, "directory" text NOT NULL, "title" text NOT NULL, "version" text NOT NULL, "time_created" integer NOT NULL, "time_updated" integer NOT NULL)`,
		`CREATE TABLE "project" ("id" text PRIMARY KEY, "worktree" text NOT NULL, "name" text, "time_created" integer NOT NULL, "time_updated" integer NOT NULL, "sandboxes" text NOT NULL)`,
		`CREATE TABLE "message" ("id" text PRIMARY KEY, "session_id" text NOT NULL, "time_created" integer NOT NULL, "time_updated" integer NOT NULL, "data" text NOT NULL)`,
		`CREATE TABLE "part" ("id" text PRIMARY KEY, "message_id" text NOT NULL, "time_created" integer NOT NULL, "time_updated" integer NOT NULL, "data" text NOT NULL)`,
	} {
		if _, err := db.Exec(s); err != nil {
			t.Fatal(err)
		}
	}

	_, err = db.Exec(`INSERT INTO session (id, project_id, parent_id, slug, directory, title, version, time_created, time_updated) 
		VALUES ('s1', 'p1', NULL, 'slug1', '/dir', 'Test', '1.0', 1700000000000, 1700000000000)`)
	if err != nil {
		t.Fatal(err)
	}

	_, err = db.Exec(`INSERT INTO project (id, worktree, name, time_created, time_updated, sandboxes) 
		VALUES ('p1', '/fake', 'TestProject', 1700000000000, 1700000000000, '{}')`)
	if err != nil {
		t.Fatal(err)
	}

	msgData := `{"role":"assistant","agent":"general","modelID":"test-model","providerID":"test","tokens":{"input":100,"output":50,"cache":{}},"time":{"created":1775925856369}}`
	_, err = db.Exec(`INSERT INTO message (id, session_id, time_created, time_updated, data) VALUES ('m1', 's1', 1700000000000, 1700000000000, ?)`, msgData)
	if err != nil {
		t.Fatal(err)
	}

	corruptPart := `{"type":"tool","tool":"read","state":{"input":`
	_, err = db.Exec(`INSERT INTO part (id, message_id, time_created, time_updated, data) VALUES ('p1', 'm1', 1700000000001, 1700000000001, ?)`, corruptPart)
	if err != nil {
		t.Fatal(err)
	}

	s := &OpenCodeSource{dbPath: dbPath}
	events, errs := s.Events()

	var eventList []TokenEvent
	var errList []error
	done := make(chan struct{})
	go func() {
		for e := range events {
			eventList = append(eventList, e)
		}
		close(done)
	}()

	for e := range errs {
		errList = append(errList, e)
	}
	<-done

	if len(eventList) != 1 {
		t.Fatalf("expected 1 event despite corrupt part data, got %d", len(eventList))
	}
	if len(eventList[0].ToolCalls) != 0 {
		t.Errorf("expected 0 ToolCalls with corrupt part data, got %d", len(eventList[0].ToolCalls))
	}

	foundPartErr := false
	for _, e := range errList {
		if strings.Contains(e.Error(), "malformed") || strings.Contains(e.Error(), "part") {
			foundPartErr = true
			break
		}
	}
	if !foundPartErr {
		t.Errorf("expected part parse error, got errors: %v", errList)
	}
}

func TestOpenCodeSource_FileOpMapping(t *testing.T) {
	tests := []struct {
		toolName string
		input    string
		wantOp   string
		wantPath string
	}{
		{"read", `{"filePath":"src/main.go"}`, "read", "src/main.go"},
		{"write", `{"filePath":"output.txt"}`, "write", "output.txt"},
		{"edit", `{"filePath":"app.js"}`, "edit", "app.js"},
		{"glob", `{"pattern":"*.go"}`, "", ""},
		{"bash", `{"command":"ls"}`, "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.toolName, func(t *testing.T) {
			fo := fileOpFromOpenCodeTool(tt.toolName, json.RawMessage(tt.input))
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
