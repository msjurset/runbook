package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/msjurset/runbook/internal/engine"
	"github.com/msjurset/runbook/internal/runbook"
)

// stepListModel manages the left panel showing all steps and their statuses.
type stepListModel struct {
	steps    []runbook.Step
	statuses []engine.StepStatus
	cursor   int
	width    int
	height   int
}

func newStepListModel(steps []runbook.Step) stepListModel {
	statuses := make([]engine.StepStatus, len(steps))
	return stepListModel{
		steps:    steps,
		statuses: statuses,
	}
}

func (m *stepListModel) setSize(w, h int) {
	m.width = w
	m.height = h
}

func (m *stepListModel) setStatus(index int, status engine.StepStatus) {
	if index >= 0 && index < len(m.statuses) {
		m.statuses[index] = status
	}
}

func (m *stepListModel) moveUp() {
	if m.cursor > 0 {
		m.cursor--
	}
}

func (m *stepListModel) moveDown() {
	if m.cursor < len(m.steps)-1 {
		m.cursor++
	}
}

func (m *stepListModel) focusStep(index int) {
	if index >= 0 && index < len(m.steps) {
		m.cursor = index
	}
}

func (m stepListModel) view() string {
	var lines []string

	// Inner width: panel width minus border (2) and padding (2)
	innerWidth := m.width - 4
	if innerWidth < 10 {
		innerWidth = 10
	}

	// Prefix width: cursor (2) + indicator (3) + space (1) = 6
	prefixWidth := 6

	for i, step := range m.steps {
		indicator := statusIndicator(m.statuses[i])
		name := step.Name

		cursor := "  "
		if i == m.cursor {
			cursor = "▶ "
		}

		nameWidth := innerWidth - prefixWidth
		if nameWidth < 5 {
			nameWidth = 5
		}

		style := normalStepStyle
		if i == m.cursor {
			style = selectedStepStyle
		}

		if len(name) <= nameWidth {
			// Fits on one line
			line := fmt.Sprintf("%s%s %s", cursor, indicator, name)
			lines = append(lines, style.Render(line))
		} else {
			// Word-wrap: break at last space that fits
			first, rest := wordWrap(name, nameWidth)
			// Continuation indent: same column as the name starts on the first line
			// First line: cursor(2) + indicator(3) + space(1) = 6 chars before name
			contIndent := strings.Repeat(" ", prefixWidth)
			lines = append(lines, style.Render(fmt.Sprintf("%s%s %s", cursor, indicator, first)))
			for rest != "" {
				var next string
				next, rest = wordWrap(rest, nameWidth)
				lines = append(lines, style.Render(fmt.Sprintf("%s%s", contIndent, next)))
			}
		}
	}

	// Limit to available height
	maxLines := m.height - 2 // -2 for border
	if maxLines < 1 {
		maxLines = 1
	}
	if len(lines) > maxLines {
		lines = lines[:maxLines]
	}

	content := strings.Join(lines, "\n")

	// Pad to fill height
	lineCount := len(lines)
	for lineCount < maxLines {
		content += "\n"
		lineCount++
	}

	return stepListStyle.
		Width(m.width - 2). // -2 for border
		Height(maxLines).
		MaxHeight(m.height).
		MaxWidth(m.width).
		Render(content)
}

// wordWrap splits text at the last space that fits within width.
// Returns (first line, remainder). If no space is found, breaks at width.
func wordWrap(text string, width int) (string, string) {
	if len(text) <= width {
		return text, ""
	}
	// Find last space within width
	breakAt := strings.LastIndex(text[:width], " ")
	if breakAt <= 0 {
		// No space found — hard break
		return text[:width], text[width:]
	}
	return text[:breakAt], text[breakAt+1:]
}

func statusIndicator(status engine.StepStatus) string {
	switch status {
	case engine.StatusPending:
		return pendingIndicator
	case engine.StatusRunning:
		return runningIndicator
	case engine.StatusSuccess:
		return successIndicator
	case engine.StatusFailed:
		return failedIndicator
	case engine.StatusSkipped:
		return skippedIndicator
	case engine.StatusRetrying:
		return retryingIndicator
	default:
		return pendingIndicator
	}
}

// statusBarView returns the bottom status bar content.
func statusBarView(bookName string, stepCount int, activeStep int, activeStepName string, status engine.StepStatus, running bool, width int) string {
	var left, right string

	if running {
		left = statusBarStyle.Render(fmt.Sprintf(" %s ", bookName))
		stepInfo := fmt.Sprintf(" Step %d/%d: %s ", activeStep+1, stepCount, activeStepName)
		right = statusTextStyle.Render(stepInfo)
	} else {
		left = statusBarStyle.Render(fmt.Sprintf(" %s ", bookName))
		right = statusTextStyle.Render(" Done ")
	}

	help := helpStyle.Render("j/k:navigate • enter:view • s:skip • r:retry • y/n:confirm • q:quit")

	gap := width - lipgloss.Width(left) - lipgloss.Width(right) - lipgloss.Width(help)
	if gap < 0 {
		gap = 0
	}

	return left + right + strings.Repeat(" ", gap) + help
}
