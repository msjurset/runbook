package tui

import (
	"context"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/msjurset/runbook/internal/engine"
	"github.com/msjurset/runbook/internal/runbook"
)

// clipBlock hard-clips a rendered block to the given width and height.
// This ensures no terminal overflow regardless of content.
func clipBlock(rendered string, width, height int) string {
	lines := strings.Split(rendered, "\n")
	if len(lines) > height {
		lines = lines[:height]
	}
	for i, line := range lines {
		if ansi.StringWidth(line) > width {
			lines[i] = ansi.Truncate(line, width, "")
		}
	}
	// Pad short lines to full width so the panel is solid
	for i, line := range lines {
		w := ansi.StringWidth(line)
		if w < width {
			lines[i] = line + strings.Repeat(" ", width-w)
		}
	}
	// Pad missing rows
	for len(lines) < height {
		lines = append(lines, strings.Repeat(" ", width))
	}
	return strings.Join(lines, "\n")
}

// Model is the top-level Bubble Tea model for the TUI.
type Model struct {
	book     *runbook.Runbook
	vars     map[string]string
	eng      *engine.Engine
	ctx      context.Context
	cancel   context.CancelFunc
	program  *tea.Program

	stepList stepListModel
	output   outputModel

	activeStep int
	running    bool
	done       bool
	result     *engine.RunResult

	// Prompt state
	prompting bool
	promptMsg string
	promptCh  chan bool

	width  int
	height int
}

// New creates a new TUI model.
func New(book *runbook.Runbook, vars map[string]string, ctx context.Context, cancel context.CancelFunc) Model {
	return Model{
		book:     book,
		vars:     vars,
		ctx:      ctx,
		cancel:   cancel,
		stepList: newStepListModel(book.Steps),
		output:   newOutputModel(),
	}
}

// SetProgram sets the tea.Program reference (needed for the observer).
func (m *Model) SetProgram(p *tea.Program) {
	m.program = p
}

func (m *Model) Init() tea.Cmd {
	return func() tea.Msg {
		return runStartMsg{}
	}
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.layout()
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)

	case runStartMsg:
		obs := &tuiObserver{program: m.program}
		m.eng = engine.New(m.book, m.vars, obs)
		m.running = true
		return m, func() tea.Msg {
			result := m.eng.Run(m.ctx)
			return runCompletedMsg{result: result}
		}

	case stepStartedMsg:
		m.activeStep = msg.index
		m.stepList.setStatus(msg.index, engine.StatusRunning)
		m.stepList.focusStep(msg.index)
		m.output.showStep(msg.index)
		return m, nil

	case stepOutputMsg:
		m.output.addLine(msg.index, msg.line)
		return m, nil

	case stepCompletedMsg:
		m.stepList.setStatus(msg.index, msg.result.Status)
		return m, nil

	case runCompletedMsg:
		m.running = false
		m.done = true
		m.result = &msg.result

		// Add a summary line to the output of the last active step
		if msg.result.Success {
			m.output.addLine(m.activeStep, "")
			m.output.addLine(m.activeStep, fmt.Sprintf("✓ Runbook completed successfully (%s)", msg.result.Duration.Round(100*1e6)))
		} else {
			failed := 0
			for _, s := range msg.result.Steps {
				if s.Status == engine.StatusFailed {
					failed++
				}
			}
			m.output.addLine(m.activeStep, "")
			m.output.addLine(m.activeStep, fmt.Sprintf("✗ Runbook failed (%d step(s) failed, %s)", failed, msg.result.Duration.Round(100*1e6)))
		}
		return m, nil

	case promptMsg:
		m.prompting = true
		m.promptMsg = msg.message
		m.promptCh = msg.respCh
		m.output.addLine(msg.index, "")
		m.output.addLine(msg.index, fmt.Sprintf("? %s [y/n]", msg.message))
		return m, nil
	}

	return m, nil
}

func (m *Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Handle prompts first
	if m.prompting {
		switch msg.String() {
		case "y", "Y":
			m.prompting = false
			m.promptCh <- true
			return m, nil
		case "n", "N":
			m.prompting = false
			m.promptCh <- false
			return m, nil
		}
		return m, nil
	}

	switch msg.String() {
	case "q", "ctrl+c":
		m.cancel()
		return m, tea.Quit

	case "j", "down":
		m.stepList.moveDown()
		m.output.showStep(m.stepList.cursor)
		return m, nil

	case "k", "up":
		m.stepList.moveUp()
		m.output.showStep(m.stepList.cursor)
		return m, nil

	case "enter":
		m.output.showStep(m.stepList.cursor)
		return m, nil

	case "s":
		if m.running && m.eng != nil {
			m.eng.SkipStep(m.stepList.cursor)
			m.stepList.setStatus(m.stepList.cursor, engine.StatusSkipped)
		}
		return m, nil

	case "r":
		// Retry from the selected step (only when done and a step failed)
		if m.done && m.eng != nil {
			idx := m.stepList.cursor
			if idx >= 0 && idx < len(m.stepList.statuses) &&
				m.stepList.statuses[idx] == engine.StatusFailed {
				m.done = false
				m.running = true
				// Reset statuses for this step and all after it
				for j := idx; j < len(m.stepList.statuses); j++ {
					m.stepList.setStatus(j, engine.StatusPending)
					m.output.clearStep(j)
				}
				m.result = nil
				return m, func() tea.Msg {
					result := m.eng.RunFrom(m.ctx, idx)
					return runCompletedMsg{result: result}
				}
			}
		}
		return m, nil

	case "pgup", "ctrl+u":
		m.output.pageUp()
		return m, nil

	case "pgdown", "ctrl+d":
		m.output.pageDown()
		return m, nil
	}

	return m, nil
}

func (m *Model) layout() {
	listWidth := m.width * 30 / 100
	if listWidth < 20 {
		listWidth = 20
	}
	if listWidth > 40 {
		listWidth = 40
	}
	outputWidth := m.width - listWidth

	contentHeight := m.height - 2 // -2 for status bar + help

	m.stepList.setSize(listWidth, contentHeight)
	m.output.setSize(outputWidth, contentHeight)
}

func (m *Model) View() string {
	if m.width == 0 {
		return "Loading..."
	}

	// Main content: step list + output, hard-clipped to allocated sizes
	listWidth := m.width * 30 / 100
	if listWidth < 20 {
		listWidth = 20
	}
	if listWidth > 40 {
		listWidth = 40
	}
	outputWidth := m.width - listWidth
	contentHeight := m.height - 2

	leftPanel := clipBlock(m.stepList.view(), listWidth, contentHeight)
	rightPanel := clipBlock(m.output.view(), outputWidth, contentHeight)

	panels := lipgloss.JoinHorizontal(lipgloss.Top,
		leftPanel,
		rightPanel,
	)

	// Status bar
	var bar string
	stepName := ""
	if m.activeStep < len(m.book.Steps) {
		stepName = m.book.Steps[m.activeStep].Name
	}

	if m.prompting {
		bar = statusBarStyle.Render(fmt.Sprintf(" ? %s [y/n] ", m.promptMsg))
		gap := m.width - lipgloss.Width(bar)
		if gap > 0 {
			bar += lipgloss.NewStyle().
				Background(lipgloss.AdaptiveColor{Light: "#A550DF", Dark: "#6124DF"}).
				Render(fmt.Sprintf("%*s", gap, ""))
		}
	} else {
		status := engine.StatusPending
		if m.activeStep < len(m.stepList.statuses) {
			status = m.stepList.statuses[m.activeStep]
		}
		bar = statusBarView(m.book.Name, len(m.book.Steps), m.activeStep, stepName, status, m.running, m.width)
	}

	return lipgloss.JoinVertical(lipgloss.Left, panels, bar)
}

// Run starts the TUI program and blocks until it exits.
// Returns the run result (may be nil if cancelled before completion) and any error.
func Run(book *runbook.Runbook, vars map[string]string, ctx context.Context, cancel context.CancelFunc) (*engine.RunResult, error) {
	m := New(book, vars, ctx, cancel)
	p := tea.NewProgram(&m, tea.WithAltScreen())
	m.SetProgram(p)
	_, err := p.Run()
	if err != nil {
		return nil, fmt.Errorf("tui error: %w", err)
	}

	// m is updated in place via pointer receiver methods
	if m.result != nil && !m.result.Success {
		return m.result, fmt.Errorf("runbook failed")
	}
	return m.result, nil
}

// tuiObserver implements engine.Observer by sending Bubble Tea messages.
type tuiObserver struct {
	program *tea.Program
}

func (o *tuiObserver) OnStepStart(index int, step runbook.Step) {
	o.program.Send(stepStartedMsg{index: index, step: step})
}

func (o *tuiObserver) OnStepOutput(index int, line string) {
	o.program.Send(stepOutputMsg{index: index, line: line})
}

func (o *tuiObserver) OnStepComplete(index int, result engine.StepResult) {
	o.program.Send(stepCompletedMsg{index: index, result: result})
}

func (o *tuiObserver) OnRunComplete(result engine.RunResult) {
	o.program.Send(runCompletedMsg{result: result})
}

func (o *tuiObserver) OnPrompt(index int, message string) (bool, error) {
	ch := make(chan bool, 1)
	o.program.Send(promptMsg{index: index, message: message, respCh: ch})
	resp := <-ch
	return resp, nil
}
