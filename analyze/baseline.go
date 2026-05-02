package analyze

import (
	"math"
	"sort"

	"github.com/yourname/burnwatch/source"
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
	SessionCosts []float64
	Ratios       []float64
	CacheRates   []float64
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
