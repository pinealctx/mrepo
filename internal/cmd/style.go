package cmd

import (
	"charm.land/lipgloss/v2"
)

var (
	// Semantic colors.
	successStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#22C55E"))
	errorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#EF4444"))
	warnStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#F59E0B"))
	infoStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280"))
	boldStyle    = lipgloss.NewStyle().Bold(true)
	dimStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280"))
	accentStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#7C3AED"))

	// Status-specific colors.
	cleanStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("#22C55E"))
	dirtyStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("#F59E0B"))
	stagedStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#3B82F6"))
	untrackedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#8B5CF6"))
	missingStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#EF4444")).Bold(true)
)

// Prefixes for different result types.
func successIcon() string { return successStyle.Render("✓") }
func errorIcon() string   { return errorStyle.Render("✗") }
func warnIcon() string    { return warnStyle.Render("!") }
func cloneIcon() string   { return successStyle.Render("↓") }
func infoIcon() string    { return infoStyle.Render("→") }

func formatStatus(status string) string {
	switch status {
	case "clean":
		return cleanStyle.Render("clean")
	case "dirty":
		return dirtyStyle.Render("dirty")
	case "staged":
		return stagedStyle.Render("staged")
	case "untracked":
		return untrackedStyle.Render("untracked")
	case "missing":
		return missingStyle.Render("MISSING")
	default:
		return status
	}
}
