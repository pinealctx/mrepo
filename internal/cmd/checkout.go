package cmd

import (
	"context"
	"fmt"
	"path/filepath"
	"runtime"
	"sort"
	"sync"
	"sync/atomic"

	"github.com/pinealctx/mrepo/internal/git"

	"github.com/spf13/cobra"
)

type jsonCheckout struct {
	Name   string `json:"name"`
	Branch string `json:"branch"`
	Output string `json:"output,omitempty"`
	Error  string `json:"error,omitempty"`
}

var checkoutCmd = &cobra.Command{
	Use:   "checkout <branch>",
	Short: "Checkout a branch across repos",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		branch := args[0]
		create, _ := cmd.Flags().GetBool("create")

		_, cfg, err := loadConfig(rootDir)
		if err != nil {
			return err
		}

		existing, _ := partitionRepos(filterRepos(cfg))

		ctx, cancel := context.WithTimeout(context.Background(), checkoutTimeout)
		defer cancel()

		results := parallelCheckout(ctx, existing, branch, create, runtime.NumCPU())
		sort.Slice(results, func(i, j int) bool {
			return results[i].Name < results[j].Name
		})

		if jsonOutput {
			return printJSON(results)
		}

		t := newResultTable()
		for _, r := range results {
			dn := displayRepoName(r.Name)
			if r.Error != "" {
				t.Row(errorIcon(), dn, errorStyle.Render(truncate(r.Error, 80)))
			} else {
				t.Row(successIcon(), dn, dimStyle.Render(truncate(r.Output, 80)))
			}
		}
		fmt.Println(t.Render())
		return nil
	},
}

func parallelCheckout(ctx context.Context, repos map[string]string, branch string, create bool, parallel int) []jsonCheckout {
	if parallel <= 0 {
		parallel = 4
	}

	results := make([]jsonCheckout, len(repos))
	var idx atomic.Int64
	var wg sync.WaitGroup
	sem := make(chan struct{}, parallel)

	for name, path := range repos {
		name, path := name, path
		i := int(idx.Add(1)) - 1
		wg.Add(1)
		sem <- struct{}{}

		go func() {
			defer wg.Done()
			defer func() { <-sem }()

			absPath := filepath.Join(rootDir, path)
			err := git.Checkout(ctx, absPath, branch, create)
			r := jsonCheckout{Name: name, Branch: branch}
			if err != nil {
				r.Error = err.Error()
			} else {
				r.Output = "switched to " + branch
			}
			results[i] = r
		}()
	}

	wg.Wait()
	return results
}

func init() {
	checkoutCmd.Flags().BoolVar(&jsonOutput, "json", false, "output as JSON")
	checkoutCmd.Flags().Bool("create", false, "create a new branch and checkout")
	rootCmd.AddCommand(checkoutCmd)
}
