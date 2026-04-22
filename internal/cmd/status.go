package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"sort"
	"time"

	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/table"

	"github.com/pinealctx/mrepo/internal/config"
	"github.com/pinealctx/mrepo/internal/git"

	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show status of all repos",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfgPath, err := config.FindConfigFile(rootDir)
		if err != nil {
			return err
		}
		cfg, err := config.Load(cfgPath)
		if err != nil {
			return err
		}

		repos := make(map[string]string)
		for name, repo := range filterRepos(cfg) {
			repos[name] = repo.Path
		}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		statuses := git.GetStatuses(ctx, rootDir, repos, runtime.NumCPU())
		sort.Slice(statuses, func(i, j int) bool {
			return statuses[i].Name < statuses[j].Name
		})

		if jsonOutput {
			return printStatusJSON(statuses)
		}

		printStatusTable(statuses)
		return nil
	},
}

func printStatusTable(statuses []*git.RepoStatus) {
	t := table.New().
		Headers("REPO", "BRANCH", "STATUS", "AHEAD/BEHIND").
		Width(80).
		Border(lipgloss.NormalBorder()).
		BorderTop(false).BorderBottom(false).
		BorderLeft(false).BorderRight(false).
		BorderHeader(true).
		BorderColumn(false).
		BorderRow(false).
		StyleFunc(func(row, col int) lipgloss.Style {
			if row == table.HeaderRow {
				return boldStyle
			}
			return lipgloss.NewStyle()
		})

	missingCount := 0
	for _, s := range statuses {
		if s.Worktree == git.StatusMissing && !isRootRepo(s.Name) {
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

		if s.Worktree == git.StatusMissing && !isRootRepo(s.Name) {
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
	}

	fmt.Println(t.Render())

	if missingCount > 0 {
		fmt.Printf("\n  %s %s\n",
			warnStyle.Render(fmt.Sprintf("%d repo(s) missing.", missingCount)),
			dimStyle.Render("Use 'mrepo sync' or 'mrepo clone' to download."),
		)
	}
}

func printStatusJSON(statuses []*git.RepoStatus) error {
	type jsonStatus struct {
		Name   string `json:"name"`
		Path   string `json:"path"`
		Branch string `json:"branch,omitempty"`
		Remote string `json:"remote,omitempty"`
		Ahead  int    `json:"ahead"`
		Behind int    `json:"behind"`
		Status string `json:"status"`
		Clean  bool   `json:"clean"`
		Error  string `json:"error,omitempty"`
	}

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
		out[i] = js
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

func init() {
	statusCmd.Flags().BoolVar(&jsonOutput, "json", false, "output as JSON")
	rootCmd.AddCommand(statusCmd)
}
