package tui

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/pinealctx/mrepo/internal/config"
	"github.com/pinealctx/mrepo/internal/git"
)

// --- Focus state ---

type focus int

const (
	focusRepos focus = iota
	focusBranches
	focusFiles
	focusDiff
)

// --- Data types ---

type fileTreeNode struct {
	Indent int
	Path   string
	IsDir  bool
	Status string
}

// --- Messages ---

type (
	statusMsg struct{ details map[string]*git.RepoStatus }
	detailMsg struct {
		branches []git.BranchInfo
		files    []git.DiffFile
		err      error
	}
	pullMsg           struct{ results map[string]string }
	cloneMsg          struct{ results map[string]string }
	syncMsg           struct{ results map[string]string }
	fileDiffMsg       struct{ diff *git.FileDiff }
	checkoutResultMsg struct{ err error }
)

// --- Model ---

type model struct {
	rootDir  string
	rootName string
	repos    map[string]string
	config   *config.Config
	items    []string

	details  map[string]*git.RepoStatus
	selected string

	focus focus

	// Detail panel data
	branches    []git.BranchInfo
	diffFiles   []git.DiffFile
	fileTree    []fileTreeNode
	diffContent *git.FileDiff

	// Cursors
	repoCursor    int
	branchCursor  int
	fileCursor    int
	diffScrollOff int

	// Loading
	loading       bool
	loadingDetail bool
	operating     bool // true while pull/clone/sync is in progress

	// Status bar message (shown after operations complete)
	statusText string

	width  int
	height int

	confirmCheckout bool
	checkoutBranch  string
	version         string
}

// --- Constructor ---

func NewModel(rootDir string, cfg *config.Config, filteredRepos map[string]*config.Repo, ver string) model {
	repos := make(map[string]string)
	for name, repo := range filteredRepos {
		repos[name] = repo.Path
	}

	// Build sorted item list from filteredRepos (not full config),
	// keeping root repo "." first when present.
	items := make([]string, 0, len(filteredRepos))
	for name := range filteredRepos {
		items = append(items, name)
	}
	sort.Strings(items)
	for i, n := range items {
		if n == "." {
			if i != 0 {
				items = append(items[:i], items[i+1:]...)
				items = append([]string{"."}, items...)
			}
			break
		}
	}

	absRoot, err := filepath.Abs(rootDir)
	rootName := filepath.Base(absRoot) + "/"
	if err != nil {
		rootName = filepath.Base(rootDir) + "/"
	}
	return model{
		rootDir:  rootDir,
		rootName: rootName,
		repos:    repos,
		config:   cfg,
		items:    items,
		details:  make(map[string]*git.RepoStatus),
		focus:    focusRepos,
		version:  ver,
	}
}

func (m *model) displayName(name string) string {
	if name == "." || name == "" || m.repos[name] == "." {
		if m.rootName == "./" || m.rootName == "" {
			return "<root>"
		}
		return m.rootName + " <root>"
	}
	return name
}

// --- Init ---

func (m model) Init() tea.Cmd { return refreshStatus(m.rootDir, m.repos) }

// --- Update ---

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyPressMsg:
		if msg.String() == "q" || msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
		if m.confirmCheckout {
			return m.updateConfirm(msg)
		}
		return m.updateKey(msg)

	case statusMsg:
		m.details = msg.details
		m.loading = false
		if m.selected == "" && len(m.items) > 0 {
			m.selected = m.items[0]
			m.loadingDetail = true
			return m, loadDetailForRepo(m.rootDir, m.repos[m.selected])
		}
		return m, nil

	case detailMsg:
		m.loadingDetail = false
		if msg.err != nil {
			m.branches = nil
			m.diffFiles = nil
			m.fileTree = nil
		} else {
			m.branches = msg.branches
			m.diffFiles = msg.files
			m.fileTree = buildFileTree(msg.files)
		}
		return m, nil

	case pullMsg:
		m.operating = false
		m.statusText = summarizeResults("pull", msg.results)
		return m, refreshStatus(m.rootDir, m.repos)

	case cloneMsg:
		m.operating = false
		m.statusText = summarizeResults("clone", msg.results)
		return m, refreshStatus(m.rootDir, m.repos)

	case syncMsg:
		m.operating = false
		m.statusText = summarizeResults("sync", msg.results)
		return m, refreshStatus(m.rootDir, m.repos)

	case fileDiffMsg:
		m.diffContent = msg.diff
		m.diffScrollOff = 0
		return m, nil

	case checkoutResultMsg:
		m.confirmCheckout = false
		return m, tea.Batch(refreshStatus(m.rootDir, m.repos), loadDetailForRepo(m.rootDir, m.repos[m.selected]))
	}

	return m, nil
}

func (m model) updateKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	switch key {
	case "s":
		m.loading = true
		return m, refreshStatus(m.rootDir, m.repos)
	case "p":
		if !m.operating {
			m.operating = true
			m.statusText = "pulling..."
			return m, pullAll(m.rootDir, m.repos)
		}
	case "f":
		m.loading = true
		return m, fetchAllRepos(m.rootDir, m.repos)
	case "c":
		if !m.operating {
			m.operating = true
			m.statusText = "cloning..."
			return m, cloneMissing(m.rootDir, m.config, m.repos)
		}
	case "S":
		if !m.operating {
			m.operating = true
			m.statusText = "syncing..."
			return m, syncAll(m.rootDir, m.config, m.repos)
		}
	}

	if key == "tab" {
		switch m.focus {
		case focusRepos:
			m.focus = focusBranches
		case focusBranches:
			m.focus = focusFiles
		case focusFiles:
			if m.diffContent != nil {
				m.focus = focusDiff
			} else {
				m.focus = focusRepos
			}
		case focusDiff:
			m.focus = focusRepos
		}
		return m, nil
	}

	switch m.focus {
	case focusRepos:
		return m.updateReposNav(key)
	case focusBranches:
		return m.updateBranchesNav(key)
	case focusFiles:
		return m.updateFilesNav(key)
	case focusDiff:
		return m.updateDiffNav(key)
	}
	return m, nil
}

// moveCursor adjusts a cursor position by key direction within [0, count).
// Returns the new position and whether it changed.
func moveCursor(cursor, count int, key string) (int, bool) {
	switch key {
	case "up", "k":
		if cursor > 0 {
			return cursor - 1, true
		}
	case "down", "j":
		if cursor < count-1 {
			return cursor + 1, true
		}
	}
	return cursor, false
}

func (m model) updateReposNav(key string) (tea.Model, tea.Cmd) {
	pos, moved := moveCursor(m.repoCursor, len(m.items), key)
	if moved {
		m.repoCursor = pos
		return m.switchRepo()
	}
	return m, nil
}

func (m model) updateBranchesNav(key string) (tea.Model, tea.Cmd) {
	m.branchCursor, _ = moveCursor(m.branchCursor, len(m.branches), key)
	if key == "enter" && len(m.branches) > 0 && m.branchCursor < len(m.branches) && !m.branches[m.branchCursor].Current {
		m.confirmCheckout = true
		m.checkoutBranch = m.branches[m.branchCursor].Name
	}
	return m, nil
}

func (m model) updateFilesNav(key string) (tea.Model, tea.Cmd) {
	m.fileCursor, _ = moveCursor(m.fileCursor, len(m.fileTree), key)
	if key == "enter" && m.fileCursor < len(m.fileTree) && !m.fileTree[m.fileCursor].IsDir {
		return m, loadFileDiffForRepo(m.rootDir, m.repos[m.selected], m.fileTree[m.fileCursor].Path, m.fileTree[m.fileCursor].Status == "?")
	}
	return m, nil
}

func (m model) updateDiffNav(key string) (tea.Model, tea.Cmd) {
	maxOff := m.maxDiffScroll()
	switch key {
	case "up", "k":
		if m.diffScrollOff > 0 {
			m.diffScrollOff--
		}
	case "down", "j":
		if m.diffScrollOff < maxOff {
			m.diffScrollOff++
		}
	case "pgdown":
		m.diffScrollOff = min(maxOff, m.diffScrollOff+max(5, m.height/3))
	case "pgup":
		m.diffScrollOff = max(0, m.diffScrollOff-max(5, m.height/3))
	}
	return m, nil
}

func (m model) maxDiffScroll() int {
	if m.diffContent == nil || m.diffContent.Content == "" {
		return 0
	}
	lines := strings.Split(m.diffContent.Content, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	bodyH := m.height - 1
	diffLines := bodyH - 2 - 1 // -2 for box border, -1 for header
	return max(0, len(lines)-diffLines)
}

func (m model) updateConfirm(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y":
		m.confirmCheckout = false
		return m, doCheckout(m.rootDir, m.repos[m.selected], m.checkoutBranch)
	case "n", "esc":
		m.confirmCheckout = false
	}
	return m, nil
}

func (m model) switchRepo() (tea.Model, tea.Cmd) {
	if m.repoCursor >= len(m.items) || m.items[m.repoCursor] == m.selected {
		return m, nil
	}
	m.selected = m.items[m.repoCursor]
	m.focus = focusRepos
	m.branches = nil
	m.diffFiles = nil
	m.fileTree = nil
	m.diffContent = nil
	m.branchCursor = 0
	m.fileCursor = 0
	m.diffScrollOff = 0
	m.loadingDetail = true
	return m, loadDetailForRepo(m.rootDir, m.repos[m.selected])
}

// --- Helpers ---

func summarizeResults(op string, results map[string]string) string {
	total := len(results)
	if total == 0 {
		return fmt.Sprintf("%s: nothing to do", op)
	}
	fails := 0
	for _, v := range results {
		if strings.HasPrefix(v, "FAIL") || strings.HasPrefix(v, "CLONE FAIL") || strings.HasPrefix(v, "PULL FAIL") {
			fails++
		}
	}
	if fails == 0 {
		return fmt.Sprintf("%s: %d ok", op, total)
	}
	return fmt.Sprintf("%s: %d ok, %d failed", op, total-fails, fails)
}

// --- Entry point ---

func Run(rootDir string, cfg *config.Config, filteredRepos map[string]*config.Repo, ver string) error {
	p := tea.NewProgram(NewModel(rootDir, cfg, filteredRepos, ver))
	_, err := p.Run()
	return err
}
