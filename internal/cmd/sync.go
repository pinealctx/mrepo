package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"time"

	"github.com/pinealctx/mrepo/internal/config"
	"github.com/pinealctx/mrepo/internal/git"

	"github.com/spf13/cobra"
)

type syncRepoResult struct {
	Name   string `json:"name"`
	Action string `json:"action"` // "cloned", "pulled", "skipped"
	Output string `json:"output,omitempty"`
	Error  string `json:"error,omitempty"`
}

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Clone missing repos and pull existing ones",
	RunE: func(cmd *cobra.Command, args []string) error {
		depth, _ := cmd.Flags().GetInt("depth")
		cfgPath, err := config.FindConfigFile(rootDir)
		if err != nil {
			return err
		}
		cfg, err := config.Load(cfgPath)
		if err != nil {
			return err
		}

		// Partition into missing (clone) and existing (pull).
		toClone := make(map[string]git.CloneSpec)
		toPull := make(map[string]string)

		for name, repo := range filterRepos(cfg) {
			absPath := filepath.Join(rootDir, repo.Path)
			if _, err := os.Stat(absPath); os.IsNotExist(err) {
				if repo.Remote != "" {
					toClone[name] = git.CloneSpec{
						Path:   repo.Path,
						Remote: repo.Remote,
						Branch: repo.Branch,
						Depth:  depth,
					}
				}
			} else {
				toPull[name] = repo.Path
			}
		}

		ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
		defer cancel()

		var allResults []syncRepoResult

		// Clone missing.
		if len(toClone) > 0 {
			cloneResults := git.CloneAll(ctx, rootDir, toClone, runtime.NumCPU())
			for _, r := range cloneResults {
				sr := syncRepoResult{
					Name:   r.Name,
					Action: "cloned",
					Output: r.Output,
				}
				if r.Error != nil {
					sr.Error = r.Error.Error()
				}
				allResults = append(allResults, sr)
			}
		}

		// Pull existing.
		if len(toPull) > 0 {
			pullResults := git.PullAll(ctx, rootDir, toPull, runtime.NumCPU())
			for _, r := range pullResults {
				sr := syncRepoResult{
					Name:   r.Name,
					Action: "pulled",
					Output: r.Output,
				}
				if r.Error != nil {
					sr.Error = r.Error.Error()
				}
				allResults = append(allResults, sr)
			}
		}

		// Repos with no remote and missing on disk.
		for name, repo := range filterRepos(cfg) {
			absPath := filepath.Join(rootDir, repo.Path)
			if _, err := os.Stat(absPath); os.IsNotExist(err) && repo.Remote == "" {
				allResults = append(allResults, syncRepoResult{
					Name:   name,
					Action: "skipped",
					Error:  "no remote configured",
				})
			}
		}

		sort.Slice(allResults, func(i, j int) bool {
			return allResults[i].Name < allResults[j].Name
		})

		if jsonOutput {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(allResults)
		}

		for _, r := range allResults {
			dn := displayRepoName(r.Name)
			if r.Error != "" {
				fmt.Printf("  %s %-20s %s %s\n", errorIcon(), dn, dimStyle.Render(fmt.Sprintf("[%s]", r.Action)), errorStyle.Render(truncate(r.Error, 60)))
			} else {
				output := truncate(r.Output, 60)
				if output == "" {
					output = "ok"
				}
				fmt.Printf("  %s %-20s %s %s\n", successIcon(), dn, dimStyle.Render(fmt.Sprintf("[%s]", r.Action)), dimStyle.Render(output))
			}
		}
		return nil
	},
}

func init() {
	syncCmd.Flags().BoolVar(&jsonOutput, "json", false, "output as JSON")
	syncCmd.Flags().Int("depth", 0, "shallow clone depth (0 = full)")
	rootCmd.AddCommand(syncCmd)
}
