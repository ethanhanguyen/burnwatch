package source

import "strings"

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

func CostForModel(model string, inputTokens, outputTokens, cacheRead, cacheWrite int64) float64 {
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

	model = strings.ToLower(model)
	p := fallback
	for _, entry := range pricing {
		if strings.Contains(model, entry.key) {
			p = entry.p
			break
		}
	}

	cost := float64(inputTokens)/1000.0*p.input +
		float64(outputTokens)/1000.0*p.output +
		float64(cacheRead)/1000.0*p.cacheRead +
		float64(cacheWrite)/1000.0*p.cacheWrite

	return cost
}
