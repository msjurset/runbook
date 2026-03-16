package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// outputModel manages the right panel showing step output.
type outputModel struct {
	viewport viewport.Model
	lines    map[int][]string // per-step output lines
	active   int              // which step's output to show
	width    int
	height   int
	ready    bool
}

func newOutputModel() outputModel {
	return outputModel{
		lines: make(map[int][]string),
	}
}

func (m *outputModel) setSize(w, h int) {
	m.width = w
	m.height = h

	vpWidth := w - 4  // -2 border, -2 padding
	vpHeight := h - 4 // -2 border, -1 header, -1 padding
	if vpWidth < 10 {
		vpWidth = 10
	}
	if vpHeight < 1 {
		vpHeight = 1
	}

	if !m.ready {
		m.viewport = viewport.New(vpWidth, vpHeight)
		m.ready = true
	} else {
		m.viewport.Width = vpWidth
		m.viewport.Height = vpHeight
	}
	m.refreshContent()
}

func (m *outputModel) addLine(stepIndex int, line string) {
	m.lines[stepIndex] = append(m.lines[stepIndex], line)
	if stepIndex == m.active {
		m.refreshContent()
		m.viewport.GotoBottom()
	}
}

func (m *outputModel) clearStep(index int) {
	delete(m.lines, index)
	if index == m.active {
		m.refreshContent()
	}
}

func (m *outputModel) showStep(index int) {
	m.active = index
	m.refreshContent()
	m.viewport.GotoBottom()
}

func (m *outputModel) refreshContent() {
	lines := m.lines[m.active]
	maxWidth := m.viewport.Width
	if maxWidth < 10 {
		maxWidth = 10
	}
	var processed []string
	for _, line := range lines {
		// Strip ANSI codes for clean handling, then truncate/wrap
		clean := ansi.Strip(line)
		for len(clean) > maxWidth {
			processed = append(processed, clean[:maxWidth])
			clean = clean[maxWidth:]
		}
		processed = append(processed, clean)
	}
	m.viewport.SetContent(strings.Join(processed, "\n"))
}

func (m *outputModel) scrollUp() {
	m.viewport.LineUp(1)
}

func (m *outputModel) scrollDown() {
	m.viewport.LineDown(1)
}

func (m *outputModel) pageUp() {
	m.viewport.HalfViewUp()
}

func (m *outputModel) pageDown() {
	m.viewport.HalfViewDown()
}

func (m outputModel) view() string {
	if !m.ready {
		return ""
	}

	header := lipgloss.NewStyle().
		Foreground(highlight).
		Bold(true).
		Padding(0, 0).
		Render(fmt.Sprintf("Output — Step %d", m.active+1))

	content := lipgloss.JoinVertical(lipgloss.Left,
		header,
		m.viewport.View(),
	)

	return outputStyle.
		Width(m.width - 2).
		Height(m.height - 2).
		MaxHeight(m.height).
		MaxWidth(m.width).
		Render(content)
}
