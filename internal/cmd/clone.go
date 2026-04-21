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

var cloneCmd = &cobra.Command{
	Use:   "clone",
	Short: "Clone repos that are not yet on disk",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfgPath, err := config.FindConfigFile(rootDir)
		if err != nil {
			return err
		}
		cfg, err := config.Load(cfgPath)
		if err != nil {
			return err
		}

		force, _ := cmd.Flags().GetBool("force")
		depth, _ := cmd.Flags().GetInt("depth")

		// Collect repos that need cloning.
		specs := make(map[string]git.CloneSpec)
		for name, repo := range filterRepos(cfg) {
			if repo.Remote == "" {
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

		ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
		defer cancel()

		results := git.CloneAll(ctx, rootDir, specs, runtime.NumCPU())
		sort.Slice(results, func(i, j int) bool {
			return results[i].Name < results[j].Name
		})

		if jsonOutput {
			return printCloneJSON(results)
		}

		for _, r := range results {
			if r.Error != nil {
				fmt.Printf("  %s %-20s %s\n", errorIcon(), r.Name, errorStyle.Render(truncate(r.Error.Error(), 80)))
			} else {
				fmt.Printf("  %s %-20s %s\n", cloneIcon(), r.Name, dimStyle.Render(truncate(r.Output, 80)))
			}
		}
		return nil
	},
}

func printCloneJSON(results []*git.CloneResult) error {
	type jsonClone struct {
		Name   string `json:"name"`
		Path   string `json:"path"`
		Output string `json:"output,omitempty"`
		Error  string `json:"error,omitempty"`
	}

	out := make([]jsonClone, len(results))
	for i, r := range results {
		jc := jsonClone{Name: r.Name, Path: r.Path, Output: r.Output}
		if r.Error != nil {
			jc.Error = r.Error.Error()
		}
		out[i] = jc
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

func init() {
	cloneCmd.Flags().BoolVar(&jsonOutput, "json", false, "output as JSON")
	cloneCmd.Flags().Bool("force", false, "re-clone even if directory exists")
	cloneCmd.Flags().Int("depth", 0, "create a shallow clone with given depth (0 = full)")
	rootCmd.AddCommand(cloneCmd)
}
