package analyze

import (
	"strings"
	"testing"
)

func TestGenerateRecommendationsCostOutlier(t *testing.T) {
	signals := []WasteSignal{
		{SessionID: "s1", Project: "p", Severity: "high", Reason: "cost_outlier", Detail: "cost outlier detail", Metric: 10.0, Threshold: 5.0},
	}
	baselines := map[string]Baseline{
		"p:h": {CostMean: 2.0, CostStd: 1.0},
	}
	recs := GenerateRecommendations(signals, baselines)

	if len(recs) != 1 {
		t.Fatalf("expected 1 recommendation, got %d", len(recs))
	}
	r := recs[0]
	expectedSavings := 10.0 - 2.0
	if r.SavingsEst != expectedSavings {
		t.Errorf("SavingsEst = %f, want %f", r.SavingsEst, expectedSavings)
	}
	if !strings.Contains(r.Action, "unnecessary loops") {
		t.Errorf("unexpected action: %s", r.Action)
	}
	if r.Signal.Reason != "cost_outlier" {
		t.Errorf("signal reason = %s, want cost_outlier", r.Signal.Reason)
	}
}

func TestGenerateRecommendationsLowSignal(t *testing.T) {
	signals := []WasteSignal{
		{SessionID: "s1", Project: "p", Severity: "medium", Reason: "low_signal", Detail: "low signal detail", Metric: 0.05, Threshold: 0.2},
	}
	baselines := map[string]Baseline{}
	recs := GenerateRecommendations(signals, baselines)

	if len(recs) != 1 {
		t.Fatalf("expected 1 recommendation, got %d", len(recs))
	}
	if !strings.Contains(recs[0].Action, "full agent interaction") {
		t.Errorf("unexpected action: %s", recs[0].Action)
	}
}

func TestGenerateRecommendationsSubagentOverhead(t *testing.T) {
	signals := []WasteSignal{
		{SessionID: "s1", Project: "p", Severity: "medium", Reason: "subagent_overhead", Detail: "overhead detail", Metric: 87.5, Threshold: 50},
	}
	baselines := map[string]Baseline{}
	recs := GenerateRecommendations(signals, baselines)

	if len(recs) != 1 {
		t.Fatalf("expected 1 recommendation, got %d", len(recs))
	}
	if !strings.Contains(recs[0].Action, "delegation") {
		t.Errorf("unexpected action: %s", recs[0].Action)
	}
}

func TestGenerateRecommendationsCacheUnderutilized(t *testing.T) {
	signals := []WasteSignal{
		{SessionID: "s1", Project: "p", Severity: "low", Reason: "cache_underutilized", Detail: "cache detail", Metric: 0.05, Threshold: 0.1},
	}
	baselines := map[string]Baseline{}
	recs := GenerateRecommendations(signals, baselines)

	if len(recs) != 1 {
		t.Fatalf("expected 1 recommendation, got %d", len(recs))
	}
	if !strings.Contains(recs[0].Action, "CLAUDE.md") {
		t.Errorf("unexpected action: %s", recs[0].Action)
	}
}

func TestGenerateRecommendationsSessionChurn(t *testing.T) {
	signals := []WasteSignal{
		{SessionID: "s1", Project: "p", Severity: "medium", Reason: "session_churn", Detail: "churn detail", Metric: 4, Threshold: 2},
	}
	baselines := map[string]Baseline{}
	recs := GenerateRecommendations(signals, baselines)

	if len(recs) != 1 {
		t.Fatalf("expected 1 recommendation, got %d", len(recs))
	}
	if !strings.Contains(recs[0].Action, "Consolidate") {
		t.Errorf("unexpected action: %s", recs[0].Action)
	}
}

func TestGenerateRecommendationsMultipleSignals(t *testing.T) {
	signals := []WasteSignal{
		{SessionID: "s1", Project: "p", Severity: "high", Reason: "cost_outlier", Metric: 10.0, Threshold: 5.0},
		{SessionID: "s2", Project: "p", Severity: "low", Reason: "cache_underutilized", Metric: 0.05, Threshold: 0.1},
	}
	baselines := map[string]Baseline{
		"p:h": {CostMean: 2.0},
	}
	recs := GenerateRecommendations(signals, baselines)

	if len(recs) != 2 {
		t.Fatalf("expected 2 recommendations, got %d", len(recs))
	}
}

func TestGenerateRecommendationsEmptyInput(t *testing.T) {
	recs := GenerateRecommendations(nil, nil)
	if len(recs) != 0 {
		t.Errorf("expected 0 recommendations for nil, got %d", len(recs))
	}

	recs = GenerateRecommendations([]WasteSignal{}, nil)
	if len(recs) != 0 {
		t.Errorf("expected 0 recommendations for empty, got %d", len(recs))
	}
}

func TestGenerateRecommendationsUnknownReason(t *testing.T) {
	signals := []WasteSignal{
		{SessionID: "s1", Project: "p", Severity: "high", Reason: "unknown_type", Metric: 10.0},
	}
	baselines := map[string]Baseline{}
	recs := GenerateRecommendations(signals, baselines)

	if len(recs) != 1 {
		t.Fatalf("expected 1 recommendation for unknown reason, got %d", len(recs))
	}
	if recs[0].Action == "" {
		t.Error("expected non-empty action for unknown reason")
	}
}
