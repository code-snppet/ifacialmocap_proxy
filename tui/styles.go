package tui

import "github.com/charmbracelet/lipgloss"

const (
	COLOR_WHITE    = lipgloss.Color("#F0F0F0")
	COLOR_GRAY     = lipgloss.Color("#7A7A7A")
	COLOR_DARKGRAY = lipgloss.Color("#444444")
	COLOR_BLACK    = lipgloss.Color("#111111")
	COLOR_PURPLE   = lipgloss.Color("#6C6CFF")
	COLOR_GREEN    = lipgloss.Color("#00D3A7")
	COLOR_ERROR    = lipgloss.Color("#FF6C6C")
	COLOR_YELLOW   = lipgloss.Color("#FFD75F")
)

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(COLOR_WHITE).
			Background(COLOR_PURPLE).
			Padding(0, 1)
	boldStyle   = lipgloss.NewStyle().Bold(true)
	subtleStyle = lipgloss.NewStyle().
			Foreground(COLOR_GRAY)
	sectionTitle = lipgloss.NewStyle().
			Bold(true).
			Foreground(COLOR_GRAY)
	sectionBox = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(COLOR_DARKGRAY).
			Padding(0, 1)
	footerStyle = lipgloss.NewStyle().
			Foreground(COLOR_WHITE).
			Background(COLOR_DARKGRAY).
			Padding(0, 1)
	errorStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(COLOR_ERROR)
	chipStyle = lipgloss.NewStyle().
			Foreground(COLOR_GREEN)

	redStyle    = lipgloss.NewStyle().Foreground(COLOR_ERROR)
	yellowStyle = lipgloss.NewStyle().Foreground(COLOR_YELLOW)
	greenStyle  = lipgloss.NewStyle().Foreground(COLOR_GREEN)
)
