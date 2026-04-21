package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"sort"
	"time"

	"github.com/pinealctx/mrepo/internal/config"
	"github.com/pinealctx/mrepo/internal/git"

	"github.com/spf13/cobra"
)

var pullCmd = &cobra.Command{
	Use:   "pull",
	Short: "Pull latest changes for all repos",
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
		for name, repo := range cfg.Repos {
			repos[name] = repo.Path
		}

		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		results := git.PullAll(ctx, rootDir, repos, runtime.NumCPU())
		sort.Slice(results, func(i, j int) bool {
			return results[i].Name < results[j].Name
		})

		if jsonOutput {
			return printPullJSON(results)
		}

		for _, r := range results {
			if r.Error != nil {
				fmt.Printf("  ✗ %s: %s\n", r.Name, r.Error)
			} else {
				fmt.Printf("  ✓ %s: %s\n", r.Name, truncate(r.Output, 80))
			}
		}
		return nil
	},
}

func printPullJSON(results []*git.PullResult) error {
	type jsonPull struct {
		Name   string `json:"name"`
		Path   string `json:"path"`
		Output string `json:"output,omitempty"`
		Error  string `json:"error,omitempty"`
	}

	out := make([]jsonPull, len(results))
	for i, r := range results {
		jp := jsonPull{Name: r.Name, Path: r.Path, Output: r.Output}
		if r.Error != nil {
			jp.Error = r.Error.Error()
		}
		out[i] = jp
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}

func init() {
	pullCmd.Flags().BoolVar(&jsonOutput, "json", false, "output as JSON")
	rootCmd.AddCommand(pullCmd)
}
