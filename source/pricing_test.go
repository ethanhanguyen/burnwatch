package source

import (
	"math"
	"net/http"
	"testing"
	"time"
)

func TestCostForModel(t *testing.T) {
	const delta = 0.0001

	tests := []struct {
		name        string
		model       string
		input       int64
		output      int64
		cacheRead   int64
		cacheWrite  int64
		want        float64
		wantApprox  bool
		wantUnknown bool
	}{
		{
			name:  "claude sonnet 4-5 exact",
			model: "claude-sonnet-4-5-20250929",
			input: 1000, output: 100, cacheRead: 500, cacheWrite: 200,
			want:       (1000/1000.0)*0.003 + (100/1000.0)*0.015 + (500/1000.0)*0.0003 + (200/1000.0)*0.00375,
			wantApprox: false,
		},
		{
			name:  "claude sonnet 4-5 substring match",
			model: "some/prefix-claude-sonnet-4-5-suffix",
			input: 2000, output: 0, cacheRead: 0, cacheWrite: 0,
			want:       (2000 / 1000.0) * 0.003,
			wantApprox: false,
		},
		{
			name:  "claude opus 4-5",
			model: "claude-opus-4-5-20250305",
			input: 1000, output: 500, cacheRead: 100, cacheWrite: 50,
			want:       (1000/1000.0)*0.015 + (500/1000.0)*0.075 + (100/1000.0)*0.0015 + (50/1000.0)*0.01875,
			wantApprox: false,
		},
		{
			name:  "claude haiku 4-5",
			model: "claude-haiku-4-5-20251001",
			input: 500, output: 200, cacheRead: 0, cacheWrite: 0,
			want:       (500/1000.0)*0.0008 + (200/1000.0)*0.004,
			wantApprox: false,
		},
		{
			name:  "gemini 3 pro",
			model: "google/gemini-3-pro-preview",
			input: 1000, output: 500, cacheRead: 0, cacheWrite: 0,
			want:       (1000/1000.0)*0.00125 + (500/1000.0)*0.005,
			wantApprox: false,
		},
		{
			name:  "gemini 2.5 pro",
			model: "google/gemini-2.5-pro",
			input: 2000, output: 1000, cacheRead: 0, cacheWrite: 0,
			want:       (2000/1000.0)*0.00125 + (1000/1000.0)*0.005,
			wantApprox: false,
		},
		{
			name:  "gemini 2.5 flash",
			model: "google/gemini-2.5-flash",
			input: 10000, output: 1000, cacheRead: 0, cacheWrite: 0,
			want:       (10000/1000.0)*0.00015 + (1000/1000.0)*0.0006,
			wantApprox: false,
		},
		{
			name:  "unknown model has no pricing",
			model: "some-unknown-model-v42",
			input: 1000, output: 100, cacheRead: 0, cacheWrite: 0,
			want:       0,
			wantApprox: false,
			wantUnknown: true,
		},
		{
			name:  "zero tokens gives zero cost",
			model: "claude-sonnet-4-5-20250929",
			input: 0, output: 0, cacheRead: 0, cacheWrite: 0,
			want:       0,
			wantApprox: false,
		},
		{
			name:  "very large token counts no overflow",
			model: "claude-sonnet-4-5-20250929",
			input: 1_000_000_000, output: 1_000_000_000, cacheRead: 500_000_000, cacheWrite: 200_000_000,
			want:       (1_000_000_000/1000.0)*0.003 + (1_000_000_000/1000.0)*0.015 + (500_000_000/1000.0)*0.0003 + (200_000_000/1000.0)*0.00375,
			wantApprox: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, approx, unknown := CostForModel(tt.model, tt.input, tt.output, tt.cacheRead, tt.cacheWrite)
			if math.Abs(got-tt.want) > delta {
				t.Errorf("CostForModel(%q, %d, %d, %d, %d) = %f, want %f",
					tt.model, tt.input, tt.output, tt.cacheRead, tt.cacheWrite, got, tt.want)
			}
			if approx != tt.wantApprox {
				t.Errorf("CostForModel(%q) approximate = %v, want %v", tt.model, approx, tt.wantApprox)
			}
			if unknown != tt.wantUnknown {
				t.Errorf("CostForModel(%q) costUnknown = %v, want %v", tt.model, unknown, tt.wantUnknown)
			}
		})
	}
}

func TestCostForModelNonNegative(t *testing.T) {
	got, _, _ := CostForModel("any-model", -5, -10, -3, -1)
	if got < 0 {
		t.Errorf("CostForModel with negative tokens should not return negative, got %f", got)
	}
}

func TestCostForModel_FetchedPricingMatch(t *testing.T) {
	fetchedPricing = []PricingEntry{
		{Key: "deepseek-v4-pro", Input: 0.435, Output: 0.87, CacheRead: 0},
	}
	pricingInitialized = true
	defer func() { fetchedPricing = nil; pricingInitialized = false }()

	got, approx, _ := CostForModel("vercel/deepseek/deepseek-v4-pro", 1000, 1000, 0, 0)
	want := (1000/1000.0)*0.435 + (1000/1000.0)*0.87
	if math.Abs(got-want) > 0.0001 {
		t.Errorf("CostForModel(deepseek) = %f, want %f", got, want)
	}
	if approx {
		t.Error("expected non-approximate for fetched pricing match")
	}
}

func TestCostForModel_SubstringMatchLongestKey(t *testing.T) {
	fetchedPricing = []PricingEntry{
		{Key: "claude-sonnet-4", Input: 1.0, Output: 5.0, CacheRead: 0},
		{Key: "claude-sonnet-4-5", Input: 3.0, Output: 15.0, CacheRead: 0},
	}
	pricingInitialized = true
	defer func() { fetchedPricing = nil; pricingInitialized = false }()

	got, approx, _ := CostForModel("claude-sonnet-4-5-20250929", 1000, 0, 0, 0)
	if math.Abs(got-3.0) > 0.0001 {
		t.Errorf("expected longest key match (3.0), got %f", got)
	}
	if approx {
		t.Error("expected non-approximate for longest key match")
	}
}

func TestCostForModel_EmptyFetchedPricing(t *testing.T) {
	fetchedPricing = nil
	pricingInitialized = true

	_, _, unknown := CostForModel("deepseek-v4-pro", 1000, 0, 0, 0)
	if !unknown {
		t.Error("expected costUnknown for model not in embedded pricing")
	}
}

func TestCostForModel_ExactMatchPrecedence(t *testing.T) {
	fetchedPricing = []PricingEntry{
		{Key: "claude-sonnet-4-5", Input: 99.0, Output: 99.0, CacheRead: 0},
	}
	pricingInitialized = true
	defer func() { fetchedPricing = nil; pricingInitialized = false }()

	got, approx, _ := CostForModel("claude-sonnet-4-5", 1000, 0, 0, 0)
	if math.Abs(got-99.0) > 0.0001 {
		t.Errorf("expected 99.0 from exact match, got %f", got)
	}
	if approx {
		t.Error("exact match should not be approximate")
	}
}

func TestLookupPrice_NoFetchedNoEmbedded(t *testing.T) {
	fetchedPricing = nil
	pricingInitialized = true
	defer func() { fetchedPricing = nil; pricingInitialized = false }()

	_, approx, unknown := lookupPrice("unknown-model-xyz")
	if !unknown {
		t.Error("expected costUnknown for unknown model")
	}
	if approx {
		t.Error("unexpected approximate for unknown model")
	}
}

func TestInitPricing_NoNetwork(t *testing.T) {
	pricingInitialized = false
	fetchedPricing = nil
	defer func() { pricingInitialized = false; fetchedPricing = nil }()

	client := &http.Client{Timeout: 1 * time.Second}
	_ = InitPricing(client)
	if !pricingInitialized {
		t.Error("InitPricing should set pricingInitialized even on error")
	}
}

func TestInitPricing_Idempotent(t *testing.T) {
	pricingInitialized = true
	fetchedPricing = []PricingEntry{{Key: "test", Input: 1.0}}
	defer func() { pricingInitialized = false; fetchedPricing = nil }()

	client := &http.Client{}
	err := InitPricing(client)
	if err != nil {
		t.Errorf("expected no error for idempotent InitPricing, got %v", err)
	}
	if len(fetchedPricing) != 1 {
		t.Error("fetchedPricing should not change on re-init")
	}
}

func TestRefreshPricing_NoNetwork(t *testing.T) {
	pricingInitialized = false
	fetchedPricing = nil
	defer func() {
		pricingInitialized = false
		fetchedPricing = nil
	}()

	client := &http.Client{Timeout: 1 * time.Millisecond}
	err := RefreshPricing(client)
	if err == nil {
		t.Log("RefreshPricing succeeded unexpectedly (network available?)")
	}
	if !pricingInitialized {
		t.Error("RefreshPricing should set pricingInitialized even on error")
	}
}

func TestCostForModel_DeepseekMatched(t *testing.T) {
	orig := fetchedPricing
	origInit := pricingInitialized
	fetchedPricing = []PricingEntry{
		{Key: "deepseek-v4-pro", Input: 0.435, Output: 0.87, CacheRead: 0},
	}
	pricingInitialized = true
	defer func() {
		fetchedPricing = orig
		pricingInitialized = origInit
	}()

	got, approx, _ := CostForModel("deepseek/deepseek-v4-pro", 1000, 500, 0, 0)
	wantInput := 0.435
	wantOutput := 0.87 * 0.5
	want := wantInput + wantOutput
	if math.Abs(got-want) > 0.001 {
		t.Errorf("deepseek cost = %f, want ~%f", got, want)
	}
	if approx {
		t.Error("deepseek match should not be approximate")
	}
}
