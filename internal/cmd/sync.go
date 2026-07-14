package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

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
		parallel, _ := cmd.Flags().GetInt("parallel")
		timeout, _ := cmd.Flags().GetDuration("timeout")
		_, cfg, err := loadConfig(rootDir)
		if err != nil {
			return err
		}

		filtered := filterRepos(cfg)
		started := time.Now()

		// Partition into missing (clone) and existing (pull).
		toClone := make(map[string]git.CloneSpec)
		toPull := make(map[string]string)

		for name, repo := range filtered {
			absPath := filepath.Join(rootDir, repo.Path)
			if _, err := os.Stat(absPath); os.IsNotExist(err) {
				if repo.Remote != "" && repo.Path != "." {
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

		ctx, cancel := context.WithTimeout(cmd.Context(), syncTimeout)
		defer cancel()

		var allResults []syncRepoResult

		// Clone missing.
		if len(toClone) > 0 {
			cloneResults := git.CloneAll(ctx, rootDir, toClone, parallel)
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
			progress := newOperationProgress("Pulling", !jsonOutput)
			pullResults := git.PullAllWithOptions(ctx, rootDir, toPull, git.BatchOptions{
				Parallel:   parallel,
				Timeout:    timeout,
				OnProgress: progress.Update,
			})
			progress.Done()
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
		for name, repo := range filtered {
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
		failed := 0
		skipped := 0
		for _, r := range allResults {
			if r.Action == "skipped" {
				skipped++
			} else if r.Error != "" {
				failed++
			}
		}

		if jsonOutput {
			if err := printJSON(allResults); err != nil {
				return err
			}
			if failed > 0 {
				return fmt.Errorf("sync: %d repositories failed", failed)
			}
			return nil
		}

		rows := make([]operationRow, 0, len(allResults))
		succeeded := 0

		for _, r := range allResults {
			dn := displayRepoName(r.Name)
			action := r.Action
			if action == "skipped" {
				rows = append(rows, operationRow{Icon: warnIcon(), Name: dn, Action: action, Result: operationErrorSummary(fmt.Errorf("%s", r.Error)), ResultStyle: warnStyle})
			} else if r.Error != "" {
				rows = append(rows, operationRow{Icon: errorIcon(), Name: dn, Action: action, Result: operationErrorSummary(fmt.Errorf("%s", r.Error)), ResultStyle: errorStyle})
			} else {
				succeeded++
				rows = append(rows, operationRow{Icon: successIcon(), Name: dn, Action: action, Result: operationOutputSummary(r.Output, "ok"), ResultStyle: dimStyle})
			}
		}

		fmt.Println(renderOperationTable(rows))
		printOperationSummary("Sync complete", succeeded, failed, skipped, time.Since(started))
		if failed > 0 {
			return fmt.Errorf("sync: %d repositories failed", failed)
		}
		return nil
	},
}

func init() {
	syncCmd.Flags().BoolVar(&jsonOutput, "json", false, "output as JSON")
	syncCmd.Flags().Int("depth", 0, "shallow clone depth (0 = full)")
	syncCmd.Flags().Int("parallel", defaultNetworkParallel(), "maximum concurrent network operations")
	syncCmd.Flags().Duration("timeout", pullTimeout, "timeout for each pull operation")
	rootCmd.AddCommand(syncCmd)
}
