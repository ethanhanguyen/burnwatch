package analyze

import (
	"testing"
	"time"

	"github.com/ethanhanguyen/burnwatch/source"
)

func TestComputeTrends_Empty(t *testing.T) {
	tr := ComputeTrends(nil)
	if tr != nil {
		t.Error("expected nil for nil input")
	}

	tr = ComputeTrends([]source.TokenEvent{})
	if tr != nil {
		t.Error("expected nil for empty input")
	}
}

func TestComputeTrends_SingleWeek(t *testing.T) {
	events := []source.TokenEvent{
		{SessionID: "s1", CostUSD: 100, InputTokens: 1000, OutputTokens: 100, Timestamp: time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)},
		{SessionID: "s1", CostUSD: 50, InputTokens: 500, OutputTokens: 50, Timestamp: time.Date(2026, 5, 1, 11, 0, 0, 0, time.UTC)},
		{SessionID: "s2", CostUSD: 20, InputTokens: 200, OutputTokens: 40, Timestamp: time.Date(2026, 5, 3, 12, 0, 0, 0, time.UTC)},
	}
	tr := ComputeTrends(events)
	if tr != nil {
		t.Error("expected nil for single week of data")
	}
}

func TestComputeTrends_TwoWeeks(t *testing.T) {
	events := []source.TokenEvent{
		{SessionID: "s1", CostUSD: 100, InputTokens: 1000, OutputTokens: 100, Timestamp: time.Date(2026, 5, 4, 10, 0, 0, 0, time.UTC)},
		{SessionID: "s2", CostUSD: 80, InputTokens: 800, OutputTokens: 80, Timestamp: time.Date(2026, 5, 11, 10, 0, 0, 0, time.UTC)},
	}

	tr := ComputeTrends(events)
	if tr == nil {
		t.Fatal("expected trend")
	}

	if tr.First.TotalCost != 100 {
		t.Errorf("first week cost = %f, want 100", tr.First.TotalCost)
	}
	if tr.Last.TotalCost != 80 {
		t.Errorf("last week cost = %f, want 80", tr.Last.TotalCost)
	}
	if tr.CostDirection != "↓" {
		t.Errorf("CostDirection = %s, want ↓", tr.CostDirection)
	}
	if tr.CostChange != -20.0 {
		t.Errorf("CostChange = %f, want -20.0", tr.CostChange)
	}
}

func TestComputeTrends_RatioUp(t *testing.T) {
	events := []source.TokenEvent{
		{SessionID: "s1", CostUSD: 100, InputTokens: 1000, OutputTokens: 100, Timestamp: time.Date(2026, 5, 4, 10, 0, 0, 0, time.UTC)},
		{SessionID: "s2", CostUSD: 100, InputTokens: 800, OutputTokens: 120, Timestamp: time.Date(2026, 5, 11, 10, 0, 0, 0, time.UTC)},
	}
	tr := ComputeTrends(events)
	if tr == nil {
		t.Fatal("expected trend")
	}

	if tr.RatioDirection != "↑" {
		t.Errorf("RatioDirection = %s, want ↑", tr.RatioDirection)
	}
	if tr.RatioChange <= 0 {
		t.Errorf("RatioChange = %f, want positive", tr.RatioChange)
	}
}

func TestComputeTrends_NoChange(t *testing.T) {
	events := []source.TokenEvent{
		{SessionID: "s1", CostUSD: 100, InputTokens: 1000, OutputTokens: 100, Timestamp: time.Date(2026, 5, 4, 10, 0, 0, 0, time.UTC)},
		{SessionID: "s2", CostUSD: 100, InputTokens: 500, OutputTokens: 50, Timestamp: time.Date(2026, 5, 4, 11, 0, 0, 0, time.UTC)},
		{SessionID: "s3", CostUSD: 200, InputTokens: 1500, OutputTokens: 150, Timestamp: time.Date(2026, 5, 11, 10, 0, 0, 0, time.UTC)},
	}

	tr := ComputeTrends(events)
	if tr == nil {
		t.Fatal("expected trend")
	}

	if tr.CostDirection != "→" {
		t.Errorf("CostDirection = %s, want →", tr.CostDirection)
	}
	if tr.RatioDirection != "→" {
		t.Errorf("RatioDirection = %s, want →", tr.RatioDirection)
	}
}

func TestWeeklyAggregation(t *testing.T) {
	events := []source.TokenEvent{
		{SessionID: "s1", CostUSD: 10, InputTokens: 100, OutputTokens: 10, Timestamp: time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)},
		{SessionID: "s2", CostUSD: 20, InputTokens: 200, OutputTokens: 20, Timestamp: time.Date(2026, 5, 3, 10, 0, 0, 0, time.UTC)},
		{SessionID: "s3", CostUSD: 30, InputTokens: 300, OutputTokens: 30, Timestamp: time.Date(2026, 5, 5, 10, 0, 0, 0, time.UTC)},
		{SessionID: "s4", CostUSD: 40, InputTokens: 400, OutputTokens: 40, Timestamp: time.Date(2026, 5, 6, 10, 0, 0, 0, time.UTC)},
	}
	tr := ComputeTrends(events)
	if tr == nil {
		t.Fatal("expected trend")
	}
	_ = tr.Format()
}

func TestWeekStartOf(t *testing.T) {
	friday := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	ws := weekStartOf(friday)
	if ws.Weekday() != time.Monday {
		t.Errorf("weekStartOf returned %s, want Monday", ws.Weekday())
	}
	if ws.Day() != 27 {
		t.Errorf("weekStartOf day = %d, want 27 (April 27 is Monday before May 1)", ws.Day())
	}
}
