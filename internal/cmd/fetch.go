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

var fetchCmd = &cobra.Command{
	Use:   "fetch",
	Short: "Fetch latest refs for all repos",
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
		var skipped []string
		for name, repo := range cfg.Repos {
			if isDirMissing(rootDir, repo.Path) {
				skipped = append(skipped, name)
				continue
			}
			repos[name] = repo.Path
		}

		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		results := git.FetchAll(ctx, rootDir, repos, runtime.NumCPU())
		sort.Slice(results, func(i, j int) bool {
			return results[i].Name < results[j].Name
		})

		if jsonOutput {
			return printFetchJSON(results, skipped)
		}

		for _, r := range results {
			if r.Error != nil {
				fmt.Printf("  x %s: %s\n", r.Name, r.Error)
			} else if r.Output == "" {
				fmt.Printf("  ok %s: up to date\n", r.Name)
			} else {
				fmt.Printf("  ok %s: %s\n", r.Name, truncate(r.Output, 80))
			}
		}

		for _, name := range skipped {
			fmt.Printf("  - %s: not cloned (use 'mrepo sync')\n", name)
		}
		return nil
	},
}

func printFetchJSON(results []*git.PullResult, skipped []string) error {
	type jsonFetch struct {
		Name    string `json:"name"`
		Path    string `json:"path"`
		Output  string `json:"output,omitempty"`
		Error   string `json:"error,omitempty"`
		Skipped bool   `json:"skipped,omitempty"`
	}

	var out []jsonFetch
	for _, r := range results {
		jf := jsonFetch{Name: r.Name, Path: r.Path, Output: r.Output}
		if r.Error != nil {
			jf.Error = r.Error.Error()
		}
		out = append(out, jf)
	}
	for _, name := range skipped {
		out = append(out, jsonFetch{Name: name, Skipped: true})
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

func init() {
	fetchCmd.Flags().BoolVar(&jsonOutput, "json", false, "output as JSON")
	rootCmd.AddCommand(fetchCmd)
}
