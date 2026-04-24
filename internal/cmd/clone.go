package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"

	"github.com/pinealctx/mrepo/internal/config"
	"github.com/pinealctx/mrepo/internal/git"

	"github.com/spf13/cobra"
)

type jsonClone struct {
	Name   string `json:"name"`
	Path   string `json:"path"`
	Output string `json:"output,omitempty"`
	Error  string `json:"error,omitempty"`
}

var cloneCmd = &cobra.Command{
	Use:   "clone",
	Short: "Clone repos that are not yet on disk",
	RunE: func(cmd *cobra.Command, args []string) error {
		_, cfg, err := loadConfig(rootDir)
		if err != nil {
			return err
		}

		force, _ := cmd.Flags().GetBool("force")
		depth, _ := cmd.Flags().GetInt("depth")

		// Collect repos that need cloning.
		specs := make(map[string]git.CloneSpec)
		for name, repo := range filterRepos(cfg) {
			if err := config.ValidateRepo(rootDir, repo.Path, repo.Remote); err != nil {
				return fmt.Errorf("repo %q %w", name, err)
			}
			if repo.Remote == "" {
				continue
			}
			// Cannot clone into the workspace root itself.
			if repo.Path == "." {
				continue
			}
			absPath := filepath.Join(rootDir, repo.Path)
			if _, err := os.Stat(absPath); err == nil && !force {
				continue
			}
			if force && err == nil {
				if rmErr := os.RemoveAll(absPath); rmErr != nil {
					return fmt.Errorf("remove %s: %w", absPath, rmErr)
				}
			}
			specs[name] = git.CloneSpec{
				Path:   repo.Path,
				Remote: repo.Remote,
				Branch: repo.Branch,
				Depth:  depth,
			}
		}

		if len(specs) == 0 {
			fmt.Println(infoStyle.Render("  All repos already exist locally."))
			return nil
		}

		ctx, cancel := context.WithTimeout(context.Background(), cloneTimeout)
		defer cancel()

		results := git.CloneAll(ctx, rootDir, specs, runtime.NumCPU())
		sort.Slice(results, func(i, j int) bool {
			return results[i].Name < results[j].Name
		})

		if jsonOutput {
			out := make([]jsonClone, len(results))
			for i, r := range results {
				jc := jsonClone{Name: r.Name, Path: r.Path, Output: r.Output}
				if r.Error != nil {
					jc.Error = r.Error.Error()
				}
				out[i] = jc
			}
			return printJSON(out)
		}

		t := newResultTable()

		for _, r := range results {
			dn := displayRepoName(r.Name)
			if r.Error != nil {
				t.Row(errorIcon(), dn, errorStyle.Render(truncate(r.Error.Error(), 80)))
			} else {
				t.Row(cloneIcon(), dn, dimStyle.Render(truncate(r.Output, 80)))
			}
		}

		fmt.Println(t.Render())
		return nil
	},
}

func init() {
	cloneCmd.Flags().BoolVar(&jsonOutput, "json", false, "output as JSON")
	cloneCmd.Flags().Bool("force", false, "re-clone even if directory exists")
	cloneCmd.Flags().Int("depth", 0, "create a shallow clone with given depth (0 = full)")
	rootCmd.AddCommand(cloneCmd)
}
