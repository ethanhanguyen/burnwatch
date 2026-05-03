package analyze

import (
	"fmt"
	"sort"
	"time"

	"github.com/ethanhanguyen/burnwatch/source"
)

type WeeklyAgg struct {
	WeekStart    time.Time
	SessionCount int
	TotalCost    float64
	TotalInput   int64
	TotalOutput  int64
	Ratio        float64
}

type Trend struct {
	CostDirection    string
	CostChange       float64
	RatioDirection   string
	RatioChange      float64
	SessionDirection string
	SessionChange    float64

	First  WeeklyAgg
	Last   WeeklyAgg
}

func ComputeTrends(events []source.TokenEvent) *Trend {
	if len(events) == 0 {
		return nil
	}

	weeks := make(map[string]*WeeklyAgg)
	for _, e := range events {
		weekStart := weekStartOf(e.Timestamp)
		key := weekStart.Format("2006-01-02")
		a, ok := weeks[key]
		if !ok {
			a = &WeeklyAgg{WeekStart: weekStart}
			weeks[key] = a
		}
		a.TotalCost += e.CostUSD
		a.TotalInput += e.InputTokens
		a.TotalOutput += e.OutputTokens
	}

	sessions := make(map[string]map[string]bool)
	for _, e := range events {
		weekStart := weekStartOf(e.Timestamp)
		key := weekStart.Format("2006-01-02")
		if sessions[key] == nil {
			sessions[key] = make(map[string]bool)
		}
		sessions[key][e.SessionID] = true
	}
	for key, sessMap := range sessions {
		weeks[key].SessionCount = len(sessMap)
	}

	for _, w := range weeks {
		if w.TotalInput > 0 {
			w.Ratio = float64(w.TotalOutput) / float64(w.TotalInput)
		}
	}

	if len(weeks) <= 1 {
		return nil
	}

	sorted := make([]*WeeklyAgg, 0, len(weeks))
	for _, w := range weeks {
		sorted = append(sorted, w)
	}
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].WeekStart.Before(sorted[j].WeekStart)
	})

	first := *sorted[0]
	last := *sorted[len(sorted)-1]

	t := &Trend{First: first, Last: last}

	if first.TotalCost > 0 {
		t.CostChange = (last.TotalCost - first.TotalCost) / first.TotalCost * 100
	}
	if first.SessionCount > 0 {
		t.SessionChange = float64(last.SessionCount-first.SessionCount) / float64(first.SessionCount) * 100
	}
	if first.Ratio > 0 {
		t.RatioChange = (last.Ratio - first.Ratio) / first.Ratio * 100
	}

	t.CostDirection = arrow(t.CostChange)
	t.SessionDirection = arrow(t.SessionChange)
	t.RatioDirection = arrow(t.RatioChange)

	return t
}

func weekStartOf(t time.Time) time.Time {
	weekday := t.Weekday()
	diff := int(weekday) - int(time.Monday)
	if diff < 0 {
		diff += 7
	}
	return t.Truncate(24 * time.Hour).AddDate(0, 0, -diff)
}

func arrow(change float64) string {
	if change > 0 {
		return "↑"
	}
	if change < 0 {
		return "↓"
	}
	return "→"
}

func (t *Trend) Format() string {
	return fmt.Sprintf("Trends:\n"+
		"  Cost:    $%.2f/wk → $%.2f/wk (%s %.0f%%)\n"+
		"  Sessions: %d/wk → %d/wk (%s %.0f%%)\n"+
		"  Output/input ratio: %.2f → %.2f (%s %.0f%%)",
		t.First.TotalCost, t.Last.TotalCost, t.CostDirection, absFloat(t.CostChange),
		t.First.SessionCount, t.Last.SessionCount, t.SessionDirection, absFloat(t.SessionChange),
		t.First.Ratio, t.Last.Ratio, t.RatioDirection, absFloat(t.RatioChange),
	)
}

func absFloat(f float64) float64 {
	if f < 0 {
		return -f
	}
	return f
}
