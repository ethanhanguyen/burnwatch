package analyze

import (
	"math"
	"testing"
	"time"

	"github.com/ethanhanguyen/burnwatch/source"
)

const delta = 0.0001

func sessions(e ...int) []source.TokenEvent {
	var events []source.TokenEvent
	for _, s := range e {
		events = append(events, sessionEventSet[s]...)
	}
	return events
}

var sessionEventSet = map[int][]source.TokenEvent{
	1: {
		{SessionID: "s1", Project: "project-a", Harness: "opencode", InputTokens: 100, OutputTokens: 30, CacheRead: 10, CacheWrite: 90, CostUSD: 1.0, Timestamp: time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)},
	},
	2: {
		{SessionID: "s2", Project: "project-a", Harness: "opencode", InputTokens: 100, OutputTokens: 50, CacheRead: 50, CacheWrite: 50, CostUSD: 2.0, Timestamp: time.Date(2026, 5, 1, 11, 0, 0, 0, time.UTC)},
	},
	3: {
		{SessionID: "s3", Project: "project-a", Harness: "opencode", InputTokens: 100, OutputTokens: 80, CacheRead: 90, CacheWrite: 10, CostUSD: 3.0, Timestamp: time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)},
	},
	4: {
		{SessionID: "s4", Project: "project-b", Harness: "claude-code", InputTokens: 200, OutputTokens: 100, CacheRead: 20, CacheWrite: 180, CostUSD: 4.0, Timestamp: time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)},
	},
	5: {
		{SessionID: "s5", Project: "project-b", Harness: "claude-code", InputTokens: 200, OutputTokens: 120, CacheRead: 100, CacheWrite: 100, CostUSD: 5.0, Timestamp: time.Date(2026, 5, 1, 11, 0, 0, 0, time.UTC)},
	},
	6: {
		{SessionID: "s6", Project: "project-b", Harness: "claude-code", InputTokens: 200, OutputTokens: 160, CacheRead: 180, CacheWrite: 20, CostUSD: 6.0, Timestamp: time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)},
	},
}

func TestComputeBaselinesMultipleProjects(t *testing.T) {
	events := sessions(1, 2, 3, 4, 5, 6)
	result := ComputeBaselines(events)

	if len(result) != 3 {
		t.Fatalf("expected 3 baselines (2 project + 1 global), got %d", len(result))
	}

	pa := result["project-a:opencode"]
	if pa.SessionCount != 3 {
		t.Errorf("project-a session count = %d, want 3", pa.SessionCount)
	}
	if math.Abs(pa.CostMean-2.0) > delta {
		t.Errorf("project-a cost mean = %f, want 2.0", pa.CostMean)
	}

	if math.Abs(pa.CostStd-0.8165) > 0.001 {
		t.Errorf("project-a cost std = %f, want ~0.8165", pa.CostStd)
	}

	if math.Abs(pa.RatioP10-0.34) > delta {
		t.Errorf("project-a ratio P10 = %f, want 0.34", pa.RatioP10)
	}
	if math.Abs(pa.RatioP50-0.5) > delta {
		t.Errorf("project-a ratio P50 = %f, want 0.5", pa.RatioP50)
	}
	if math.Abs(pa.RatioP90-0.74) > delta {
		t.Errorf("project-a ratio P90 = %f, want 0.74", pa.RatioP90)
	}

	if math.Abs(pa.CacheP10-0.18) > delta {
		t.Errorf("project-a cache P10 = %f, want 0.18", pa.CacheP10)
	}
	if math.Abs(pa.CacheP50-0.5) > delta {
		t.Errorf("project-a cache P50 = %f, want 0.5", pa.CacheP50)
	}

	pb := result["project-b:claude-code"]
	if pb.SessionCount != 3 {
		t.Errorf("project-b session count = %d, want 3", pb.SessionCount)
	}
	if math.Abs(pb.CostMean-5.0) > delta {
		t.Errorf("project-b cost mean = %f, want 5.0", pb.CostMean)
	}

	if math.Abs(pb.RatioP50-0.6) > delta {
		t.Errorf("project-b ratio P50 = %f, want 0.6", pb.RatioP50)
	}

	global := result["*"]
	if global.SessionCount != 6 {
		t.Errorf("global session count = %d, want 6", global.SessionCount)
	}
	if math.Abs(global.CostMean-3.5) > delta {
		t.Errorf("global cost mean = %f, want 3.5", global.CostMean)
	}
	if math.Abs(global.RatioP50-0.55) > delta {
		t.Errorf("global ratio P50 = %f, want 0.55", global.RatioP50)
	}
}

func TestComputeBaselinesSingleSession(t *testing.T) {
	events := sessions(1)
	result := ComputeBaselines(events)

	pa := result["project-a:opencode"]
	if pa.SessionCount != 1 {
		t.Errorf("session count = %d, want 1", pa.SessionCount)
	}
	if math.Abs(pa.CostMean-1.0) > delta {
		t.Errorf("cost mean = %f, want 1.0", pa.CostMean)
	}
	if math.Abs(pa.CostStd) > delta {
		t.Errorf("cost std = %f, want 0 (single session)", pa.CostStd)
	}
	if math.Abs(pa.RatioP10-0.3) > delta {
		t.Errorf("ratio P10 = %f, want 0.3", pa.RatioP10)
	}
	if math.Abs(pa.RatioP50-0.3) > delta {
		t.Errorf("ratio P50 = %f, want 0.3", pa.RatioP50)
	}
	if math.Abs(pa.RatioP90-0.3) > delta {
		t.Errorf("ratio P90 = %f, want 0.3", pa.RatioP90)
	}
}

func TestComputeBaselinesAllIdentical(t *testing.T) {
	s1 := []source.TokenEvent{
		{SessionID: "s1", Project: "p", Harness: "h", InputTokens: 100, OutputTokens: 50, CacheRead: 10, CacheWrite: 10, CostUSD: 1.0, Timestamp: time.Now()},
	}
	s2 := []source.TokenEvent{
		{SessionID: "s2", Project: "p", Harness: "h", InputTokens: 100, OutputTokens: 50, CacheRead: 10, CacheWrite: 10, CostUSD: 1.0, Timestamp: time.Now()},
	}
	events := append(s1, s2...)
	result := ComputeBaselines(events)

	b := result["p:h"]
	if math.Abs(b.CostStd) > delta {
		t.Errorf("cost std = %f, want 0 (all identical)", b.CostStd)
	}
	if math.Abs(b.RatioP10-0.5) > delta {
		t.Errorf("ratio P10 = %f, want 0.5", b.RatioP10)
	}
	if math.Abs(b.RatioP50-0.5) > delta {
		t.Errorf("ratio P50 = %f, want 0.5", b.RatioP50)
	}
}

func TestComputeBaselinesEmptyInput(t *testing.T) {
	result := ComputeBaselines(nil)
	if len(result) != 0 {
		t.Errorf("expected empty map for nil input, got %d entries", len(result))
	}

	result = ComputeBaselines([]source.TokenEvent{})
	if len(result) != 0 {
		t.Errorf("expected empty map for empty slice, got %d entries", len(result))
	}
}

func TestComputeBaselinesZeroInput(t *testing.T) {
	events := []source.TokenEvent{
		{SessionID: "s1", Project: "p", Harness: "h", InputTokens: 0, OutputTokens: 100, CacheRead: 0, CacheWrite: 0, CostUSD: 1.0, Timestamp: time.Now()},
	}
	result := ComputeBaselines(events)
	b := result["p:h"]

	if math.Abs(b.RatioP10) > delta {
		t.Errorf("ratio P10 = %f, want 0 (zero input)", b.RatioP10)
	}
}

func TestComputeBaselinesNegativeTokens(t *testing.T) {
	events := []source.TokenEvent{
		{SessionID: "s1", Project: "p", Harness: "h", InputTokens: -100, OutputTokens: -50, CacheRead: -10, CacheWrite: -10, CostUSD: 1.0, Timestamp: time.Now()},
	}
	result := ComputeBaselines(events)
	b := result["p:h"]

	if b.RatioP10 != 0 {
		t.Errorf("ratio P10 = %f, want 0 (negative input clamped to 0)", b.RatioP10)
	}
}

func TestComputeBaselinesMultiEventSession(t *testing.T) {
	events := []source.TokenEvent{
		{SessionID: "s1", Project: "p", Harness: "h", InputTokens: 50, OutputTokens: 25, CostUSD: 0.5, Timestamp: time.Now()},
		{SessionID: "s1", Project: "p", Harness: "h", InputTokens: 50, OutputTokens: 25, CostUSD: 0.5, Timestamp: time.Now()},
	}
	result := ComputeBaselines(events)
	b := result["p:h"]

	if math.Abs(b.CostMean-1.0) > delta {
		t.Errorf("cost mean = %f, want 1.0 (two events in one session)", b.CostMean)
	}
	if math.Abs(b.RatioP50-0.5) > delta {
		t.Errorf("ratio P50 = %f, want 0.5", b.RatioP50)
	}
}

func TestComputeBaselinesSortsSessionCosts(t *testing.T) {
	events := sessions(3, 1, 2)
	result := ComputeBaselines(events)
	b := result["project-a:opencode"]

	if len(b.SessionCosts) != 3 {
		t.Fatalf("expected 3 session costs, got %d", len(b.SessionCosts))
	}
	if math.Abs(b.SessionCosts[0]-1.0) > delta {
		t.Errorf("SessionCosts[0] = %f, want 1.0", b.SessionCosts[0])
	}
	if math.Abs(b.SessionCosts[1]-2.0) > delta {
		t.Errorf("SessionCosts[1] = %f, want 2.0", b.SessionCosts[1])
	}
	if math.Abs(b.SessionCosts[2]-3.0) > delta {
		t.Errorf("SessionCosts[2] = %f, want 3.0", b.SessionCosts[2])
	}
}

func TestComputeBaselinesTokenStats(t *testing.T) {
	e1 := []source.TokenEvent{
		{SessionID: "s1", Project: "p", Harness: "h", InputTokens: 1000, OutputTokens: 300, CacheRead: 0, CacheWrite: 0, CostUSD: 0, Timestamp: time.Now()},
	}
	e2 := []source.TokenEvent{
		{SessionID: "s2", Project: "p", Harness: "h", InputTokens: 2000, OutputTokens: 600, CacheRead: 0, CacheWrite: 0, CostUSD: 0, Timestamp: time.Now()},
	}
	e3 := []source.TokenEvent{
		{SessionID: "s3", Project: "p", Harness: "h", InputTokens: 3000, OutputTokens: 900, CacheRead: 0, CacheWrite: 0, CostUSD: 0, Timestamp: time.Now()},
	}
	e4 := []source.TokenEvent{
		{SessionID: "s4", Project: "p", Harness: "h", InputTokens: 4000, OutputTokens: 1200, CacheRead: 0, CacheWrite: 0, CostUSD: 0, Timestamp: time.Now()},
	}
	e5 := []source.TokenEvent{
		{SessionID: "s5", Project: "p", Harness: "h", InputTokens: 5000, OutputTokens: 1500, CacheRead: 0, CacheWrite: 0, CostUSD: 0, Timestamp: time.Now()},
	}

	events := append(append(append(append(e1, e2...), e3...), e4...), e5...)
	result := ComputeBaselines(events)
	b := result["p:h"]

	if math.Abs(b.InputMean-3000) > 0.01 {
		t.Errorf("InputMean = %f, want 3000", b.InputMean)
	}
	if math.Abs(b.InputStd-1414.21) > 0.1 {
		t.Errorf("InputStd = %f, want ~1414.21", b.InputStd)
	}
	if math.Abs(b.InputP50-3000) > 0.01 {
		t.Errorf("InputP50 = %f, want 3000", b.InputP50)
	}
	if math.Abs(b.InputP90-4600) > 0.01 {
		t.Errorf("InputP90 = %f, want 4600", b.InputP90)
	}

	if math.Abs(b.OutputMean-900) > 0.01 {
		t.Errorf("OutputMean = %f, want 900", b.OutputMean)
	}
	if math.Abs(b.OutputStd-424.26) > 0.1 {
		t.Errorf("OutputStd = %f, want ~424.26", b.OutputStd)
	}
	if math.Abs(b.OutputP50-900) > 0.01 {
		t.Errorf("OutputP50 = %f, want 900", b.OutputP50)
	}
	if math.Abs(b.OutputP90-1380) > 0.01 {
		t.Errorf("OutputP90 = %f, want 1380", b.OutputP90)
	}

	if math.Abs(b.TERP10-0.3) > delta {
		t.Errorf("TERP10 = %f, want 0.3", b.TERP10)
	}
}

func TestComputeBaselinesTER(t *testing.T) {
	events := []source.TokenEvent{
		{SessionID: "s1", Project: "p", Harness: "h", InputTokens: 100000, OutputTokens: 50000, CacheRead: 10000, CacheWrite: 5000, CostUSD: 0, Timestamp: time.Now()},
	}
	result := ComputeBaselines(events)
	b := result["p:h"]

	expected := 60000.0 / 105000.0
	if math.Abs(b.TERP10-expected) > delta {
		t.Errorf("TERP10 = %f, want %f (TER = (out+cacheRead)/(in+cacheWrite))", b.TERP10, expected)
	}
}

func TestComputeBaselinesTokenStatsZeroInput(t *testing.T) {
	events := []source.TokenEvent{
		{SessionID: "s1", Project: "p", Harness: "h", InputTokens: 0, OutputTokens: 0, CacheRead: 0, CacheWrite: 0, CostUSD: 0, Timestamp: time.Now()},
	}
	result := ComputeBaselines(events)
	b := result["p:h"]

	if math.Abs(b.InputMean) > delta {
		t.Errorf("InputMean = %f, want 0", b.InputMean)
	}
	if math.Abs(b.InputStd) > delta {
		t.Errorf("InputStd = %f, want 0", b.InputStd)
	}
	if math.Abs(b.InputP50) > delta {
		t.Errorf("InputP50 = %f, want 0", b.InputP50)
	}
	if math.Abs(b.InputP90) > delta {
		t.Errorf("InputP90 = %f, want 0", b.InputP90)
	}
	if math.Abs(b.OutputMean) > delta {
		t.Errorf("OutputMean = %f, want 0", b.OutputMean)
	}
	if math.Abs(b.OutputStd) > delta {
		t.Errorf("OutputStd = %f, want 0", b.OutputStd)
	}
	if math.Abs(b.TERP10) > delta {
		t.Errorf("TERP10 = %f, want 0", b.TERP10)
	}
}

func TestComputeBaselinesTokenStatsSingleSession(t *testing.T) {
	events := []source.TokenEvent{
		{SessionID: "s1", Project: "p", Harness: "h", InputTokens: 500, OutputTokens: 200, CacheRead: 0, CacheWrite: 0, CostUSD: 0, Timestamp: time.Now()},
	}
	result := ComputeBaselines(events)
	b := result["p:h"]

	if math.Abs(b.InputMean-500) > delta {
		t.Errorf("InputMean = %f, want 500", b.InputMean)
	}
	if math.Abs(b.InputStd) > delta {
		t.Errorf("InputStd = %f, want 0 (single session)", b.InputStd)
	}
	if math.Abs(b.OutputMean-200) > delta {
		t.Errorf("OutputMean = %f, want 200", b.OutputMean)
	}
	if math.Abs(b.OutputStd) > delta {
		t.Errorf("OutputStd = %f, want 0 (single session)", b.OutputStd)
	}
}
