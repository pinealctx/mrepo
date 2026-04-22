package cmd

import (
	"context"
	"fmt"
	"runtime"
	"sort"
	"unicode/utf8"

	"github.com/pinealctx/mrepo/internal/git"

	"github.com/spf13/cobra"
)

type jsonPull struct {
	Name   string `json:"name"`
	Path   string `json:"path"`
	Output string `json:"output,omitempty"`
	Error  string `json:"error,omitempty"`
}

var pullCmd = &cobra.Command{
	Use:   "pull",
	Short: "Pull latest changes for all repos",
	RunE: func(cmd *cobra.Command, args []string) error {
		_, cfg, err := loadConfig(rootDir)
		if err != nil {
			return err
		}

		existing, missing := partitionRepos(filterRepos(cfg))

		ctx, cancel := context.WithTimeout(context.Background(), pullTimeout)
		defer cancel()

		results := git.PullAll(ctx, rootDir, existing, runtime.NumCPU())
		sort.Slice(results, func(i, j int) bool {
			return results[i].Name < results[j].Name
		})

		if jsonOutput {
			out := make([]jsonPull, 0, len(results)+len(missing))
			for _, r := range results {
				jp := jsonPull{Name: r.Name, Path: r.Path, Output: r.Output}
				if r.Error != nil {
					jp.Error = r.Error.Error()
				}
				out = append(out, jp)
			}
			for name := range missing {
				out = append(out, jsonPull{Name: name, Error: "not cloned"})
			}
			return printJSON(out)
		}

		t := newResultTable()

		for _, r := range results {
			dn := displayRepoName(r.Name)
			if r.Error != nil {
				t.Row(errorIcon(), dn, errorStyle.Render(truncate(r.Error.Error(), 80)))
			} else {
				t.Row(successIcon(), dn, dimStyle.Render(truncate(r.Output, 80)))
			}
		}

		for name := range missing {
			t.Row(warnIcon(), displayRepoName(name), dimStyle.Render("not cloned (use 'mrepo sync')"))
		}

		fmt.Println(t.Render())
		return nil
	},
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
