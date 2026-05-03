package source

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type CacheEntry struct {
	FetchedAt time.Time      `json:"fetched_at"`
	TTL       time.Duration  `json:"ttl_ns"`
	Entries   []PricingEntry `json:"entries"`
}

type PricingEntry struct {
	Key       string  `json:"key"`
	Input     float64 `json:"input"`
	Output    float64 `json:"output"`
	CacheRead float64 `json:"cache_read"`
}

type openRouterModel struct {
	ID      string             `json:"id"`
	Pricing openRouterPricing  `json:"pricing"`
}

type openRouterPricing struct {
	Prompt      string `json:"prompt"`
	Completion  string `json:"completion"`
	CacheRead   string `json:"cache_read"`
}

type openRouterResponse struct {
	Data []openRouterModel `json:"data"`
}

var pricingCacheTTL = 7 * 24 * time.Hour
var pricingAPIURL = "https://openrouter.ai/api/v1/models"

func FetchPricing(client *http.Client) ([]PricingEntry, error) {
	return fetchPricingFromURL(client, pricingAPIURL)
}

func fetchPricingFromURL(client *http.Client, url string) ([]PricingEntry, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	var orResp openRouterResponse
	if err := json.Unmarshal(body, &orResp); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	if len(orResp.Data) == 0 {
		return nil, fmt.Errorf("empty API response")
	}

	entries := make([]PricingEntry, 0, len(orResp.Data))
	for _, m := range orResp.Data {
		key := normalizeModelID(m.ID)
		if key == "" {
			continue
		}
		input := parsePricingFloat(m.Pricing.Prompt) * float64(tokensPerCostUnit)
		output := parsePricingFloat(m.Pricing.Completion) * float64(tokensPerCostUnit)
		cacheRead := parsePricingFloat(m.Pricing.CacheRead) * float64(tokensPerCostUnit)
		entries = append(entries, PricingEntry{
			Key:       key,
			Input:     input,
			Output:    output,
			CacheRead: cacheRead,
		})
	}

	return entries, nil
}

func parsePricingFloat(s string) float64 {
	var f float64
	_, _ = fmt.Sscanf(s, "%f", &f)
	return f
}

func normalizeModelID(openRouterID string) string {
	id := strings.ToLower(openRouterID)
	if idx := strings.LastIndex(id, "/"); idx != -1 {
		id = id[idx+1:]
	}
	id = strings.ReplaceAll(id, ".", "-")
	return id
}

func CachePath() string {
	dir, err := os.UserCacheDir()
	if err != nil {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, ".cache", "burnwatch", "pricing_v2.json")
	}
	return filepath.Join(dir, "burnwatch", "pricing_v2.json")
}

func LoadCache(path string) (*CacheEntry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cache CacheEntry
	if err := json.Unmarshal(data, &cache); err != nil {
		return nil, err
	}
	if len(cache.Entries) < 50 {
		return nil, fmt.Errorf("cache has too few entries (%d), treating as stale", len(cache.Entries))
	}
	ttl := pricingCacheTTL
	if cache.TTL > 0 {
		ttl = cache.TTL
	}
	if !isFresh(&cache, ttl) {
		return nil, fmt.Errorf("cache expired")
	}
	return &cache, nil
}

func SaveCache(path string, cache *CacheEntry) error {
	cache.TTL = pricingCacheTTL
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func isFresh(cache *CacheEntry, ttl time.Duration) bool {
	return time.Since(cache.FetchedAt) < ttl
}
