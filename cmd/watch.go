package cmd

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/ethanhanguyen/burnwatch/analyze"
	"github.com/ethanhanguyen/burnwatch/report"
	"github.com/ethanhanguyen/burnwatch/source"
)

type watchModel struct {
	state    *analyze.WatchState
	interval time.Duration
	sources  []source.Source
	err      error
	errMsg   string

	selectedPane  int
	selectedRow   int
	scrollOffset  int
	drillSession  string
	drillOffset   int

	width  int
	height int

	alertsSeen map[string]bool
}

type pollResultMsg struct {
	events []source.TokenEvent
}

type tickMsg struct{}

type errMsg struct {
	err error
}

type clearErrMsg struct{}

func handleWatch(sources []source.Source, intervalSec int) error {
	state := analyze.NewWatchState()

	m := watchModel{
		state:      state,
		interval:   time.Duration(intervalSec) * time.Second,
		sources:    sources,
		alertsSeen: make(map[string]bool),
	}

	p := tea.NewProgram(m)
	_, err := p.Run()
	return err
}

func (m watchModel) Init() tea.Cmd {
	return tea.Batch(
		pollSourcesCmd(m.sources),
		tickCmd(m.interval),
	)
}

func (m watchModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "enter":
			return m.handleEnter()
		case "esc":
			m.drillSession = ""
			m.drillOffset = 0
			return m, nil
		case "tab":
			m.selectedPane = 1 - m.selectedPane
			m.selectedRow = 0
			return m, nil
		case "up", "k":
			if m.drillSession != "" {
				m.drillOffset = max(0, m.drillOffset-1)
				return m, nil
			}
			m.selectedRow = max(0, m.selectedRow-1)
			return m, nil
		case "down", "j":
			if m.drillSession != "" {
				m.drillOffset++
				return m, nil
			}
			maxRow := m.sessionCount()
			if m.selectedPane == 1 {
				maxRow = len(m.state.Alerts)
			}
			m.selectedRow = min(maxRow-1, m.selectedRow+1)
			return m, nil
		case "pgup":
			if m.drillSession != "" {
				m.drillOffset = max(0, m.drillOffset-10)
			}
			return m, nil
		case "pgdown":
			if m.drillSession != "" {
				m.drillOffset += 10
			}
			return m, nil
		}
	case pollResultMsg:
		cfg := analyze.PartialDetectionConfig{
			ToolLoopMaxRepeats: 5,
			FileRereadMinCount: 4,
			SubagentOverlapPct: 50.0,
		}
		alerts := analyze.UpdateState(m.state, msg.events, cfg)
		for _, a := range alerts {
			key := fmt.Sprintf("%s:%s:%s", a.SessionID, a.Reason, a.Detail)
			if !m.alertsSeen[key] {
				m.alertsSeen[key] = true
				m.state.Alerts = append([]analyze.WatchAlert{a}, m.state.Alerts...)
			}
		}
		if len(m.state.Alerts) > 20 {
			m.state.Alerts = m.state.Alerts[:20]
		}
		m.errMsg = ""
		return m, tea.Batch(pollSourcesCmd(m.sources), tickCmd(m.interval))
	case tickMsg:
		return m, pollSourcesCmd(m.sources)
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case errMsg:
		m.err = msg.err
		m.errMsg = msg.err.Error()
		return m, tea.Batch(
			func() tea.Msg { return clearErrMsg{} },
			tea.Tick(3*time.Second, func(time.Time) tea.Msg { return clearErrMsg{} }),
		)
	case clearErrMsg:
		m.errMsg = ""
		return m, nil
	}
	return m, nil
}

func (m watchModel) handleEnter() (tea.Model, tea.Cmd) {
	if m.drillSession != "" {
		return m, nil
	}

	if m.selectedPane == 0 {
		sid := m.selectedSessionID()
		if sid != "" {
			m.drillSession = sid
			m.drillOffset = 0
		}
	} else {
		if m.selectedRow < len(m.state.Alerts) {
			m.drillSession = m.state.Alerts[m.selectedRow].SessionID
			m.selectedPane = 0
			m.drillOffset = 0
		}
	}
	return m, nil
}

func (m watchModel) selectedSessionID() string {
	i := 0
	for _, ws := range m.orderedSessions() {
		if i == m.selectedRow {
			return ws.SessionID
		}
		i++
	}
	return ""
}

func (m watchModel) sessionCount() int {
	count := 0
	for range m.orderedSessions() {
		count++
	}
	return count
}

func (m watchModel) orderedSessions() []*analyze.WatchSession {
	var sessions []*analyze.WatchSession
	for _, ws := range m.state.Sessions {
		sessions = append(sessions, ws)
	}
	for i := 0; i < len(sessions); i++ {
		for j := i + 1; j < len(sessions); j++ {
			if sessions[j].StartedAt.After(sessions[i].StartedAt) {
				sessions[i], sessions[j] = sessions[j], sessions[i]
			}
		}
	}
	return sessions
}

func (m watchModel) View() string {
	if m.width == 0 {
		return "Initializing..."
	}

	if m.drillSession != "" {
		return m.renderDrillDown()
	}

	header := report.RenderWatchHeader(m.state, int(m.interval.Seconds()))

	sessions := m.renderSessionPane()
	alerts := m.renderAlertPane()

	leftWidth := m.width / 2
	rightWidth := m.width - leftWidth - 1

	leftStyled := lipgloss.NewStyle().Width(leftWidth).Render(sessions)
	rightStyled := lipgloss.NewStyle().Width(rightWidth).Render(alerts)

	body := lipgloss.JoinHorizontal(lipgloss.Top, leftStyled, rightStyled)

	footer := report.RenderWatchFooter(m.selectedPane)

	if m.errMsg != "" {
		errLine := lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")).
			Bold(true).
			Render(" Error: " + m.errMsg)
		return header + "\n" + body + "\n" + footer + "\n" + errLine
	}

	return header + "\n" + body + "\n" + footer
}

func (m watchModel) renderSessionPane() string {
	var b strings.Builder

	sessions := m.orderedSessions()
	for i, ws := range sessions {
		style := lipgloss.NewStyle()
		if i == m.selectedRow && m.selectedPane == 0 {
			style = style.Background(lipgloss.Color("236"))
		}
		row := report.RenderSessionRow(ws, m.width/2-2)
		b.WriteString(style.Render(row))
		b.WriteByte('\n')
	}

	maxVisible := m.height - 5
	if m.selectedRow >= m.scrollOffset+maxVisible {
		m.scrollOffset = m.selectedRow - maxVisible + 1
	}
	if m.selectedRow < m.scrollOffset {
		m.scrollOffset = m.selectedRow
	}

	pane := lipgloss.NewStyle().
		Height(maxVisible).
		MaxHeight(maxVisible).
		Render(b.String())

	return pane
}

func (m watchModel) renderAlertPane() string {
	var b strings.Builder
	b.WriteString("  ALERTS\n")
	b.WriteString("  " + strings.Repeat("\u2500", m.width/2-4) + "\n")

	maxVisible := m.height - 6
	displayed := 0

	for i, alert := range m.state.Alerts {
		if displayed >= maxVisible {
			break
		}

		style := lipgloss.NewStyle()
		if i == m.selectedRow && m.selectedPane == 1 {
			style = style.Background(lipgloss.Color("236"))
		}

		age := time.Since(alert.Time)
		if age > 5*time.Minute {
			style = style.Faint(true)
		}

		b.WriteString(style.Render(report.RenderAlertRow(&alert)))
		b.WriteByte('\n')
		displayed++
	}

	pane := lipgloss.NewStyle().
		Height(maxVisible).
		MaxHeight(maxVisible).
		Render(b.String())

	return pane
}

func (m watchModel) renderDrillDown() string {
	ws, ok := m.state.Sessions[m.drillSession]
	if !ok {
		return lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("62")).
			Width(m.width - 4).
			Height(m.height - 2).
			Render("Session not found in watch state.")
	}

	cfg := analyze.PartialDetectionConfig{
		ToolLoopMaxRepeats: 5,
		FileRereadMinCount: 4,
		SubagentOverlapPct: 50.0,
	}

	partialSignals := analyze.DetectPartialWaste(ws, cfg)

	explainText := report.FormatExplain(m.drillSession, ws.Events, partialSignals, nil)

	lines := strings.Split(explainText, "\n")
	viewHeight := m.height - 4
	if viewHeight < 1 {
		viewHeight = 1
	}

	start := m.drillOffset
	if start > len(lines)-viewHeight {
		start = len(lines) - viewHeight
	}
	if start < 0 {
		start = 0
	}

	var visibleLines []string
	end := start + viewHeight
	if end > len(lines) {
		end = len(lines)
	}
	visibleLines = lines[start:end]

	content := strings.Join(visibleLines, "\n")
	content += fmt.Sprintf("\n\n  [Line %d-%d/%d]  [ESC] back  [\u2191\u2193] scroll  [PgUp/PgDn] page",
		start+1, end, len(lines))

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("62")).
		Width(m.width - 4).
		Height(m.height - 2).
		Render(content)
}

func pollSourcesCmd(sources []source.Source) tea.Cmd {
	return func() tea.Msg {
		var allEvents []source.TokenEvent
		for _, src := range sources {
			evCh, errCh := src.Events()
			for e := range evCh {
				allEvents = append(allEvents, e)
			}
			for range errCh {
			}
		}
		return pollResultMsg{events: allEvents}
	}
}

func tickCmd(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(t time.Time) tea.Msg {
		return tickMsg{}
	})
}
