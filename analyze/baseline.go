package analyze

import (
	"math"
	"sort"

	"github.com/ethanhanguyen/burnwatch/source"
)

type Baseline struct {
	Project      string
	Harness      string
	SessionCount int
	CostMean     float64
	CostStd      float64
	RatioMean    float64
	RatioP10     float64
	RatioP50     float64
	RatioP90     float64
	CacheP10     float64
	CacheP50     float64
	InputMean    float64   `json:"input_mean"`
	InputStd     float64   `json:"input_std"`
	InputP50     float64   `json:"input_p50"`
	InputP90     float64   `json:"input_p90"`
	OutputMean   float64   `json:"output_mean"`
	OutputStd    float64   `json:"output_std"`
	OutputP50    float64   `json:"output_p50"`
	OutputP90    float64   `json:"output_p90"`
	TERP10       float64   `json:"ter_p10"`
	SessionCosts []float64 `json:"session_costs,omitempty"`
	Ratios       []float64 `json:"ratios,omitempty"`
	CacheRates   []float64 `json:"cache_rates,omitempty"`
	InputTokens  []float64 `json:"input_tokens,omitempty"`
	OutputTokens []float64 `json:"output_tokens,omitempty"`
	TERs         []float64 `json:"ters,omitempty"`
}

const globalKey = "*"

func ComputeBaselines(events []source.TokenEvent) map[string]Baseline {
	if len(events) == 0 {
		return nil
	}

	groups := make(map[string][]source.TokenEvent)
	for _, e := range events {
		key := e.Project + ":" + e.Harness
		groups[key] = append(groups[key], e)
	}

	var allSessionMetrics []sessionMetrics
	result := make(map[string]Baseline)

	for key, group := range groups {
		sessionGroups := make(map[string][]source.TokenEvent)
		for _, e := range group {
			sessionGroups[e.SessionID] = append(sessionGroups[e.SessionID], e)
		}

		var metrics []sessionMetrics
		for sessionID, sessionEvents := range sessionGroups {
			m := aggregateMetrics(sessionID, sessionEvents)
			metrics = append(metrics, m)
			allSessionMetrics = append(allSessionMetrics, m)
		}

		b := buildBaseline(key, metrics)
		result[key] = b
	}

	global := buildBaseline(globalKey, allSessionMetrics)
	result[globalKey] = global

	return result
}

type sessionMetrics struct {
	sessionID  string
	cost       float64
	ratio      float64
	cacheRate  float64
	ter        float64
	inputSum   int64
	outputSum  int64
	cacheRead  int64
	cacheWrite int64
}

func aggregateMetrics(sessionID string, events []source.TokenEvent) sessionMetrics {
	m := sessionMetrics{sessionID: sessionID}
	for _, e := range events {
		m.cost += e.CostUSD

		in := e.InputTokens
		out := e.OutputTokens
		cr := e.CacheRead
		cw := e.CacheWrite

		if in < 0 {
			in = 0
		}
		if out < 0 {
			out = 0
		}
		if cr < 0 {
			cr = 0
		}
		if cw < 0 {
			cw = 0
		}

		m.inputSum += in
		m.outputSum += out
		m.cacheRead += cr
		m.cacheWrite += cw
	}

	if m.inputSum > 0 {
		m.ratio = float64(m.outputSum) / float64(m.inputSum)
	}

	if totalCache := m.cacheRead + m.cacheWrite; totalCache > 0 {
		m.cacheRate = float64(m.cacheRead) / float64(totalCache)
	}

	if m.inputSum+m.cacheWrite > 0 {
		m.ter = float64(m.outputSum+m.cacheRead) / float64(m.inputSum+m.cacheWrite)
	}

	return m
}

func buildBaseline(key string, metrics []sessionMetrics) Baseline {
	proj, harness := splitKey(key)
	b := Baseline{
		Project:      proj,
		Harness:      harness,
		SessionCount: len(metrics),
	}

	n := len(metrics)

	b.SessionCosts = make([]float64, n)
	for i, m := range metrics {
		b.SessionCosts[i] = m.cost
	}
	sort.Float64s(b.SessionCosts)

	var sumCost float64
	for _, c := range b.SessionCosts {
		sumCost += c
	}
	if n > 0 {
		b.CostMean = sumCost / float64(n)
	}
	if n > 1 {
		b.CostStd = stddev(b.SessionCosts, b.CostMean)
	}

	b.Ratios = make([]float64, n)
	for i, m := range metrics {
		b.Ratios[i] = m.ratio
	}
	sort.Float64s(b.Ratios)

	b.CacheRates = make([]float64, n)
	for i, m := range metrics {
		b.CacheRates[i] = m.cacheRate
	}
	sort.Float64s(b.CacheRates)

	b.RatioMean = mean(b.Ratios)
	b.RatioP10 = percentile(b.Ratios, 10)
	b.RatioP50 = percentile(b.Ratios, 50)
	b.RatioP90 = percentile(b.Ratios, 90)
	b.CacheP10 = percentile(b.CacheRates, 10)
	b.CacheP50 = percentile(b.CacheRates, 50)

	b.InputTokens = make([]float64, n)
	for i, m := range metrics {
		b.InputTokens[i] = float64(m.inputSum)
	}
	sort.Float64s(b.InputTokens)
	b.InputMean = mean(b.InputTokens)
	if n > 1 {
		b.InputStd = stddev(b.InputTokens, b.InputMean)
	}
	b.InputP50 = percentile(b.InputTokens, 50)
	b.InputP90 = percentile(b.InputTokens, 90)

	b.OutputTokens = make([]float64, n)
	for i, m := range metrics {
		b.OutputTokens[i] = float64(m.outputSum)
	}
	sort.Float64s(b.OutputTokens)
	b.OutputMean = mean(b.OutputTokens)
	if n > 1 {
		b.OutputStd = stddev(b.OutputTokens, b.OutputMean)
	}
	b.OutputP50 = percentile(b.OutputTokens, 50)
	b.OutputP90 = percentile(b.OutputTokens, 90)

	b.TERs = make([]float64, n)
	for i, m := range metrics {
		b.TERs[i] = m.ter
	}
	sort.Float64s(b.TERs)
	b.TERP10 = percentile(b.TERs, 10)

	return b
}

func splitKey(key string) (string, string) {
	for i := len(key) - 1; i >= 0; i-- {
		if key[i] == ':' {
			return key[:i], key[i+1:]
		}
	}
	return key, ""
}

func mean(vals []float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	var sum float64
	for _, v := range vals {
		sum += v
	}
	return sum / float64(len(vals))
}

func stddev(sorted []float64, mean float64) float64 {
	var sumSq float64
	for _, v := range sorted {
		d := v - mean
		sumSq += d * d
	}
	return math.Sqrt(sumSq / float64(len(sorted)))
}

func percentile(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	if len(sorted) == 1 {
		return sorted[0]
	}
	k := (p / 100.0) * float64(len(sorted)-1)
	lo := int(math.Floor(k))
	hi := int(math.Ceil(k))
	if lo == hi {
		return sorted[lo]
	}
	return sorted[lo]*(float64(hi)-k) + sorted[hi]*(k-float64(lo))
}
