package source

import (
	"math"
	"testing"
)

func TestCostForModel(t *testing.T) {
	const delta = 0.0001

	tests := []struct {
		name       string
		model      string
		input      int64
		output     int64
		cacheRead  int64
		cacheWrite int64
		want       float64
	}{
		{
			name:  "claude sonnet 4-5 exact",
			model: "claude-sonnet-4-5-20250929",
			input: 1000, output: 100, cacheRead: 500, cacheWrite: 200,
			want: (1000/1000.0)*3.00 + (100/1000.0)*15.00 + (500/1000.0)*0.30 + (200/1000.0)*3.75,
		},
		{
			name:  "claude sonnet 4-5 substring match",
			model: "some/prefix-claude-sonnet-4-5-suffix",
			input: 2000, output: 0, cacheRead: 0, cacheWrite: 0,
			want: (2000 / 1000.0) * 3.00,
		},
		{
			name:  "claude opus 4-5",
			model: "claude-opus-4-5-20250305",
			input: 1000, output: 500, cacheRead: 100, cacheWrite: 50,
			want: (1000/1000.0)*15.00 + (500/1000.0)*75.00 + (100/1000.0)*1.50 + (50/1000.0)*18.75,
		},
		{
			name:  "claude haiku 4-5",
			model: "claude-haiku-4-5-20251001",
			input: 500, output: 200, cacheRead: 0, cacheWrite: 0,
			want: (500/1000.0)*0.80 + (200/1000.0)*4.00,
		},
		{
			name:  "gemini 3 pro",
			model: "google/gemini-3-pro-preview",
			input: 1000, output: 500, cacheRead: 0, cacheWrite: 0,
			want: (1000/1000.0)*1.25 + (500/1000.0)*5.00,
		},
		{
			name:  "gemini 2.5 pro",
			model: "google/gemini-2.5-pro",
			input: 2000, output: 1000, cacheRead: 0, cacheWrite: 0,
			want: (2000/1000.0)*1.25 + (1000/1000.0)*5.00,
		},
		{
			name:  "gemini 2.5 flash",
			model: "google/gemini-2.5-flash",
			input: 10000, output: 1000, cacheRead: 0, cacheWrite: 0,
			want: (10000/1000.0)*0.15 + (1000/1000.0)*0.60,
		},
		{
			name:  "unknown model falls back to sonnet-tier",
			model: "some-unknown-model-v42",
			input: 1000, output: 100, cacheRead: 0, cacheWrite: 0,
			want: (1000/1000.0)*3.00 + (100/1000.0)*15.00,
		},
		{
			name:  "zero tokens gives zero cost",
			model: "claude-sonnet-4-5-20250929",
			input: 0, output: 0, cacheRead: 0, cacheWrite: 0,
			want: 0,
		},
		{
			name:  "very large token counts no overflow",
			model: "claude-sonnet-4-5-20250929",
			input: 1_000_000_000, output: 1_000_000_000, cacheRead: 500_000_000, cacheWrite: 200_000_000,
			want: (1_000_000_000/1000.0)*3.00 + (1_000_000_000/1000.0)*15.00 + (500_000_000/1000.0)*0.30 + (200_000_000/1000.0)*3.75,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CostForModel(tt.model, tt.input, tt.output, tt.cacheRead, tt.cacheWrite)
			if math.Abs(got-tt.want) > delta {
				t.Errorf("CostForModel(%q, %d, %d, %d, %d) = %f, want %f",
					tt.model, tt.input, tt.output, tt.cacheRead, tt.cacheWrite, got, tt.want)
			}
		})
	}
}

func TestCostForModelNonNegative(t *testing.T) {
	got := CostForModel("any-model", -5, -10, -3, -1)
	if got < 0 {
		t.Errorf("CostForModel with negative tokens should not return negative, got %f", got)
	}
}
