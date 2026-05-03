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
