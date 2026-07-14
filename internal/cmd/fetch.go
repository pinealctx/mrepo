package cmd

import (
	"fmt"
	"sort"
	"time"

	"github.com/pinealctx/mrepo/internal/git"

	"github.com/spf13/cobra"
)

type jsonFetch struct {
	Name    string `json:"name"`
	Path    string `json:"path,omitempty"`
	Output  string `json:"output,omitempty"`
	Error   string `json:"error,omitempty"`
	Skipped bool   `json:"skipped,omitempty"`
}

var fetchCmd = &cobra.Command{
	Use:   "fetch",
	Short: "Fetch latest refs for all repos",
	RunE: func(cmd *cobra.Command, args []string) error {
		_, cfg, err := loadConfig(rootDir)
		if err != nil {
			return err
		}

		existing, missing := partitionRepos(filterRepos(cfg))
		parallel, _ := cmd.Flags().GetInt("parallel")
		timeout, _ := cmd.Flags().GetDuration("timeout")

		started := time.Now()
		progress := newOperationProgress("Fetching", !jsonOutput)
		results := git.FetchAllWithOptions(cmd.Context(), rootDir, existing, git.BatchOptions{
			Parallel:   parallel,
			Timeout:    timeout,
			OnProgress: progress.Update,
		})
		progress.Done()
		sort.Slice(results, func(i, j int) bool {
			return results[i].Name < results[j].Name
		})
		missingNames := make([]string, 0, len(missing))
		for name := range missing {
			missingNames = append(missingNames, name)
		}
		sort.Strings(missingNames)

		failed := 0
		for _, r := range results {
			if r.Error != nil {
				failed++
			}
		}

		if jsonOutput {
			out := make([]jsonFetch, 0, len(results)+len(missing))
			for _, r := range results {
				jf := jsonFetch{Name: r.Name, Path: r.Path, Output: r.Output}
				if r.Error != nil {
					jf.Error = r.Error.Error()
				}
				out = append(out, jf)
			}
			for _, name := range missingNames {
				out = append(out, jsonFetch{Name: name, Skipped: true})
			}
			if err := printJSON(out); err != nil {
				return err
			}
			if failed > 0 {
				return fmt.Errorf("fetch: %d repositories failed", failed)
			}
			return nil
		}

		rows := make([]operationRow, 0, len(results)+len(missing))
		succeeded := 0

		for _, r := range results {
			dn := displayRepoName(r.Name)
			if r.Error != nil {
				rows = append(rows, operationRow{Icon: errorIcon(), Name: dn, Result: operationErrorSummary(r.Error), ResultStyle: errorStyle})
			} else {
				succeeded++
				rows = append(rows, operationRow{Icon: successIcon(), Name: dn, Result: operationOutputSummary(r.Output, "up to date"), ResultStyle: dimStyle})
			}
		}

		for _, name := range missingNames {
			rows = append(rows, operationRow{Icon: warnIcon(), Name: displayRepoName(name), Result: "not cloned (use 'mrepo sync')", ResultStyle: dimStyle})
		}

		fmt.Println(renderOperationTable(rows))
		printOperationSummary("Fetch complete", succeeded, failed, len(missing), time.Since(started))
		if failed > 0 {
			return fmt.Errorf("fetch: %d repositories failed", failed)
		}
		return nil
	},
}

func init() {
	fetchCmd.Flags().BoolVar(&jsonOutput, "json", false, "output as JSON")
	fetchCmd.Flags().Int("parallel", defaultNetworkParallel(), "maximum concurrent fetches")
	fetchCmd.Flags().Duration("timeout", fetchTimeout, "timeout for each repository")
	rootCmd.AddCommand(fetchCmd)
}
