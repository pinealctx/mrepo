package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/pinealctx/mrepo/internal/git"
)

// --- Styles ---

var (
	cleanStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("#22C55E"))
	dirtyStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("#F59E0B"))
	errorStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("#EF4444"))
	stagedStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#3B82F6"))
	untrackedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#8B5CF6"))
	dimStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280"))
	accentStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#14B8A6"))
	boldStyle      = lipgloss.NewStyle().Bold(true)

	helpStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("#4B5563"))
	focusDotStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#14B8A6")).Bold(true)
	brandStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#14B8A6")).Bold(true)

	panelBorder = lipgloss.RoundedBorder()
	sectionBox  = lipgloss.NewStyle().
			Border(panelBorder).
			BorderForeground(lipgloss.Color("#374151"))
	focusedBox = lipgloss.NewStyle().
			Border(panelBorder).
			BorderForeground(lipgloss.Color("#14B8A6"))
)

// --- View ---

func (m model) View() tea.View {
	if m.width == 0 {
		v := tea.NewView("Loading...")
		v.AltScreen = true
		return v
	}

	leftW := clamp(int(float64(m.width)*0.30), 28, 44)
	rightW := m.width - leftW - 1

	// Main body height: terminal height minus 1 line for status bar.
	bodyH := m.height - 1

	// Left panel: 3 bordered sections.
	// Each section = content + 2 border lines. Total borders = 6.
	borderOverhead := 6
	available := max(6, bodyH-borderOverhead)

	// Proportions: Repos 40%, Branches 30%, Files 30%.
	repoContentH := available * 40 / 100
	branchContentH := available * 30 / 100
	fileContentH := available - repoContentH - branchContentH

	// lipgloss Width/Height handles all content sizing and border rendering.
	// No manual width calculation — lipgloss uses displaywidth internally,
	// so content sizing and border rendering are always consistent.
	innerW := leftW - 2 // inner width = box width minus left+right border

	repoBox := sectionBoxForFocus(m.focus == focusRepos).
		Width(leftW).
		Height(repoContentH + 2).
		MaxHeight(repoContentH + 2).
		Render(m.renderReposSection(repoContentH, innerW))

	branchBox := sectionBoxForFocus(m.focus == focusBranches).
		Width(leftW).
		Height(branchContentH + 2).
		MaxHeight(branchContentH + 2).
		Render(m.renderBranchSection(branchContentH, innerW))

	fileBox := sectionBoxForFocus(m.focus == focusFiles).
		Width(leftW).
		Height(fileContentH + 2).
		MaxHeight(fileContentH + 2).
		Render(m.renderFilesSection(fileContentH, innerW))

	leftPanel := lipgloss.JoinVertical(lipgloss.Left, repoBox, branchBox, fileBox)

	rightBoxStyle := sectionBox
	if m.focus == focusDiff {
		rightBoxStyle = focusedBox
	}

	rightBox := rightBoxStyle.
		Width(rightW).
		Height(bodyH).
		Render(m.renderDiffPanel(bodyH - 2))

	combined := lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, " ", rightBox)

	var b strings.Builder
	b.WriteString(combined)
	b.WriteString("\n")
	b.WriteString(m.renderHelp())

	v := tea.NewView(b.String())
	v.AltScreen = true
	return v
}

func sectionBoxForFocus(focused bool) lipgloss.Style {
	if focused {
		return focusedBox
	}
	return sectionBox
}

// truncLine truncates an ANSI-styled string to fit within maxW display columns.
var truncStyle lipgloss.Style

func truncLine(s string, maxW int) string {
	if lipgloss.Width(s) <= maxW {
		return s
	}
	return truncStyle.MaxWidth(maxW).Render(s)
}

func (m model) renderReposSection(maxLines, maxW int) string {
	var lines []string
	lines = append(lines, dimStyle.Render("Repos"))
	contentLines := maxLines - 1 // -1 for header
	start, end := visibleRange(m.repoCursor, len(m.items), contentLines)
	for i := start; i < end; i++ {
		name := m.items[i]
		s := m.details[name]
		lines = append(lines, truncLine(m.renderRepoRow(name, s, i), maxW))
	}
	return strings.Join(lines, "\n")
}

func (m model) renderRepoRow(name string, s *git.RepoStatus, idx int) string {
	fi := " "
	if idx == m.repoCursor {
		fi = focusDotStyle.Render(">")
	}
	disp := m.displayName(name)
	if s == nil {
		return fi + disp
	}
	icon := dirtyStyle.Render("●")
	switch s.Worktree {
	case git.StatusClean:
		icon = cleanStyle.Render("✓")
	case git.StatusMissing:
		icon = errorStyle.Render("✗")
	}
	branch := dimStyle.Render(s.Branch)
	ab := ""
	if s.Ahead > 0 {
		ab += cleanStyle.Render(fmt.Sprintf("↑%d", s.Ahead))
	}
	if s.Behind > 0 {
		if ab != "" {
			ab += " "
		}
		ab += dirtyStyle.Render(fmt.Sprintf("↓%d", s.Behind))
	}
	return fmt.Sprintf("%s%s %s %s", fi, icon, disp, branch) + ab
}

func (m model) renderBranchSection(maxLines, maxW int) string {
	var lines []string
	lines = append(lines, dimStyle.Render("Branches"))
	s := m.details[m.selected]
	if s == nil {
		return strings.Join(lines, "\n")
	}
	status := m.formatStatus(s)
	info := fmt.Sprintf("%s %s", s.Branch, status)
	if s.Remote != "" {
		info += dimStyle.Render(fmt.Sprintf(" %s", s.Remote))
	}
	lines = append(lines, truncLine(info, maxW))
	if m.loadingDetail {
		return strings.Join(lines, "\n")
	}
	remaining := maxLines - 2 // -2 for header + status line
	start, end := visibleRange(m.branchCursor, len(m.branches), remaining)
	for i := start; i < end; i++ {
		br := m.branches[i]
		cursor := "  "
		if m.focus == focusBranches && i == m.branchCursor {
			cursor = focusDotStyle.Render(" >")
		}
		marker := "  "
		name := dimStyle.Render(br.Name)
		if br.Current {
			marker = cleanStyle.Render(" *")
			name = boldStyle.Render(br.Name)
		}
		upstream := ""
		if br.Remote != "" {
			upstream = dimStyle.Render(fmt.Sprintf(" → %s", br.Remote))
		}
		ab := ""
		if br.Ahead > 0 {
			ab += cleanStyle.Render(fmt.Sprintf(" ↑%d", br.Ahead))
		}
		if br.Behind > 0 {
			ab += dirtyStyle.Render(fmt.Sprintf(" ↓%d", br.Behind))
		}
		line := fmt.Sprintf("%s%s%s%s%s", cursor, marker, name, upstream, ab)
		lines = append(lines, truncLine(line, maxW))
	}
	return strings.Join(lines, "\n")
}

func (m model) renderFilesSection(maxLines, maxW int) string {
	var lines []string
	lines = append(lines, dimStyle.Render("Files"))
	if m.loadingDetail {
		return strings.Join(lines, "\n")
	}
	if len(m.fileTree) == 0 {
		lines = append(lines, cleanStyle.Render("  clean"))
		return strings.Join(lines, "\n")
	}
	start, end := visibleRange(m.fileCursor, len(m.fileTree), maxLines-1)
	for i := start; i < end; i++ {
		node := m.fileTree[i]
		cursor := "  "
		if m.focus == focusFiles && i == m.fileCursor {
			cursor = focusDotStyle.Render(" >")
		}
		indent := strings.Repeat("  ", node.Indent)
		var line string
		if node.IsDir {
			parts := strings.Split(node.Path, "/")
			line = fmt.Sprintf("%s%s%s", cursor, indent, dimStyle.Render(parts[len(parts)-1]+"/"))
		} else {
			parts := strings.Split(node.Path, "/")
			line = fmt.Sprintf("%s%s%s%s", cursor, indent, formatFileStatus(node.Status), parts[len(parts)-1])
		}
		lines = append(lines, truncLine(line, maxW))
	}
	return strings.Join(lines, "\n")
}

func (m model) renderDiffPanel(height int) string {
	if m.diffContent == nil {
		return dimStyle.Render("  Select a file to view diff")
	}
	if m.diffContent.Error != nil {
		return errorStyle.Render("  " + m.diffContent.Error.Error())
	}
	var out []string
	out = append(out, "  "+accentStyle.Render(m.diffContent.Path))
	lines := strings.Split(m.diffContent.Content, "\n")
	// Remove trailing empty line from split.
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	diffLines := height - 1 // -1 for header line
	if diffLines < 1 {
		return strings.Join(out, "\n")
	}
	maxOff := max(0, len(lines)-diffLines)
	scrollOff := min(m.diffScrollOff, maxOff)
	end := min(scrollOff+diffLines, len(lines))
	for _, line := range lines[scrollOff:end] {
		switch {
		case strings.HasPrefix(line, "diff --git") || strings.HasPrefix(line, "index "):
			out = append(out, "  "+dimStyle.Render(line))
		case strings.HasPrefix(line, "---") || strings.HasPrefix(line, "+++"):
			out = append(out, "  "+dimStyle.Render(line))
		case strings.HasPrefix(line, "@@"):
			out = append(out, "  "+accentStyle.Render(line))
		case strings.HasPrefix(line, "+"):
			out = append(out, "  "+cleanStyle.Render(line))
		case strings.HasPrefix(line, "-"):
			out = append(out, "  "+errorStyle.Render(line))
		default:
			out = append(out, "  "+line)
		}
	}
	return strings.Join(out, "\n")
}

func (m model) renderHelp() string {
	if m.confirmCheckout {
		return helpStyle.Render(fmt.Sprintf(" Checkout %s? [y/n]", m.checkoutBranch))
	}

	brand := brandStyle.Render("mrepo") + dimStyle.Render(fmt.Sprintf(" %s", m.version))

	var keys string
	switch m.focus {
	case focusRepos:
		keys = "[j/k] move [p]ull [f]etch [c]lone [S]ync [s]tatus"
	case focusBranches:
		keys = "[j/k] move [enter] checkout"
	case focusFiles:
		keys = "[j/k] move [enter] diff"
	case focusDiff:
		keys = "[j/k/pgup/pgdn] scroll"
	}

	help := helpStyle.Render(fmt.Sprintf(" %s  [tab] switch %s [q]", brand, keys))
	if m.statusText != "" {
		help += "  " + dimStyle.Render(m.statusText)
	}
	return help
}

// --- Format helpers ---

func (m model) formatStatus(s *git.RepoStatus) string {
	switch s.Worktree {
	case git.StatusClean:
		return cleanStyle.Render("✓")
	case git.StatusDirty:
		return dirtyStyle.Render("●")
	case git.StatusStaged:
		return stagedStyle.Render("◆")
	case git.StatusUntracked:
		return untrackedStyle.Render("●")
	case git.StatusMissing:
		return errorStyle.Render("✗")
	default:
		return dimStyle.Render("?")
	}
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// visibleRange returns the [start, end) indices for a viewport that keeps
// cursor visible within maxLines content lines.  Returns (0, total) if
// everything fits.
func visibleRange(cursor, total, maxLines int) (int, int) {
	if maxLines <= 0 || total <= 0 {
		return 0, 0
	}
	if total <= maxLines {
		return 0, total
	}
	// Keep cursor inside the viewport; scroll when it hits the edge.
	start := clamp(cursor-maxLines/2, 0, total-maxLines)
	return start, start + maxLines
}

func formatFileStatus(status string) string {
	switch status {
	case "M":
		return dirtyStyle.Render("M ")
	case "A":
		return cleanStyle.Render("A ")
	case "D":
		return errorStyle.Render("D ")
	case "?":
		return untrackedStyle.Render("? ")
	case "R":
		return stagedStyle.Render("R ")
	default:
		return dimStyle.Render(status + " ")
	}
}
