package source

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

type priceEntry struct {
	input      float64
	output     float64
	cacheRead  float64
	cacheWrite float64
}

var pricing = []struct {
	key string
	p   priceEntry
}{
	{"claude-sonnet-4-5", priceEntry{3.00, 15.00, 0.30, 3.75}},
	{"claude-opus-4-5", priceEntry{15.00, 75.00, 1.50, 18.75}},
	{"claude-haiku-4-5", priceEntry{0.80, 4.00, 0.08, 1.00}},
	{"gemini-3-pro", priceEntry{1.25, 5.00, 0, 0}},
	{"gemini-2.5-pro", priceEntry{1.25, 5.00, 0, 0}},
	{"gemini-2.5-flash", priceEntry{0.15, 0.60, 0, 0}},
}

var fallback = priceEntry{3.00, 15.00, 0.30, 3.75}

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

func CostForModel(model string, inputTokens, outputTokens, cacheRead, cacheWrite int64) (float64, bool) {
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

	p, approximate := lookupPrice(modelLower)

	cost := float64(inputTokens)/1000.0*p.input +
		float64(outputTokens)/1000.0*p.output +
		float64(cacheRead)/1000.0*p.cacheRead +
		float64(cacheWrite)/1000.0*p.cacheWrite

	return cost, approximate
}

func lookupPrice(modelLower string) (priceEntry, bool) {
	bestMatch := ""
	bestLen := 0
	for _, e := range fetchedPricing {
		if strings.EqualFold(modelLower, e.Key) {
			return priceEntry{e.Input, e.Output, e.CacheRead, 0}, false
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
				return priceEntry{e.Input, e.Output, e.CacheRead, 0}, false
			}
		}
	}

	for _, entry := range pricing {
		if strings.Contains(modelLower, entry.key) {
			return entry.p, false
		}
	}

	return fallback, true
}
