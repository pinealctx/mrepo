package cmd

import (
	"context"
	"fmt"
	"runtime"
	"sort"

	"github.com/pinealctx/mrepo/internal/git"

	"github.com/spf13/cobra"
)

var showBranches bool

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show status of all repos",
	RunE: func(cmd *cobra.Command, args []string) error {
		_, cfg, err := loadConfig(rootDir)
		if err != nil {
			return err
		}

		repos := make(map[string]string)
		for name, repo := range filterRepos(cfg) {
			repos[name] = repo.Path
		}

		ctx, cancel := context.WithTimeout(context.Background(), statusTimeout)
		defer cancel()

		statuses := git.GetStatuses(ctx, rootDir, repos, runtime.NumCPU())
		sort.Slice(statuses, func(i, j int) bool {
			return statuses[i].Name < statuses[j].Name
		})

		// Optionally fetch branch info.
		var branchesMap map[string][]git.BranchInfo
		if showBranches {
			branchesMap = loadBranchesMap(ctx, statuses)
		}

		if jsonOutput {
			return printJSON(toStatusJSON(statuses, branchesMap))
		}

		printStatusTable(statuses, branchesMap)
		return nil
	},
}

func loadBranchesMap(ctx context.Context, statuses []*git.RepoStatus) map[string][]git.BranchInfo {
	m := make(map[string][]git.BranchInfo, len(statuses))
	for _, s := range statuses {
		if s.Worktree == git.StatusMissing || s.Error != nil {
			continue
		}
		// s.Path is already an absolute path from GetStatuses.
		branches, err := git.GetBranches(ctx, s.Path)
		if err != nil {
			continue
		}
		m[s.Name] = branches
	}
	return m
}

type jsonStatusBranch struct {
	Name    string `json:"name"`
	Current bool   `json:"current"`
	Remote  string `json:"remote,omitempty"`
	Ahead   int    `json:"ahead"`
	Behind  int    `json:"behind"`
}

type jsonStatus struct {
	Name     string             `json:"name"`
	Path     string             `json:"path"`
	Branch   string             `json:"branch,omitempty"`
	Remote   string             `json:"remote,omitempty"`
	Ahead    int                `json:"ahead"`
	Behind   int                `json:"behind"`
	Status   string             `json:"status"`
	Clean    bool               `json:"clean"`
	Error    string             `json:"error,omitempty"`
	Branches []jsonStatusBranch `json:"branches,omitempty"`
}

func toStatusJSON(statuses []*git.RepoStatus, branchesMap map[string][]git.BranchInfo) []jsonStatus {
	out := make([]jsonStatus, len(statuses))
	for i, s := range statuses {
		js := jsonStatus{
			Name:   s.Name,
			Path:   s.Path,
			Branch: s.Branch,
			Remote: s.Remote,
			Ahead:  s.Ahead,
			Behind: s.Behind,
			Status: s.StatusString(),
			Clean:  s.IsClean(),
		}
		if s.Error != nil {
			js.Error = s.Error.Error()
		}
		if branches, ok := branchesMap[s.Name]; ok {
			js.Branches = make([]jsonStatusBranch, len(branches))
			for j, b := range branches {
				js.Branches[j] = jsonStatusBranch{
					Name:    b.Name,
					Current: b.Current,
					Remote:  b.Remote,
					Ahead:   b.Ahead,
					Behind:  b.Behind,
				}
			}
		}
		out[i] = js
	}
	return out
}

func printStatusTable(statuses []*git.RepoStatus, branchesMap map[string][]git.BranchInfo) {
	t := newHeaderTable("REPO", "BRANCH", "STATUS", "AHEAD/BEHIND")

	missingCount := 0
	for _, s := range statuses {
		if s.Worktree == git.StatusMissing {
			missingCount++
		}

		displayName := displayRepoName(s.Name)
		nameStyle := accentStyle
		if isRootRepo(s.Name) {
			nameStyle = rootStyle
		}

		icon := cleanStyle.Render("✓")
		if s.Worktree == git.StatusMissing {
			icon = warnStyle.Render("⚠")
		} else if s.Worktree != git.StatusClean {
			icon = dirtyStyle.Render("●")
		}

		nameStr := icon + " " + nameStyle.Render(displayName)

		if s.Worktree == git.StatusMissing {
			t.Row(nameStr, "", missingStyle.Render("MISSING"), "")
			continue
		}

		if s.Error != nil {
			t.Row(nameStr, "", errorStyle.Render(s.Error.Error()), "")
			continue
		}

		aheadBehind := dimStyle.Render("-")
		if s.Ahead > 0 || s.Behind > 0 {
			if s.Ahead > 0 && s.Behind > 0 {
				aheadBehind = fmt.Sprintf("%s %s",
					successStyle.Render(fmt.Sprintf("↑%d", s.Ahead)),
					warnStyle.Render(fmt.Sprintf("↓%d", s.Behind)))
			} else if s.Ahead > 0 {
				aheadBehind = successStyle.Render(fmt.Sprintf("↑%d", s.Ahead))
			} else {
				aheadBehind = warnStyle.Render(fmt.Sprintf("↓%d", s.Behind))
			}
		}

		t.Row(nameStr, dimStyle.Render(s.Branch), formatStatus(s.StatusString()), aheadBehind)

		// Show branches if --branches flag is set.
		if branches, ok := branchesMap[s.Name]; ok {
			for _, b := range branches {
				if b.Current && b.Name == s.Branch {
					continue // skip the already-displayed current branch
				}
				marker := "  "
				branchName := dimStyle.Render(b.Name)
				ab := dimStyle.Render("-")
				if b.Ahead > 0 || b.Behind > 0 {
					ab = fmt.Sprintf("%s %s",
						successStyle.Render(fmt.Sprintf("↑%d", b.Ahead)),
						warnStyle.Render(fmt.Sprintf("↓%d", b.Behind)))
				}
				t.Row("", branchName, marker, ab)
			}
		}
	}

	fmt.Println(t.Render())

	if missingCount > 0 {
		fmt.Printf("\n  %s %s\n",
			warnStyle.Render(fmt.Sprintf("%d repo(s) missing.", missingCount)),
			dimStyle.Render("Use 'mrepo sync' or 'mrepo clone' to download."),
		)
	}
}

func init() {
	statusCmd.Flags().BoolVar(&jsonOutput, "json", false, "output as JSON")
	statusCmd.Flags().BoolVar(&showBranches, "branches", false, "show all local branches per repo")
	rootCmd.AddCommand(statusCmd)
}
