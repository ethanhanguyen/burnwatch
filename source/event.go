package source

import "time"

type TokenEvent struct {
	SessionID       string
	ParentSessionID string
	AgentType       string
	Model           string
	Provider        string
	Timestamp       time.Time
	InputTokens     int64
	OutputTokens    int64
	CacheRead       int64
	CacheWrite      int64
	ReasoningTokens int64
	CostUSD         float64
	CostApproximate bool
	Project         string
	Harness         string
	IsSubagent      bool
}
