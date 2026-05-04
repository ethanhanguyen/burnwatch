package source

import (
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

const tokensPerCostUnit = 1_000_000 // tokens per $ pricing unit (per 1M)

type priceEntry struct {
	input      float64 // $ per 1M input tokens
	output     float64 // $ per 1M output tokens
	cacheRead  float64 // $ per 1M cache-read tokens
	cacheWrite float64 // $ per 1M cache-write tokens
}

var pricing = []struct {
	key string
	p   priceEntry
}{
	{"claude-opus-4-7", priceEntry{15.00, 75.00, 1.50, 18.75}},
	{"claude-opus-4-6", priceEntry{15.00, 75.00, 1.50, 18.75}},
	{"claude-opus-4-5", priceEntry{15.00, 75.00, 1.50, 18.75}},
	{"claude-sonnet-4-6", priceEntry{3.00, 15.00, 0.30, 3.75}},
	{"claude-sonnet-4-5", priceEntry{3.00, 15.00, 0.30, 3.75}},
	{"claude-haiku-4-5", priceEntry{0.80, 4.00, 0.08, 1.00}},
	{"claude-3-5-haiku", priceEntry{0.80, 4.00, 0.08, 1.00}},
	{"gemini-3.1-pro", priceEntry{1.25, 5.00, 0, 0}},
	{"gemini-3-pro", priceEntry{1.25, 5.00, 0, 0}},
	{"gemini-2.5-pro", priceEntry{1.25, 5.00, 0, 0}},
	{"gemini-2.5-flash", priceEntry{0.15, 0.60, 0, 0}},
	{"gpt-5.4-pro", priceEntry{15.00, 75.00, 0, 0}},
	{"gpt-5.4-nano", priceEntry{0.15, 0.60, 0, 0}},
	{"gpt-5.4", priceEntry{2.50, 10.00, 0, 0}},
	{"gpt-5-nano", priceEntry{0.15, 0.60, 0, 0}},
	{"deepseek-v4-pro", priceEntry{2.50, 8.00, 0, 0}},
	{"deepseek-v4-flash", priceEntry{0.27, 1.10, 0, 0}},
	{"grok-4.20-multi-agent", priceEntry{3.00, 15.00, 0, 0}},
	{"grok-4.20-reasoning", priceEntry{8.00, 20.00, 0, 0}},
	{"grok-4.20", priceEntry{2.00, 8.00, 0, 0}},
	{"kimi-k2.6", priceEntry{0.40, 2.00, 0, 0}},
	{"kimi-k2.5", priceEntry{0.40, 2.00, 0, 0}},
	{"gemma-4-31b", priceEntry{0.15, 0.60, 0, 0}},
	{"gemma-4-26b", priceEntry{0.10, 0.40, 0, 0}},
	{"qwen3.6-plus", priceEntry{0.80, 3.20, 0, 0}},
	{"qwen3.6", priceEntry{0.40, 0.80, 0, 0}},
	{"minimax-m2.7", priceEntry{0.30, 1.20, 0, 0}},
	{"claude-opus", priceEntry{15.00, 75.00, 1.50, 18.75}},
	{"claude-sonnet", priceEntry{3.00, 15.00, 0.30, 3.75}},
	{"claude-haiku", priceEntry{0.80, 4.00, 0.08, 1.00}},
	{"gemini-pro", priceEntry{1.25, 5.00, 0, 0}},
	{"gemini-flash", priceEntry{0.15, 0.60, 0, 0}},
	{"deepseek", priceEntry{1.25, 5.00, 0, 0}},
	{"gpt", priceEntry{2.50, 10.00, 0, 0}},
	{"grok", priceEntry{2.00, 8.00, 0, 0}},
	{"kimi", priceEntry{0.40, 2.00, 0, 0}},
	{"gemma", priceEntry{0.15, 0.60, 0, 0}},
	{"qwen", priceEntry{0.40, 0.80, 0, 0}},
	{"minimax", priceEntry{0.30, 1.20, 0, 0}},
	{"gemini", priceEntry{1.25, 5.00, 0, 0}},
}

var fetchedPricing []PricingEntry
var pricingInitialized bool

func InitPricing(client *http.Client) error {
	if pricingInitialized {
		return nil
	}
	cachePath := CachePath()
	cache, err := LoadCache(cachePath)
	if err == nil {
		fetchedPricing = cache.Entries
		pricingInitialized = true
		return nil
	}
	return fetchAndCache(client)
}

func RefreshPricing(client *http.Client) error {
	return fetchAndCache(client)
}

func fetchAndCache(client *http.Client) error {
	entries, err := FetchPricing(client)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Pricing fetch failed: %v (using embedded pricing)\n", err)
		pricingInitialized = true
		return err
	}
	cache := &CacheEntry{
		FetchedAt: time.Now(),
		Entries:   entries,
	}
	if saveErr := SaveCache(CachePath(), cache); saveErr != nil {
		fmt.Fprintf(os.Stderr, "Cache save failed: %v\n", saveErr)
	}
	fetchedPricing = entries
	pricingInitialized = true
	return nil
}

func CostForModel(model string, inputTokens, outputTokens, cacheRead, cacheWrite int64) (float64, bool, bool) {
	if inputTokens < 0 {
		inputTokens = 0
	}
	if outputTokens < 0 {
		outputTokens = 0
	}
	if cacheRead < 0 {
		cacheRead = 0
	}
	if cacheWrite < 0 {
		cacheWrite = 0
	}

	modelLower := strings.ToLower(model)

	p, approximate, costUnknown := lookupPrice(modelLower)
	if costUnknown {
		return 0, false, true
	}

	tcu := float64(tokensPerCostUnit)
	cost := float64(inputTokens)/tcu*p.input +
		float64(outputTokens)/tcu*p.output +
		float64(cacheRead)/tcu*p.cacheRead +
		float64(cacheWrite)/tcu*p.cacheWrite

	return cost, approximate, false
}

func lookupPrice(modelLower string) (priceEntry, bool, bool) {
	normalized := normalizeModelForLookup(modelLower)

	bestMatch := ""
	bestLen := 0
	for _, e := range fetchedPricing {
		if strings.EqualFold(modelLower, e.Key) {
			return priceEntry{e.Input, e.Output, e.CacheRead, 0}, false, false
		}
		if strings.Contains(modelLower, e.Key) {
			if len(e.Key) > bestLen {
				bestMatch = e.Key
				bestLen = len(e.Key)
			}
		}
	}
	if bestMatch != "" {
		for _, e := range fetchedPricing {
			if e.Key == bestMatch {
				return priceEntry{e.Input, e.Output, e.CacheRead, 0}, false, false
			}
		}
	}

	for _, entry := range pricing {
		if strings.Contains(normalized, entry.key) {
			return entry.p, false, false
		}
	}

	return priceEntry{}, false, true
}

func normalizeModelForLookup(model string) string {
	s := strings.ToLower(model)
	if idx := strings.LastIndex(s, "/"); idx >= 0 {
		s = s[idx+1:]
	}
	if len(s) > 9 && s[len(s)-9] == '-' {
		if _, err := strconv.Atoi(s[len(s)-8:]); err == nil {
			s = s[:len(s)-9]
		}
	}
	return s
}
