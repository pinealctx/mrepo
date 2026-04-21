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
	"unicode/utf8"

	"github.com/pinealctx/mrepo/internal/config"
	"github.com/pinealctx/mrepo/internal/git"

	"github.com/spf13/cobra"
)

func isDirMissing(rootDir, relPath string) bool {
	absPath := filepath.Join(rootDir, relPath)
	_, err := os.Stat(absPath)
	return os.IsNotExist(err)
}

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
		var skipped []string
		for name, repo := range filterRepos(cfg) {
			if isDirMissing(rootDir, repo.Path) {
				skipped = append(skipped, name)
				continue
			}
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
				fmt.Printf("  %s %-20s %s\n", errorIcon(), r.Name, errorStyle.Render(truncate(r.Error.Error(), 80)))
			} else {
				fmt.Printf("  %s %-20s %s\n", successIcon(), r.Name, dimStyle.Render(truncate(r.Output, 80)))
			}
		}

		for _, name := range skipped {
			fmt.Printf("  %s %-20s %s\n", warnIcon(), name, dimStyle.Render("not cloned (use 'mrepo sync')"))
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

func truncate(s string, maxRunes int) string {
	if utf8.RuneCountInString(s) <= maxRunes {
		return s
	}
	runes := []rune(s)
	cut := maxRunes - 3
	if cut < 0 {
		cut = 0
	}
	return string(runes[:cut]) + "..."
}

func init() {
	pullCmd.Flags().BoolVar(&jsonOutput, "json", false, "output as JSON")
	rootCmd.AddCommand(pullCmd)
}
