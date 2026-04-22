package tui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"charm.land/bubbles/v2/table"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/pinealctx/mrepo/internal/config"
	"github.com/pinealctx/mrepo/internal/git"
)

type tab int

const (
	tabStatus tab = iota
	tabLog
)

type repoDetail struct {
	Status *git.RepoStatus
	Log    string
}

type model struct {
	rootDir  string
	rootName string
	repos    map[string]string
	config   *config.Config

	tbl   table.Model
	items []string // repo names in table order (maps cursor → name)

	details  map[string]*repoDetail
	selected string
	tab      tab

	loading bool
	err     error
	width   int
	height  int

	pulling     bool
	pullResults map[string]string

	cloning      bool
	cloneResults map[string]string
}

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#7C3AED")).
			MarginBottom(1)

	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#9CA3AF"))

	cleanStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#22C55E"))

	dirtyStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#F59E0B"))

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#EF4444"))

	missingStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#EF4444"))

	stagedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#3B82F6"))

	dimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6B7280"))

	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6B7280")).
			MarginTop(1)

	borderStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#374151")).
			Padding(0, 1)
)

type statusMsg struct {
	details map[string]*repoDetail
}

type pullMsg struct {
	results map[string]string
}

type cloneMsg struct {
	results map[string]string
}

type syncMsg struct {
	results map[string]string
}

func refreshStatus(rootDir string, repos map[string]string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		statuses := git.GetStatuses(ctx, rootDir, repos, runtime.NumCPU())
		details := make(map[string]*repoDetail, len(statuses))
		for _, s := range statuses {
			details[s.Name] = &repoDetail{Status: s}
		}
		return statusMsg{details: details}
	}
}

func pullAll(rootDir string, repos map[string]string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		existing := make(map[string]string)
		for name, path := range repos {
			if !isMissing(rootDir, path) {
				existing[name] = path
			}
		}

		results := git.PullAll(ctx, rootDir, existing, runtime.NumCPU())
		out := make(map[string]string, len(results))
		for _, r := range results {
			if r.Error != nil {
				out[r.Name] = fmt.Sprintf("FAIL: %s", r.Error)
			} else {
				out[r.Name] = r.Output
			}
		}
		return pullMsg{results: out}
	}
}

func fetchAll(rootDir string, repos map[string]string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		existing := make(map[string]string)
		for name, path := range repos {
			if !isMissing(rootDir, path) {
				existing[name] = path
			}
		}

		_ = git.FetchAll(ctx, rootDir, existing, runtime.NumCPU())
		statuses := git.GetStatuses(ctx, rootDir, repos, runtime.NumCPU())
		details := make(map[string]*repoDetail, len(statuses))
		for _, s := range statuses {
			details[s.Name] = &repoDetail{Status: s}
		}
		return statusMsg{details: details}
	}
}

func cloneMissing(rootDir string, cfg *config.Config, repos map[string]string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
		defer cancel()

		specs := make(map[string]git.CloneSpec)
		for name, repo := range cfg.Repos {
			if repo.Remote == "" {
				continue
			}
			if !isMissing(rootDir, repo.Path) {
				continue
			}
			specs[name] = git.CloneSpec{
				Path:   repo.Path,
				Remote: repo.Remote,
				Branch: repo.Branch,
			}
		}

		if len(specs) == 0 {
			return cloneMsg{results: map[string]string{}}
		}

		results := git.CloneAll(ctx, rootDir, specs, runtime.NumCPU())
		out := make(map[string]string, len(results))
		for _, r := range results {
			if r.Error != nil {
				out[r.Name] = fmt.Sprintf("FAIL: %s", r.Error)
			} else {
				out[r.Name] = r.Output
			}
		}
		return cloneMsg{results: out}
	}
}

func syncAll(rootDir string, cfg *config.Config, repos map[string]string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
		defer cancel()

		out := make(map[string]string)

		cloneSpecs := make(map[string]git.CloneSpec)
		for name, repo := range cfg.Repos {
			if repo.Remote != "" && isMissing(rootDir, repo.Path) {
				cloneSpecs[name] = git.CloneSpec{
					Path:   repo.Path,
					Remote: repo.Remote,
					Branch: repo.Branch,
				}
			}
		}
		if len(cloneSpecs) > 0 {
			cloneResults := git.CloneAll(ctx, rootDir, cloneSpecs, runtime.NumCPU())
			for _, r := range cloneResults {
				if r.Error != nil {
					out[r.Name] = fmt.Sprintf("CLONE FAIL: %s", r.Error)
				} else {
					out[r.Name] = "cloned"
				}
			}
		}

		existing := make(map[string]string)
		for name, path := range repos {
			if !isMissing(rootDir, path) {
				existing[name] = path
			}
		}
		if len(existing) > 0 {
			pullResults := git.PullAll(ctx, rootDir, existing, runtime.NumCPU())
			for _, r := range pullResults {
				if r.Error != nil {
					out[r.Name] = fmt.Sprintf("PULL FAIL: %s", r.Error)
				} else if _, has := out[r.Name]; !has {
					out[r.Name] = "pulled"
				}
			}
		}

		return syncMsg{results: out}
	}
}

func loadLogForRepo(rootDir, relPath string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		absPath := filepath.Join(rootDir, relPath)
		log, err := git.Log(ctx, absPath, 15)
		msg := logMsg{name: filepath.Base(relPath)}
		if err != nil {
			msg.err = err.Error()
		} else {
			msg.log = log
		}
		return msg
	}
}

type logMsg struct {
	name string
	log  string
	err  string
}

func isMissing(rootDir, relPath string) bool {
	absPath := filepath.Join(rootDir, relPath)
	_, err := os.Stat(absPath)
	return os.IsNotExist(err)
}

func NewModel(rootDir string, cfg *config.Config) model {
	repos := make(map[string]string)
	for name, repo := range cfg.Repos {
		repos[name] = repo.Path
	}

	names := cfg.SortedRepoNames()
	rootDisplayName := filepath.Base(rootDir) + "/"

	columns := []table.Column{
		{Title: "REPO", Width: 22},
		{Title: "BRANCH", Width: 18},
		{Title: "STATUS", Width: 10},
		{Title: "AHEAD/BEHIND", Width: 14},
	}

	tbl := table.New(
		table.WithColumns(columns),
		table.WithFocused(true),
		table.WithHeight(len(names)),
	)

	tblStyles := table.DefaultStyles()
	tblStyles.Header = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#9CA3AF")).Padding(0)
	tblStyles.Selected = lipgloss.NewStyle().Background(lipgloss.Color("#2D1B69")).Padding(0)
	tblStyles.Cell = lipgloss.NewStyle().Padding(0)
	tbl.SetStyles(tblStyles)

	return model{
		rootDir:      rootDir,
		rootName:     rootDisplayName,
		repos:        repos,
		config:       cfg,
		tbl:          tbl,
		items:        names,
		details:      make(map[string]*repoDetail),
		pullResults:  make(map[string]string),
		cloneResults: make(map[string]string),
	}
}

func (m *model) displayName(name string) string {
	if name == "." {
		return m.rootName
	}
	return name
}

func (m model) Init() tea.Cmd {
	return refreshStatus(m.rootDir, m.repos)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.tbl.SetWidth(msg.Width)
		m.tbl.SetHeight(msg.Height - 4)
		return m, nil

	case tea.KeyPressMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "up", "k", "down", "j":
			if m.selected == "" {
				var cmd tea.Cmd
				m.tbl, cmd = m.tbl.Update(msg)
				return m, cmd
			}
		case "s":
			m.loading = true
			return m, refreshStatus(m.rootDir, m.repos)
		case "p":
			if !m.pulling && !m.cloning {
				m.pulling = true
				m.pullResults = make(map[string]string)
				return m, pullAll(m.rootDir, m.repos)
			}
		case "f":
			m.loading = true
			return m, fetchAll(m.rootDir, m.repos)
		case "c":
			if !m.cloning && !m.pulling {
				m.cloning = true
				m.cloneResults = make(map[string]string)
				return m, cloneMissing(m.rootDir, m.config, m.repos)
			}
		case "S":
			if !m.cloning && !m.pulling {
				m.cloning = true
				m.cloneResults = make(map[string]string)
				return m, syncAll(m.rootDir, m.config, m.repos)
			}
		case "enter":
			if m.selected == "" {
				cursor := m.tbl.Cursor()
				if cursor < len(m.items) {
					m.selected = m.items[cursor]
					m.tab = tabLog
					return m, loadLogForRepo(m.rootDir, m.repos[m.selected])
				}
			}
		case "esc":
			m.selected = ""
			m.tab = tabStatus
		}

	case statusMsg:
		m.details = msg.details
		m.loading = false
		m.buildTableRows()
		return m, nil

	case pullMsg:
		m.pulling = false
		m.pullResults = msg.results
		return m, refreshStatus(m.rootDir, m.repos)

	case cloneMsg:
		m.cloning = false
		m.cloneResults = msg.results
		return m, refreshStatus(m.rootDir, m.repos)

	case syncMsg:
		m.cloning = false
		m.cloneResults = msg.results
		return m, refreshStatus(m.rootDir, m.repos)

	case logMsg:
		if d, ok := m.details[msg.name]; ok {
			if msg.err != "" {
				d.Log = "Error: " + msg.err
			} else {
				d.Log = msg.log
			}
		}
		return m, nil
	}

	return m, nil
}

func (m *model) buildTableRows() {
	rows := make([]table.Row, 0, len(m.items))
	for _, name := range m.items {
		detail, ok := m.details[name]
		if !ok || detail.Status == nil {
			rows = append(rows, table.Row{m.displayName(name), "", dimStyle.Render("loading..."), ""})
			continue
		}

		s := detail.Status

		icon := cleanStyle.Render("○")
		if s.Worktree == git.StatusMissing {
			icon = errorStyle.Render("●")
		} else if s.Worktree != git.StatusClean {
			icon = dirtyStyle.Render("●")
		}

		nameStr := icon + " " + m.displayName(name)

		if s.Worktree == git.StatusMissing {
			rows = append(rows, table.Row{nameStr, "", missingStyle.Render("MISSING"), ""})
			continue
		}

		if s.Error != nil {
			rows = append(rows, table.Row{nameStr, "", errorStyle.Render(s.Error.Error()), ""})
			continue
		}

		var statusText string
		switch s.Worktree {
		case git.StatusClean:
			statusText = cleanStyle.Render("clean")
		case git.StatusDirty:
			statusText = dirtyStyle.Render("dirty")
		case git.StatusStaged:
			statusText = stagedStyle.Render("staged")
		default:
			statusText = dirtyStyle.Render(s.StatusString())
		}

		aheadBehind := dimStyle.Render("-")
		if s.Ahead > 0 || s.Behind > 0 {
			if s.Ahead > 0 && s.Behind > 0 {
				aheadBehind = fmt.Sprintf("%s %s",
					cleanStyle.Render(fmt.Sprintf("↑%d", s.Ahead)),
					dirtyStyle.Render(fmt.Sprintf("↓%d", s.Behind)))
			} else if s.Ahead > 0 {
				aheadBehind = cleanStyle.Render(fmt.Sprintf("↑%d", s.Ahead))
			} else {
				aheadBehind = dirtyStyle.Render(fmt.Sprintf("↓%d", s.Behind))
			}
		}

		rows = append(rows, table.Row{nameStr, dimStyle.Render(s.Branch), statusText, aheadBehind})
	}
	m.tbl.SetRows(rows)
}

func (m model) View() tea.View {
	if m.width == 0 {
		v := tea.NewView("Loading...")
		v.AltScreen = true
		return v
	}

	var b strings.Builder

	title := titleStyle.Render("mrepo - Monorepo Manager")
	b.WriteString(title)
	b.WriteString("\n")

	if m.selected != "" {
		b.WriteString(m.renderDetail())
	} else {
		b.WriteString(m.tbl.View())
		b.WriteString("\n")

		if m.pulling {
			b.WriteString(dirtyStyle.Render("  Pulling..."))
			b.WriteString("\n")
		}
		if m.cloning {
			b.WriteString(dirtyStyle.Render("  Syncing..."))
			b.WriteString("\n")
		}
	}

	var help string
	if m.selected != "" {
		help = helpStyle.Render("[esc] back  [q] quit")
	} else {
		help = helpStyle.Render("[s] refresh  [p] pull  [f] fetch  [c] clone  [S] sync  [enter] detail  [q] quit")
	}
	b.WriteString(help)

	v := tea.NewView(b.String())
	v.AltScreen = true
	return v
}

func (m *model) renderDetail() string {
	var b strings.Builder

	detail, ok := m.details[m.selected]
	if !ok || detail.Status == nil {
		return "Loading..."
	}

	s := detail.Status

	box := borderStyle.Render(fmt.Sprintf("  %s (%s)", m.displayName(m.selected), s.Path))
	b.WriteString(box)
	b.WriteString("\n\n")

	if s.Worktree == git.StatusMissing {
		b.WriteString(missingStyle.Render("  MISSING — not cloned yet"))
		b.WriteString("\n")
		return b.String()
	}

	fmt.Fprintf(&b, "  Branch:   %s\n", s.Branch)
	fmt.Fprintf(&b, "  Remote:   %s\n", s.Remote)
	fmt.Fprintf(&b, "  Status:   %s\n", s.StatusString())
	fmt.Fprintf(&b, "  Ahead:    %d  Behind: %d\n", s.Ahead, s.Behind)

	if detail.Log != "" {
		b.WriteString("\n")
		b.WriteString(headerStyle.Render("  Recent commits:"))
		b.WriteString("\n")
		for line := range strings.SplitSeq(detail.Log, "\n") {
			b.WriteString(dimStyle.Render("  " + line))
			b.WriteString("\n")
		}
	}

	return b.String()
}

func Run(rootDir string, cfg *config.Config) error {
	p := tea.NewProgram(NewModel(rootDir, cfg))
	_, err := p.Run()
	return err
}
