package tui

import "github.com/charmbracelet/lipgloss"

var (
	subtle    = lipgloss.AdaptiveColor{Light: "#D9DCCF", Dark: "#383838"}
	highlight = lipgloss.AdaptiveColor{Light: "#874BFD", Dark: "#7D56F4"}
	special   = lipgloss.AdaptiveColor{Light: "#43BF6D", Dark: "#73F59F"}
	dimText   = lipgloss.AdaptiveColor{Light: "#999999", Dark: "#666666"}
	errorText = lipgloss.AdaptiveColor{Light: "#FF0000", Dark: "#FF6666"}
	warnText  = lipgloss.AdaptiveColor{Light: "#FF8800", Dark: "#FFAA33"}

	// Step list styles
	stepListStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(subtle).
			Padding(0, 1)

	selectedStepStyle = lipgloss.NewStyle().
				Foreground(highlight).
				Bold(true)

	normalStepStyle = lipgloss.NewStyle()

	// Status indicators
	pendingIndicator  = lipgloss.NewStyle().Foreground(dimText).Render("[ ]")
	runningIndicator  = lipgloss.NewStyle().Foreground(highlight).Bold(true).Render("[▸]")
	successIndicator  = lipgloss.NewStyle().Foreground(special).Render("[✓]")
	failedIndicator   = lipgloss.NewStyle().Foreground(errorText).Render("[✗]")
	skippedIndicator  = lipgloss.NewStyle().Foreground(dimText).Render("[-]")
	retryingIndicator = lipgloss.NewStyle().Foreground(warnText).Render("[~]")

	// Output viewport styles
	outputStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(subtle).
			Padding(0, 1)

	// Status bar styles
	statusBarStyle = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#FFFDF5", Dark: "#FFFDF5"}).
			Background(lipgloss.AdaptiveColor{Light: "#6124DF", Dark: "#7D56F4"}).
			Padding(0, 1)

	statusTextStyle = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#FFFDF5", Dark: "#FFFDF5"}).
			Background(lipgloss.AdaptiveColor{Light: "#A550DF", Dark: "#6124DF"}).
			Padding(0, 1)

	helpStyle = lipgloss.NewStyle().
			Foreground(dimText).
			Padding(0, 1)

	// Title style
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(highlight).
			Padding(0, 1)
)
