package cmd

import (
	"context"
	"fmt"
	"runtime"
	"sort"

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

		ctx, cancel := context.WithTimeout(context.Background(), fetchTimeout)
		defer cancel()

		results := git.FetchAll(ctx, rootDir, existing, runtime.NumCPU())
		sort.Slice(results, func(i, j int) bool {
			return results[i].Name < results[j].Name
		})

		if jsonOutput {
			out := make([]jsonFetch, 0, len(results)+len(missing))
			for _, r := range results {
				jf := jsonFetch{Name: r.Name, Path: r.Path, Output: r.Output}
				if r.Error != nil {
					jf.Error = r.Error.Error()
				}
				out = append(out, jf)
			}
			for name := range missing {
				out = append(out, jsonFetch{Name: name, Skipped: true})
			}
			return printJSON(out)
		}

		t := newResultTable()

		for _, r := range results {
			dn := displayRepoName(r.Name)
			if r.Error != nil {
				t.Row(errorIcon(), dn, errorStyle.Render(truncate(r.Error.Error(), 80)))
			} else if r.Output == "" {
				t.Row(infoIcon(), dn, dimStyle.Render("up to date"))
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

func init() {
	fetchCmd.Flags().BoolVar(&jsonOutput, "json", false, "output as JSON")
	rootCmd.AddCommand(fetchCmd)
}
