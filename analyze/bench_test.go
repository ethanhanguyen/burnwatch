package analyze

import (
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/ethanhanguyen/burnwatch/config"
	"github.com/ethanhanguyen/burnwatch/source"
)

func behavioralCfg() config.Config {
	return config.Config{
		Thresholds: config.Thresholds{
			CostOutlierSigma:            2.0,
			LowSignalPercentile:         10.0,
			CachePercentile:             10.0,
			SubagentOverheadPct:         50.0,
			ChurnMinSessions:            3,
			FragmentationIndexThreshold: 3.0,
			InputOverconsumptionSigma:   2.0,
			OutputExplosionSigma:        2.0,
			TokenEfficiencyPercentile:   10.0,
			FragmentationMinCost:        0.50,
			ToolLoopMaxRepeats:          5,
			FileRereadMinCount:          3,
			SubagentOverlapPct:          50.0,
			SessionRestartPct:           80.0,
			SessionRestartInitialOps:    10,
		},
		Signals: config.Signals{
			CostOutlier:          true,
			LowSignal:            true,
			SubagentOverhead:     true,
			CacheUnderutilized:   true,
			FragmentationIndex:   true,
			InputOverconsumption: true,
			OutputExplosion:      true,
			TokenEfficiency:      true,
			ToolLoop:             true,
			FileReread:           true,
			SubagentOverlap:      true,
			SessionRestart:       true,
		},
		Filters: config.Filters{MinCost: 0, Deduplicate: false},
		Output:  config.Output{},
	}
}

func generateBehavioralBenchEvents(rng *rand.Rand, sessions, eventsPerSession int) []source.TokenEvent {
	var events []source.TokenEvent
	baseTime := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	toolNames := []string{"read", "write", "edit", "bash", "glob", "grep"}
	filePaths := []string{
		"src/main.go",
		"src/handler.go",
		"pkg/util/helper.go",
		"config/settings.json",
		"docs/README.md",
	}

	eventIdx := 0
	for s := 0; s < sessions; s++ {
		sid := fmt.Sprintf("ses_bench_%06d", s)
		model := "claude-sonnet-4-5-20250929"
		if rng.Float64() < 0.3 {
			model = "claude-haiku-4-5-20251001"
		}
		proj := "bench-project"
		if rng.Float64() < 0.5 {
			proj = "bench-project-2"
		}

		for e := 0; e < eventsPerSession; e++ {
			eventIdx++
			in := int64(200 + rng.Intn(8000))
			out := int64(50 + rng.Intn(4000))
			ts := baseTime.Add(time.Duration(s*eventsPerSession+e) * time.Minute)
			cost, approx, _ := source.CostForModel(model, in, out, 0, 0)

			numToolCalls := rng.Intn(4)
			var toolCalls []source.ToolCall
			var fileOps []source.FileOp
			for t := 0; t < numToolCalls; t++ {
				toolName := toolNames[rng.Intn(len(toolNames))]
				filePath := filePaths[rng.Intn(len(filePaths))]
				toolCalls = append(toolCalls, source.ToolCall{
					Name:      toolName,
					Arguments: fmt.Sprintf(`{"file_path":"%s"}`, filePath),
				})
				fileOps = append(fileOps, source.FileOp{
					Path:      filePath,
					Operation: toolName,
				})
			}

			events = append(events, source.TokenEvent{
				SessionID:       sid,
				Model:           model,
				Provider:        "anthropic",
				Timestamp:       ts,
				InputTokens:     in,
				OutputTokens:    out,
				CostUSD:         cost,
				CostApproximate: approx,
				Project:         proj,
				Harness:         "claude-code",
				ToolCalls:       toolCalls,
				FileOps:         fileOps,
				EventIndex:      eventIdx,
			})
		}
	}

	return events
}

func BenchmarkBehavioralDetectWaste(b *testing.B) {
	rng := rand.New(rand.NewSource(42))
	events := generateBehavioralBenchEvents(rng, 500, 50)
	baselines := ComputeBaselines(events, config.Defaults())
	trees := BuildSubagentTree(events)
	cfg := behavioralCfg()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = DetectWaste(events, baselines, trees, cfg)
	}
}

func BenchmarkBehavioralDetectWaste_1K(b *testing.B) {
	rng := rand.New(rand.NewSource(42))
	events := generateBehavioralBenchEvents(rng, 1000, 30)
	baselines := ComputeBaselines(events, config.Defaults())
	trees := BuildSubagentTree(events)
	cfg := behavioralCfg()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = DetectWaste(events, baselines, trees, cfg)
	}
}

func BenchmarkBehavioralDetectWaste_10K(b *testing.B) {
	rng := rand.New(rand.NewSource(42))
	events := generateBehavioralBenchEvents(rng, 10000, 10)
	baselines := ComputeBaselines(events, config.Defaults())
	trees := BuildSubagentTree(events)
	cfg := behavioralCfg()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = DetectWaste(events, baselines, trees, cfg)
	}
}
