package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"sort"
	"strings"
	"time"

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
	// header
	fmt.Printf("  %-20s %-20s %-10s %s\n", "REPO", "BRANCH", "STATUS", "AHEAD/BEHIND")
	fmt.Println(strings.Repeat("-", 72))

	missingCount := 0
	for _, s := range statuses {
		if s.Worktree == git.StatusMissing {
			missingCount++
		}

		aheadBehind := fmt.Sprintf("↑%d ↓%d", s.Ahead, s.Behind)
		if s.Ahead == 0 && s.Behind == 0 {
			aheadBehind = "-"
		}

		statusStr := s.StatusString()
		var statusIcon string
		switch s.Worktree {
		case git.StatusClean:
			statusIcon = "\u25cb" // ○
		case git.StatusMissing:
			statusIcon = "!" // !
		default:
			statusIcon = "\u25cf" // ●
		}

		if s.Worktree == git.StatusMissing {
			fmt.Printf("  %s %-18s %-20s\n", statusIcon, s.Name, "MISSING")
			continue
		}

		if s.Error != nil {
			fmt.Printf("  %s %-18s %s\n", statusIcon, s.Name, s.Error)
			continue
		}

		fmt.Printf("  %s %-18s %-20s %-10s %s\n",
			statusIcon,
			s.Name,
			s.Branch,
			statusStr,
			aheadBehind,
		)
	}

	if missingCount > 0 {
		fmt.Printf("\n  %d repo(s) missing. Use 'mrepo sync' or 'mrepo clone' to download.\n", missingCount)
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
