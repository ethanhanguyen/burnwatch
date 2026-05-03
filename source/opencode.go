package source

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

type OpenCodeSource struct {
	dbPath string
}

func (s *OpenCodeSource) Name() string {
	return "opencode"
}

func (s *OpenCodeSource) Events() (<-chan TokenEvent, <-chan error) {
	events := make(chan TokenEvent)
	errs := make(chan error, 10)

	go func() {
		defer close(errs)
		defer close(events)

		dbPath := s.dbPath
		if dbPath == "" {
			dbPath = defaultDBPath()
		}

		db, err := sql.Open("sqlite", dbPath)
		if err != nil {
			errs <- fmt.Errorf("open db: %w", err)
			return
		}
		defer func() { _ = db.Close() }()

		toolParts := make(map[string][]opencodeToolPart)
		partRows, err := db.Query(`SELECT p.message_id, p.data FROM part p WHERE json_extract(p.data, '$.type') = 'tool' ORDER BY p.time_created`)
		if err != nil {
			errs <- fmt.Errorf("part query: %w", err)
		} else {
			for partRows.Next() {
				var msgID, partData string
				if err := partRows.Scan(&msgID, &partData); err != nil {
					errs <- fmt.Errorf("part scan: %w", err)
					continue
				}
				var tp opencodeToolPart
				if err := json.Unmarshal([]byte(partData), &tp); err != nil {
					errs <- fmt.Errorf("part unmarshal %s: %w", msgID, err)
					continue
				}
				toolParts[msgID] = append(toolParts[msgID], tp)
			}
			_ = partRows.Close()
		}

		rows, err := db.Query(`
			SELECT
				s.id,
				s.parent_id,
				s.project_id,
				p.name AS project_name,
				m.id AS message_id,
				m.data AS message_data,
				m.time_created
			FROM message m
			JOIN session s ON s.id = m.session_id
			LEFT JOIN project p ON p.id = s.project_id
			WHERE json_extract(m.data, '$.role') = 'assistant'
			  AND json_extract(m.data, '$.tokens') IS NOT NULL
			ORDER BY m.time_created
		`)
		if err != nil {
			errs <- fmt.Errorf("query: %w", err)
			return
		}
		defer func() { _ = rows.Close() }()

		eventIndex := make(map[string]int)

		for rows.Next() {
			var (
				sessionID, parentID, projectID, projectName sql.NullString
				messageID, messageData                      string
				timeCreated                                 int64
			)
			if err := rows.Scan(&sessionID, &parentID, &projectID, &projectName, &messageID, &messageData, &timeCreated); err != nil {
				errs <- fmt.Errorf("scan: %w", err)
				continue
			}

			var td tokenData
			if err := parseMessageJSON(messageData, &td); err != nil {
				errs <- fmt.Errorf("parse message %s: %w", messageID, err)
				continue
			}

			project := projectName.String
			if project == "" {
				project = projectID.String
			}

			parent := ""
			if parentID.Valid {
				parent = parentID.String
			}

			sid := sessionID.String
			eventIndex[sid]++
			event := s.tokenDataToEvent(td, sid, parent, project, eventIndex[sid], toolParts[messageID])
			events <- event
		}

		if err := rows.Err(); err != nil {
			errs <- fmt.Errorf("rows: %w", err)
		}
	}()

	return events, errs
}

type tokenData struct {
	Role     string     `json:"role"`
	Agent    string     `json:"agent"`
	ModelID  string     `json:"modelID"`
	Provider string     `json:"providerID"`
	Cost     float64    `json:"cost"`
	Tokens   *tokenInfo `json:"tokens"`
	Time     *timeInfo  `json:"time"`
}

type tokenInfo struct {
	Input     int64      `json:"input"`
	Output    int64      `json:"output"`
	Reasoning int64      `json:"reasoning"`
	Cache     *cacheInfo `json:"cache"`
}

type cacheInfo struct {
	Read  int64 `json:"read"`
	Write int64 `json:"write"`
}

type timeInfo struct {
	Created int64 `json:"created"`
}

type opencodeToolPart struct {
	Type  string         `json:"type"`
	Tool  string         `json:"tool"`
	State opencodeState  `json:"state"`
}

type opencodeState struct {
	Input json.RawMessage `json:"input"`
}

func parseMessageJSON(data string, td *tokenData) error {
	if err := json.Unmarshal([]byte(data), td); err != nil {
		return fmt.Errorf("json unmarshal: %w", err)
	}
	return nil
}

func (s *OpenCodeSource) tokenDataToEvent(td tokenData, sessionID, parentSessionID, project string, eventIdx int, toolParts []opencodeToolPart) TokenEvent {
	inputTokens := int64(0)
	outputTokens := int64(0)
	cacheRead := int64(0)
	cacheWrite := int64(0)
	reasoningTokens := int64(0)

	if td.Tokens != nil {
		inputTokens = td.Tokens.Input
		outputTokens = td.Tokens.Output
		reasoningTokens = td.Tokens.Reasoning
		if td.Tokens.Cache != nil {
			cacheRead = td.Tokens.Cache.Read
			cacheWrite = td.Tokens.Cache.Write
		}
	}

	ts := time.Time{}
	if td.Time != nil {
		ts = time.UnixMilli(td.Time.Created)
	}

	var toolCalls []ToolCall
	var fileOps []FileOp
	for _, tp := range toolParts {
		if tp.Type != "tool" {
			continue
		}
		args := truncateString(string(tp.State.Input), 1024)
		normalizedName := canonicalizeToolName(tp.Tool)
		toolCalls = append(toolCalls, ToolCall{
			Name:      normalizedName,
			Arguments: args,
		})
		fo := fileOpFromOpenCodeTool(tp.Tool, tp.State.Input)
		if fo != nil {
			fileOps = append(fileOps, *fo)
		}
	}

	messageRole := td.Role
	if messageRole == "" {
		messageRole = "assistant"
	}

	cost, approx, costUnknown := CostForModel(
		td.ModelID,
		inputTokens,
		outputTokens,
		cacheRead,
		cacheWrite,
	)

	return TokenEvent{
		SessionID:       sessionID,
		ParentSessionID: parentSessionID,
		AgentType:       td.Agent,
		Model:           td.ModelID,
		Provider:        td.Provider,
		Timestamp:       ts,
		InputTokens:     inputTokens,
		OutputTokens:    outputTokens,
		CacheRead:       cacheRead,
		CacheWrite:      cacheWrite,
		ReasoningTokens: reasoningTokens,
		CostUSD:         cost,
		CostApproximate: approx,
		CostUnknown:     costUnknown,
		Project:         project,
		Harness:         "opencode",
		IsSubagent:      parentSessionID != "",
		ToolCalls:       toolCalls,
		FileOps:         fileOps,
		MessageRole:     messageRole,
		EventIndex:      eventIdx,
	}
}

type opencodeFileInput struct {
	FilePath string `json:"filePath"`
}

func fileOpFromOpenCodeTool(toolName string, input json.RawMessage) *FileOp {
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

	var fi opencodeFileInput
	if err := json.Unmarshal(input, &fi); err != nil || fi.FilePath == "" {
		return nil
	}

	return &FileOp{
		Path:      NormalizePath(fi.FilePath, ""),
		Operation: op,
	}
}

func defaultDBPath() string {
	if p := os.Getenv("BURNWATCH_OPENCODE_DB"); p != "" {
		return p
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "share", "opencode", "opencode.db")
}
