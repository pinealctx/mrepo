package tui

import (
	"fmt"
	"strings"
	"unicode/utf8"

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

	// fitLines + padLines: fully manual width/height control.
	// No lipgloss Width/MaxWidth — we enforce exact visual width ourselves.
	cw := leftW - 2

	repoBox := sectionBoxForFocus(m.focus == focusRepos).
		Render(padLines(fitLines(m.renderReposSection(repoContentH), cw), repoContentH))

	branchBox := sectionBoxForFocus(m.focus == focusBranches).
		Render(padLines(fitLines(m.renderBranchSection(branchContentH), cw), branchContentH))

	fileBox := sectionBoxForFocus(m.focus == focusFiles).
		Render(padLines(fitLines(m.renderFilesSection(fileContentH), cw), fileContentH))

	leftPanel := lipgloss.JoinVertical(lipgloss.Left, repoBox, branchBox, fileBox)

	rightBoxStyle := sectionBox
	if m.focus == focusDiff {
		rightBoxStyle = focusedBox
	}

	rw := rightW - 2
	rightBox := rightBoxStyle.
		Render(padLines(fitLines(m.renderDiffPanel(bodyH-2), rw), bodyH-2))

	combined := lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, " ", rightBox)

	var b strings.Builder
	b.WriteString(combined)
	b.WriteString("\n")
	b.WriteString(m.renderHelp())

	v := tea.NewView(b.String())
	v.AltScreen = true
	return v
}

// fitLines ensures every line of s is exactly w visual characters:
// truncates lines that are too wide, pads shorter lines with spaces.
// ANSI escape codes are preserved and not counted as visual width.
func fitLines(s string, w int) string {
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		vl := visualLen(line)
		if vl > w {
			lines[i] = truncateANSI(line, w)
		} else if vl < w {
			lines[i] = line + strings.Repeat(" ", w-vl)
		}
	}
	return strings.Join(lines, "\n")
}

// visualLen returns the visual width of s, ignoring ANSI escape sequences.
func visualLen(s string) int {
	w := 0
	for i := 0; i < len(s); {
		if s[i] == '\x1b' {
			for i < len(s) && s[i] != 'm' {
				i++
			}
			if i < len(s) {
				i++
			}
			continue
		}
		_, size := utf8.DecodeRuneInString(s[i:])
		w++
		i += size
	}
	return w
}

// truncateANSI truncates s to maxW visual characters, preserving ANSI codes.
func truncateANSI(s string, maxW int) string {
	var b strings.Builder
	w := 0
	for i := 0; i < len(s) && w < maxW; {
		if s[i] == '\x1b' {
			j := i
			for j < len(s) && s[j] != 'm' {
				j++
			}
			if j < len(s) {
				j++
			}
			b.WriteString(s[i:j])
			i = j
			continue
		}
		_, size := utf8.DecodeRuneInString(s[i:])
		b.WriteString(s[i : i+size])
		w++
		i += size
	}
	return b.String()
}

// padLines ensures s has exactly n lines: trims trailing newlines,
// truncates overflow, pads with empty lines if short.
func padLines(s string, n int) string {
	s = strings.TrimRight(s, "\n")
	if n <= 0 {
		return ""
	}
	if s == "" {
		if n == 1 {
			return " "
		}
		return " " + strings.Repeat("\n ", n-1)
	}
	lines := strings.Split(s, "\n")
	if len(lines) > n {
		return strings.Join(lines[:n], "\n")
	}
	for len(lines) < n {
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n")
}

func sectionBoxForFocus(focused bool) lipgloss.Style {
	if focused {
		return focusedBox
	}
	return sectionBox
}

func (m model) renderReposSection(maxLines int) string {
	var b strings.Builder
	b.WriteString(dimStyle.Render("Repos"))
	b.WriteString("\n")
	for i, name := range m.items {
		if i >= maxLines-1 {
			break
		}
		s := m.details[name]
		b.WriteString(m.renderRepoRow(name, s, i))
		b.WriteString("\n")
	}
	return b.String()
}

func (m model) renderRepoRow(name string, s *git.RepoStatus, idx int) string {
	fi := " "
	if idx == m.repoCursor {
		fi = focusDotStyle.Render("›")
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

func (m model) renderBranchSection(maxLines int) string {
	var b strings.Builder
	b.WriteString(dimStyle.Render("Branches"))
	b.WriteString("\n")
	s := m.details[m.selected]
	if s == nil {
		return b.String()
	}
	status := m.formatStatus(s)
	info := fmt.Sprintf("%s %s", s.Branch, status)
	if s.Remote != "" {
		info += dimStyle.Render(fmt.Sprintf(" %s", s.Remote))
	}
	b.WriteString(info)
	b.WriteString("\n")
	if m.loadingDetail {
		return b.String()
	}
	remaining := maxLines - 2 // -2 for header + status line
	for i := 0; i < remaining && i < len(m.branches); i++ {
		br := m.branches[i]
		cursor := "  "
		if m.focus == focusBranches && i == m.branchCursor {
			cursor = focusDotStyle.Render(" ›")
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
		fmt.Fprintf(&b, "%s%s%s%s%s\n", cursor, marker, name, upstream, ab)
	}
	return b.String()
}

func (m model) renderFilesSection(maxLines int) string {
	var b strings.Builder
	b.WriteString(dimStyle.Render("Files"))
	b.WriteString("\n")
	if m.loadingDetail {
		return b.String()
	}
	if len(m.fileTree) == 0 {
		b.WriteString(cleanStyle.Render("  clean"))
		return b.String()
	}
	for i := 0; i < maxLines-1 && i < len(m.fileTree); i++ {
		node := m.fileTree[i]
		cursor := "  "
		if m.focus == focusFiles && i == m.fileCursor {
			cursor = focusDotStyle.Render(" ›")
		}
		indent := strings.Repeat("  ", node.Indent)
		if node.IsDir {
			parts := strings.Split(node.Path, "/")
			fmt.Fprintf(&b, "%s%s%s\n", cursor, indent, dimStyle.Render(parts[len(parts)-1]+"/"))
		} else {
			parts := strings.Split(node.Path, "/")
			fmt.Fprintf(&b, "%s%s%s%s\n", cursor, indent, formatFileStatus(node.Status), parts[len(parts)-1])
		}
	}
	return b.String()
}

func (m model) renderDiffPanel(height int) string {
	if m.diffContent == nil {
		return dimStyle.Render("  Select a file to view diff")
	}
	if m.diffContent.Error != nil {
		return errorStyle.Render("  " + m.diffContent.Error.Error())
	}
	var b strings.Builder
	fmt.Fprintf(&b, "  %s\n", accentStyle.Render(m.diffContent.Path))
	lines := strings.Split(m.diffContent.Content, "\n")
	// Remove trailing empty line from split.
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	diffLines := height - 1 // -1 for header line
	if diffLines < 1 {
		return b.String()
	}
	maxOff := max(0, len(lines)-diffLines)
	m.diffScrollOff = min(m.diffScrollOff, maxOff)
	end := min(m.diffScrollOff+diffLines, len(lines))
	for _, line := range lines[m.diffScrollOff:end] {
		switch {
		case strings.HasPrefix(line, "diff --git") || strings.HasPrefix(line, "index "):
			fmt.Fprintf(&b, "  %s\n", dimStyle.Render(line))
		case strings.HasPrefix(line, "---") || strings.HasPrefix(line, "+++"):
			fmt.Fprintf(&b, "  %s\n", dimStyle.Render(line))
		case strings.HasPrefix(line, "@@"):
			fmt.Fprintf(&b, "  %s\n", accentStyle.Render(line))
		case strings.HasPrefix(line, "+"):
			fmt.Fprintf(&b, "  %s\n", cleanStyle.Render(line))
		case strings.HasPrefix(line, "-"):
			fmt.Fprintf(&b, "  %s\n", errorStyle.Render(line))
		default:
			fmt.Fprintf(&b, "  %s\n", line)
		}
	}
	return b.String()
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

	return helpStyle.Render(fmt.Sprintf(" %s  [tab] switch %s [q]", brand, keys))
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
