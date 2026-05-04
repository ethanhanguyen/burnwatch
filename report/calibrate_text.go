package report

import (
	"fmt"
	"strings"

	"github.com/ethanhanguyen/burnwatch/analyze"
)

func FormatCalibrationText(report analyze.CalibrationReport) string {
	var b strings.Builder

	fmt.Fprintf(&b, "Your data: %d main sessions", report.TotalSessions)
	if report.TotalSubagents > 0 {
		fmt.Fprintf(&b, ", %d subagent sessions", report.TotalSubagents)
	}
	fmt.Fprintf(&b, " across %d projects\n", report.ProjectCount)
	fmt.Fprintf(&b, "Period: %s to %s\n\n", report.DateRangeStart, report.DateRangeEnd)

	writeDistSection(&b, "Session costs ($):", report.SessionCost, formatDollar, true, true)
	writeDistSection(&b, "Input tokens:", report.InputTokens, formatToken, true, true)
	writeDistSection(&b, "Output tokens:", report.OutputTokens, formatToken, true, true)
	writeDistSection(&b, "Output/input ratio:", report.Ratio, formatRatio, true, false)
	writeDistSection(&b, "Cache hit rate (%):", report.CacheHitRate, formatPercent, false, false)
	writeDistSection(&b, "Token efficiency ratio:", report.TokenEfficiency, formatRatio, true, false)

	if report.SubagentOverhead.Count > 0 {
		writeDistSectionWithNote(&b, "Subagent overhead (%):", report.SubagentOverhead, formatPercent, false, false,
			" (sessions with subagents)")
	}

	if report.ToolLoopMaxRepeats.Count > 0 {
		writeDistSectionWithNote(&b, "Max consecutive tool repeats per session:", report.ToolLoopMaxRepeats, formatToken, false, false,
			" (sessions with tool calls)")
	}

	if report.FileReReadMaxCount.Count > 0 {
		writeDistSectionWithNote(&b, "Max file re-read count per session:", report.FileReReadMaxCount, formatToken, false, false,
			" (sessions with file reads)")
	}

	if report.SubagentOverlapPcts.Count > 0 {
		writeDistSectionWithNote(&b, "Subagent-parent file overlap (%):", report.SubagentOverlapPcts, formatPercent, false, false,
			" (subagent-parent pairs)")
	}

	if report.RestartOverlapPcts.Count > 0 {
		writeDistSectionWithNote(&b, "Session restart overlap (%):", report.RestartOverlapPcts, formatPercent, false, false,
			" (consecutive session pairs)")
	}

	if len(report.Suggestions) > 0 {
		fmt.Fprintln(&b)
		fmt.Fprintln(&b, "Suggested thresholds (for .burnwatch.toml):")
		fmt.Fprintln(&b, "  [thresholds]")
		for _, sug := range report.Suggestions {
			valStr := fmt.Sprintf("%.1f", sug.Value)
			if sug.ConfigKey == "subagent_overhead_pct" {
				valStr = fmt.Sprintf("%.0f", sug.Value)
			}
			fmt.Fprintf(&b, "  %s = %s  # %s\n", sug.ConfigKey, valStr, sug.Rationale)
		}
	}

	return b.String()
}

func writeDistSection(b *strings.Builder, label string, ds analyze.DistStats, fmtFn func(float64) string, showMu, showSigma bool) {
	writeDistSectionWithNote(b, label, ds, fmtFn, showMu, showSigma, "")
}

func writeDistSectionWithNote(b *strings.Builder, label string, ds analyze.DistStats, fmtFn func(float64) string, showMu, showSigma bool, note string) {
	if ds.Count == 0 {
		fmt.Fprintf(b, "%s\n  n=0  (no data)\n\n", label)
		return
	}

	fmt.Fprintf(b, "%s\n", label)
	fmt.Fprintf(b, "  n=%d%s", ds.Count, note)

	if showMu {
		fmt.Fprintf(b, "  μ=%s", fmtFn(ds.Mean))
	}
	if showSigma {
		fmt.Fprintf(b, "  σ=%s", fmtFn(ds.Std))
	}
	fmt.Fprintln(b)

	fmt.Fprintf(b, "  P10=%s  P25=%s  P50=%s  P75=%s  P90=%s  P95=%s  P99=%s\n",
		fmtFn(ds.P10), fmtFn(ds.P25), fmtFn(ds.P50), fmtFn(ds.P75), fmtFn(ds.P90), fmtFn(ds.P95), fmtFn(ds.P99))

	fmt.Fprintf(b, "  min=%s  max=%s\n\n", fmtFn(ds.Min), fmtFn(ds.Max))
}

func formatDollar(v float64) string {
	return fmt.Sprintf("%.2f", v)
}

func formatToken(v float64) string {
	return fmt.Sprintf("%.0f", v)
}

func formatRatio(v float64) string {
	return fmt.Sprintf("%.2f", v)
}

func formatPercent(v float64) string {
	return fmt.Sprintf("%.1f", v)
}
