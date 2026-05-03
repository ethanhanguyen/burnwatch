package source

import (
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestFetchPricing_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	client := &http.Client{Timeout: 10 * time.Second}
	entries, err := FetchPricing(client)
	if err != nil {
		t.Fatalf("FetchPricing: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("expected non-empty entries from OpenRouter")
	}

	foundDeepSeek := false
	foundSonnet := false
	for _, e := range entries {
		if strings.Contains(e.Key, "deepseek") {
			foundDeepSeek = true
			if e.Input <= 0 {
				t.Errorf("deepseek input price = %f, want > 0", e.Input)
			}
			if e.Output <= 0 {
				t.Errorf("deepseek output price = %f, want > 0", e.Output)
			}
		}
		if strings.Contains(e.Key, "claude-sonnet") {
			foundSonnet = true
		}
	}
	if !foundDeepSeek {
		t.Fatal("expected deepseek model in OpenRouter pricing")
	}
	if !foundSonnet {
		t.Fatal("expected claude-sonnet model in OpenRouter pricing")
	}
}

func TestFetchPricing_Mock(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(openRouterResponse{
			Data: []openRouterModel{
				{ID: "deepseek/deepseek-v4-pro", Pricing: openRouterPricing{Prompt: "0.000000435", Completion: "0.00000087"}},
				{ID: "anthropic/claude-sonnet-4-5", Pricing: openRouterPricing{Prompt: "0.000003", Completion: "0.000015", CacheRead: "0.00000030"}},
			},
		})
	}))
	defer server.Close()

	pricingAPIURL := server.URL
	entries, err := fetchPricingFromURL(&http.Client{Timeout: 10 * time.Second}, pricingAPIURL)
	if err != nil {
		t.Fatalf("fetchPricingFromURL: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	ds := entries[0]
	if ds.Key != "deepseek-v4-pro" {
		t.Errorf("deepseek key = %q, want deepseek-v4-pro", ds.Key)
	}
	if math.Abs(ds.Input-0.000435) > 0.000001 {
		t.Errorf("deepseek input = %f, want 0.000435", ds.Input)
	}
	if math.Abs(ds.Output-0.00087) > 0.000001 {
		t.Errorf("deepseek output = %f, want 0.00087", ds.Output)
	}
	cs := entries[1]
	if math.Abs(cs.CacheRead-0.00030) > 0.000001 {
		t.Errorf("sonnet cache_read = %f, want 0.00030", cs.CacheRead)
	}
}

func TestFetchPricing_MockError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	defer server.Close()

	_, err := fetchPricingFromURL(&http.Client{Timeout: 10 * time.Second}, server.URL)
	if err == nil {
		t.Error("expected error for non-200 status")
	}
}

func TestFetchPricing_MockEmpty(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(openRouterResponse{Data: nil})
	}))
	defer server.Close()

	_, err := fetchPricingFromURL(&http.Client{Timeout: 10 * time.Second}, server.URL)
	if err == nil {
		t.Error("expected error for empty response")
	}
}

func TestFetchPricing_HTTPError(t *testing.T) {
	client := &http.Client{Timeout: 1 * time.Millisecond}
	_, err := FetchPricing(client)
	if err == nil {
		t.Error("expected error with 1ms timeout")
	}
}

func TestFetchAndCache(t *testing.T) {
	pricingAPIURL = "http://127.0.0.1:1" // invalid URL to force fetch error
	pricingInitialized = false
	fetchedPricing = nil
	defer func() {
		pricingAPIURL = "https://openrouter.ai/api/v1/models"
		pricingInitialized = false
		fetchedPricing = nil
	}()

	err := fetchAndCache(&http.Client{Timeout: 1 * time.Second})
	if err == nil {
		t.Log("fetchAndCache succeeded (unexpected network)")
	}
	if !pricingInitialized {
		t.Error("fetchAndCache should set pricingInitialized even on failure")
	}
}

func TestFetchAndCache_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(openRouterResponse{
			Data: []openRouterModel{
				{ID: "test/model-1", Pricing: openRouterPricing{Prompt: "0.000001", Completion: "0.000002"}},
			},
		})
	}))
	defer server.Close()

	prevURL := pricingAPIURL
	pricingAPIURL = server.URL
	prevInit := pricingInitialized
	pricingInitialized = false
	prevFetched := fetchedPricing
	fetchedPricing = nil
	defer func() {
		pricingAPIURL = prevURL
		pricingInitialized = prevInit
		fetchedPricing = prevFetched
	}()

	err := fetchAndCache(&http.Client{Timeout: 10 * time.Second})
	if err != nil {
		t.Fatalf("fetchAndCache: %v", err)
	}
	if !pricingInitialized {
		t.Error("fetchAndCache should set pricingInitialized on success")
	}
	if len(fetchedPricing) != 1 {
		t.Errorf("expected 1 fetched entry, got %d", len(fetchedPricing))
	}
}

func TestInitPricing_CacheHit(t *testing.T) {
	tmpDir := t.TempDir()
	entries := make([]PricingEntry, 50)
	for i := range entries {
		entries[i] = PricingEntry{Key: "model-" + string(rune('a'+i%26)), Input: float64(i+1), Output: float64(i+2), CacheRead: 0}
	}
	cache := &CacheEntry{
		FetchedAt: time.Now(),
		TTL:       24 * time.Hour,
		Entries:   entries,
	}
	cachePath := filepath.Join(tmpDir, "pricing.json")
	if err := SaveCache(cachePath, cache); err != nil {
		t.Fatal(err)
	}

	prevInit := pricingInitialized
	pricingInitialized = false
	prevFetched := fetchedPricing
	fetchedPricing = nil
	defer func() {
		pricingInitialized = prevInit
		fetchedPricing = prevFetched
	}()

	cacheData, _ := LoadCache(cachePath)
	if cacheData == nil {
		t.Fatal("failed to load cache")
	}
	fetchedPricing = cacheData.Entries
	pricingInitialized = true

	client := &http.Client{Timeout: 1 * time.Second}
	err := InitPricing(client)
	if err != nil {
		t.Errorf("InitPricing with cached data should not error, got %v", err)
	}
	if !pricingInitialized {
		t.Error("InitPricing should leave pricingInitialized true")
	}
	if len(fetchedPricing) != 50 {
		t.Error("fetchedPricing should not change on cache hit")
	}
}

func TestCacheRoundTrip(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "pricing.json")

	entries := make([]PricingEntry, 50)
	for i := range entries {
		entries[i] = PricingEntry{Key: "model-" + string(rune('a'+i%26)), Input: float64(i+1), Output: float64(i+2), CacheRead: 0}
	}
	entries[0] = PricingEntry{Key: "claude-sonnet-4-5", Input: 3.0, Output: 15.0, CacheRead: 0.30}
	entries[1] = PricingEntry{Key: "deepseek-v4-pro", Input: 0.435, Output: 0.87, CacheRead: 0}
	cache := &CacheEntry{
		FetchedAt: time.Now(),
		Entries:   entries,
	}

	if err := SaveCache(path, cache); err != nil {
		t.Fatalf("SaveCache: %v", err)
	}

	loaded, err := LoadCache(path)
	if err != nil {
		t.Fatalf("LoadCache: %v", err)
	}
	if len(loaded.Entries) != len(entries) {
		t.Errorf("entries count: %d, want %d", len(loaded.Entries), len(entries))
	}
}

func TestCacheFreshness(t *testing.T) {
	fresh := &CacheEntry{
		FetchedAt: time.Now().Add(-1 * time.Hour),
	}
	if !isFresh(fresh, 24*time.Hour) {
		t.Error("1h old cache with 24h TTL should be fresh")
	}

	stale := &CacheEntry{
		FetchedAt: time.Now().Add(-48 * time.Hour),
	}
	if isFresh(stale, 24*time.Hour) {
		t.Error("48h old cache with 24h TTL should be stale")
	}
}

func TestLoadCache_MissingFile(t *testing.T) {
	_, err := LoadCache("/tmp/nonexistent/burnwatch/pricing.json")
	if err == nil {
		t.Error("expected error for missing cache file")
	}
}

func TestLoadCache_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "bad.json")
	if err := os.WriteFile(path, []byte("not json"), 0644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadCache(path)
	if err == nil {
		t.Error("expected error for invalid JSON cache")
	}
}

func TestLoadCache_EmptyEntries(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "empty.json")
	cache := &CacheEntry{FetchedAt: time.Now(), Entries: nil}
	data, _ := json.Marshal(cache)
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadCache(path)
	if err == nil {
		t.Error("expected error for cache with empty entries")
	}
}

func TestNormalizeModelID(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"anthropic/claude-sonnet-4-5", "claude-sonnet-4-5"},
		{"vercel/deepseek/deepseek-v4-pro", "deepseek-v4-pro"},
		{"openai/gpt-5.4", "gpt-5-4"},
		{"google/gemini-3-pro-preview", "gemini-3-pro-preview"},
		{"moonshotai/kimi-k2.6", "kimi-k2-6"},
		{"naked-model", "naked-model"},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got := normalizeModelID(tt.in)
			if got != tt.want {
				t.Errorf("normalizeModelID(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestParsePricingFloat(t *testing.T) {
	tests := []struct {
		in   string
		want float64
	}{
		{"0.000003", 0.000003},
		{"0.000015", 0.000015},
		{"0.000000435", 0.000000435},
		{"", 0},
		{"not-a-number", 0},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got := parsePricingFloat(tt.in)
			if math.Abs(got-tt.want) > 1e-12 {
				t.Errorf("parsePricingFloat(%q) = %f, want %f", tt.in, got, tt.want)
			}
		})
	}
}

func TestCachePath(t *testing.T) {
	got := CachePath()
	if !strings.HasSuffix(got, "burnwatch/pricing.json") {
		t.Errorf("expected path to end with burnwatch/pricing.json, got %s", got)
	}
}

func TestLoadCache_TooFewEntries(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "few.json")
	entries := make([]PricingEntry, 49)
	for i := range entries {
		entries[i] = PricingEntry{Key: "model-" + string(rune('a'+i%26)), Input: float64(i+1)}
	}
	cache := &CacheEntry{FetchedAt: time.Now(), TTL: 24 * time.Hour, Entries: entries}
	data, _ := json.Marshal(cache)
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadCache(path)
	if err == nil {
		t.Error("expected error for cache with only 49 entries (<50)")
	}
}

func TestLoadCache_MinEntriesValid(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "min.json")
	entries := make([]PricingEntry, 50)
	for i := range entries {
		entries[i] = PricingEntry{Key: "model-" + string(rune('a'+i%26)), Input: float64(i+1)}
	}
	cache := &CacheEntry{FetchedAt: time.Now(), TTL: 24 * time.Hour, Entries: entries}
	data, _ := json.Marshal(cache)
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadCache(path)
	if err != nil {
		t.Errorf("expected success for cache with 50 entries, got: %v", err)
	}
}

func TestSaveCacheDirectoryCreation(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "new", "subdir", "pricing.json")
	entries := make([]PricingEntry, 50)
	for i := range entries {
		entries[i] = PricingEntry{Key: "model-" + string(rune('a'+i%26)), Input: float64(i+1)}
	}
	cache := &CacheEntry{
		FetchedAt: time.Now(),
		Entries:   entries,
	}
	if err := SaveCache(path, cache); err != nil {
		t.Fatalf("SaveCache should create directories: %v", err)
	}
	loaded, err := LoadCache(path)
	if err != nil {
		t.Fatalf("LoadCache: %v", err)
	}
	if len(loaded.Entries) != 50 {
		t.Errorf("expected 50 entries, got %d", len(loaded.Entries))
	}
}

func TestLoadCache_Expired(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "expired.json")
	entries := make([]PricingEntry, 50)
	for i := range entries {
		entries[i] = PricingEntry{Key: "model-" + string(rune('a'+i%26)), Input: float64(i+1)}
	}
	cache := &CacheEntry{
		FetchedAt: time.Now().Add(-8 * 24 * time.Hour),
		TTL:       1 * time.Hour,
		Entries:   entries,
	}
	data, _ := json.Marshal(cache)
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadCache(path)
	if err == nil {
		t.Error("expected error for expired cache")
	}
}
