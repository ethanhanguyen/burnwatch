package report

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/ethanhanguyen/burnwatch/analyze"
)

var (
	watchStatusActive = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	watchStatusIdle   = lipgloss.NewStyle().Foreground(lipgloss.Color("220"))

	watchSeverityHigh  = lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true)
	watchSeverityMed   = lipgloss.NewStyle().Foreground(lipgloss.Color("220"))
	watchSeverityLow   = lipgloss.NewStyle().Foreground(lipgloss.Color("39"))

	watchSpBar   = lipgloss.NewStyle().Foreground(lipgloss.Color("99"))
	watchFaint   = lipgloss.NewStyle().Foreground(lipgloss.Color("243"))

	watchHeaderStyle = lipgloss.NewStyle().
				Background(lipgloss.Color("62")).
				Foreground(lipgloss.Color("255")).
				Bold(true)
)

func RenderSparkline(freq [10]int) string {
	allZero := true
	maxVal := 0
	for _, f := range freq {
		if f > 0 {
			allZero = false
		}
		if f > maxVal {
			maxVal = f
		}
	}
	if allZero {
		return strings.Repeat(" ", 10)
	}

	blocks := []string{" ", "\u2581", "\u2582", "\u2583", "\u2584", "\u2585", "\u2586", "\u2587", "\u2588"}
	var sb strings.Builder
	for _, f := range freq {
		if maxVal == 0 {
			sb.WriteByte(' ')
			continue
		}
		idx := int(float64(f) / float64(maxVal) * 8.0)
		if idx >= len(blocks) {
			idx = len(blocks) - 1
		} else if idx < 0 {
			idx = 0
		}
		if f > 0 {
			sb.WriteString(watchSpBar.Render(blocks[idx]))
		} else {
			sb.WriteByte(' ')
		}
	}
	return sb.String()
}

func RenderSessionRow(ws *analyze.WatchSession, width int) string {
	var b strings.Builder

	statusDot := watchStatusActive.Render("\u25CF")
	if ws.Idle {
		statusDot = watchStatusIdle.Render("\u25CC")
	}

	dur := formatWatchDuration(time.Since(ws.StartedAt))
	costStr := fmt.Sprintf("$%.2f", ws.Cost)
	if ws.Cost == 0 {
		costStr = "$0.00"
	}

	sparkline := RenderSparkline(ws.EventFreq)

	fmt.Fprintf(&b, "%s %s  %s  %s\n", statusDot, ws.SessionID, ws.Harness, dur)
	fmt.Fprintf(&b, "  %s  %s  %s  %s  %s\n",
		ws.Project, costStr, ws.Model, sparkline, lastActionStr(ws.LastAction))
	fmt.Fprintf(&b, "  %d ev", len(ws.Events))

	return b.String()
}

func RenderAlertRow(alert *analyze.WatchAlert) string {
	var sevStyle lipgloss.Style
	switch alert.Severity {
	case "high":
		sevStyle = watchSeverityHigh
	case "medium":
		sevStyle = watchSeverityMed
	default:
		sevStyle = watchSeverityLow
	}

	sevLabel := strings.ToUpper(alert.Severity)
	if len(sevLabel) < 5 {
		sevLabel = sevLabel + strings.Repeat(" ", 5-len(sevLabel))
	}

	timeStr := alert.Time.Format("15:04")

	age := time.Since(alert.Time)
	var reasonStr string
	if age > 5*time.Minute {
		reasonStr = watchFaint.Render(fmt.Sprintf("%s %s", timeStr, alert.Reason))
	} else {
		reasonStr = fmt.Sprintf("%s %s", timeStr, alert.Reason)
	}

	return fmt.Sprintf("%s %s  %s\n  %s",
		sevStyle.Render("\u26A0 "+sevLabel), reasonStr, alert.SessionID, alert.Detail)
}

func formatWatchDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		m := int(d.Minutes())
		s := int(d.Seconds()) % 60
		return fmt.Sprintf("%dm%ds", m, s)
	}
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	return fmt.Sprintf("%dh%dm", h, m)
}

func lastActionStr(action string) string {
	if action == "" {
		return ""
	}
	if len(action) > 30 {
		return action[:27] + "..."
	}
	return action
}

func RenderWatchHeader(state *analyze.WatchState, intervalSec int) string {
	active := 0
	for _, ws := range state.Sessions {
		if !ws.Idle {
			active++
		}
	}

	var costRate float64
	var todayCost float64
	now := time.Now()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	for _, ws := range state.Sessions {
		if ws.Cost > 0 && ws.StartedAt.After(todayStart) {
			dur := now.Sub(ws.StartedAt).Minutes()
			if dur > 0 {
				costRate += ws.Cost / dur
			}
		}
		if ws.StartedAt.After(todayStart) {
			todayCost += ws.Cost
		}
	}

	title := fmt.Sprintf(" burnwatch watch [\u27F3 %ds]  Active: %d  Rate: $%.2f/min  Today: $%.2f ",
		intervalSec, active, costRate, todayCost)

	return watchHeaderStyle.Render(title)
}

func RenderWatchFooter(selectedPane int) string {
	return watchFaint.Render(fmt.Sprintf(
		" [TAB] switch pane  [\u2191\u2193] navigate  [ENTER] drill-in  [Q] quit  | Pane: %s",
		map[int]string{0: "Sessions", 1: "Alerts"}[selectedPane],
	))
}
