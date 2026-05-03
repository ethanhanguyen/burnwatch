package output

import (
	"fmt"
	"math/rand"
	"strings"
	"testing"
	"time"

	"github.com/ethanhanguyen/burnwatch/analyze"
	"github.com/ethanhanguyen/burnwatch/config"
	"github.com/ethanhanguyen/burnwatch/source"
)

func generateSyntheticEvents(n int) []source.TokenEvent {
	rng := rand.New(rand.NewSource(42))
	models := []string{
		"claude-sonnet-4-5-20250929",
		"claude-haiku-4-5-20251001",
		"deepseek/deepseek-v4-pro",
		"moonshotai/kimi-k2.6",
	}
	names := []string{"project-alpha", "project-beta"}
	events := make([]source.TokenEvent, 0, n)
	baseTime := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)

	for i := 0; i < n; i++ {
		sid := fmt.Sprintf("ses_bench_%06d", i)
		in := int64(500 + rng.Intn(10000))
		out := int64(100 + rng.Intn(5000))
		cr := int64(0)
		cw := int64(0)
		if rng.Float64() < 0.3 {
			cw = int64(rng.Intn(2000))
			cr = int64(rng.Intn(2000))
		}
		model := models[rng.Intn(len(models))]
		proj := names[rng.Intn(len(names))]
		ts := baseTime.Add(time.Duration(i) * time.Minute)
		cost, approx := source.CostForModel(model, in, out, cr, cw)
		events = append(events, source.TokenEvent{
			SessionID:       sid,
			Model:           model,
			Provider:        "test",
			Timestamp:       ts,
			InputTokens:     in,
			OutputTokens:    out,
			CacheRead:       cr,
			CacheWrite:      cw,
			CostUSD:         cost,
			CostApproximate: approx,
			Project:         proj,
			Harness:         "claude-code",
		})
	}
	return events
}

func BenchmarkPipeline_1K(b *testing.B) {
	events := generateSyntheticEvents(1000)
	cfg := config.Defaults()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		baselines := analyze.ComputeBaselines(events, cfg)
		_ = analyze.DetectWaste(events, baselines, cfg, allToggles)
	}
}

func BenchmarkPipeline_10K(b *testing.B) {
	events := generateSyntheticEvents(10000)
	cfg := config.Defaults()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		baselines := analyze.ComputeBaselines(events, cfg)
		_ = analyze.DetectWaste(events, baselines, cfg, allToggles)
	}
}

func BenchmarkBaselineComputation(b *testing.B) {
	events := generateSyntheticEvents(5000)
	cfg := config.Defaults()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = analyze.ComputeBaselines(events, cfg)
		_ = cfg
	}
}

func BenchmarkWasteDetection(b *testing.B) {
	events := generateSyntheticEvents(5000)
	baselines := analyze.ComputeBaselines(events, config.Defaults())
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = analyze.DetectWaste(events, baselines, config.Defaults(), allToggles)
	}
}

func BenchmarkPricingLookup(b *testing.B) {
	models := []string{
		"claude-sonnet-4-5-20250929",
		"deepseek/deepseek-v4-pro",
		"moonshotai/kimi-k2.6",
		"openai/gpt-5.4",
		"qwen/qwen3.6-plus",
		"unknown-model-v99",
	}
	tokens := []struct{ in, out, cr, cw int64 }{
		{1000, 200, 100, 50},
		{50000, 10000, 0, 0},
		{100, 50, 0, 0},
		{1000000, 200000, 50000, 0},
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mi := i % len(models)
		ti := i % len(tokens)
		tok := tokens[ti]
		_, _ = source.CostForModel(models[mi], tok.in, tok.out, tok.cr, tok.cw)
	}
}

func BenchmarkBuildSubagentTree(b *testing.B) {
	rng := rand.New(rand.NewSource(42))
	baseTime := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	events := make([]source.TokenEvent, 0, 500)
	for i := 0; i < 100; i++ {
		parentID := fmt.Sprintf("ses_parent_%04d", i)
		events = append(events, source.TokenEvent{
			SessionID:  parentID,
			Model:      "claude-sonnet-4-5-20250929",
			Provider:   "test",
			Timestamp:  baseTime,
			InputTokens: 1000,
			OutputTokens: 200,
			CostUSD:    3.0 + 3.0,
			Project:    "bench",
			Harness:    "claude-code",
		})
		numSubs := 1 + rng.Intn(4)
		for j := 0; j < numSubs; j++ {
			subID := fmt.Sprintf("ses_sub_%04d_%02d", i, j)
			cost := 0.5 + rng.Float64()*2.0
			events = append(events, source.TokenEvent{
				SessionID:       subID,
				ParentSessionID: parentID,
				Model:           "claude-haiku-4-5-20251001",
				Provider:        "test",
				Timestamp:       baseTime,
				InputTokens:     500,
				OutputTokens:    100,
				CostUSD:         cost,
				Project:         "bench",
				Harness:         "claude-code",
				IsSubagent:      true,
			})
		}
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = analyze.BuildSubagentTree(events)
	}
}

func BenchmarkSignalQuality(b *testing.B) {
	events := loadScenarioJSONL(b, "cost_outlier.jsonl")
	events = append(events, loadScenarioJSONL(b, "all_clean.jsonl")...)
	events = append(events, loadScenarioJSONL(b, "cache_underutilized.jsonl")...)
	events = append(events, loadScenarioJSONL(b, "multi_signal.jsonl")...)

	baselines := analyze.ComputeBaselines(events, config.Defaults())
	b.ResetTimer()

	var tp, fp, fn int
	for i := 0; i < b.N; i++ {
		tp, fp, fn = 0, 0, 0
		signals := analyze.DetectWaste(events, baselines, config.Defaults(), allToggles)
		for _, s := range signals {
			if strings.Contains(s.SessionID, "_waste") {
				tp++
			}
			if strings.Contains(s.SessionID, "_normal") || strings.Contains(s.SessionID, "_clean") {
				fp++
			}
		}
		for _, e := range events {
			if strings.Contains(e.SessionID, "_waste") {
				found := false
				for _, s := range signals {
					if s.SessionID == e.SessionID {
						found = true
						break
					}
				}
				if !found {
					fn++
				}
			}
		}
	}
	precision := float64(tp) / float64(tp+fp)
	recall := float64(tp) / float64(tp+fn)
	f1 := 2 * precision * recall / (precision + recall)
	b.ReportMetric(precision, "precision")
	b.ReportMetric(recall, "recall")
	b.ReportMetric(f1, "f1")
}
