package cmd

import (
	"context"
	"fmt"
	"path/filepath"
	"runtime"
	"sort"
	"sync"
	"time"

	"github.com/pinealctx/mrepo/internal/config"
	"github.com/pinealctx/mrepo/internal/git"

	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
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
		for name, repo := range cfg.Repos {
			repos[name] = repo.Path
		}

		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		var results []*git.PullResult
		var mu sync.Mutex
		eg, egCtx := errgroup.WithContext(ctx)
		eg.SetLimit(runtime.NumCPU())

		for name, relPath := range repos {
			name, relPath := name, relPath
			eg.Go(func() error {
				absPath := filepath.Join(rootDir, relPath)
				r := git.Fetch(egCtx, name, absPath)
				mu.Lock()
				results = append(results, r)
				mu.Unlock()
				return nil
			})
		}

		_ = eg.Wait()
		sort.Slice(results, func(i, j int) bool {
			return results[i].Name < results[j].Name
		})

		for _, r := range results {
			if r.Error != nil {
				fmt.Printf("  x %s: %s\n", r.Name, r.Error)
			} else if r.Output == "" {
				fmt.Printf("  ok %s: up to date\n", r.Name)
			} else {
				fmt.Printf("  ok %s: %s\n", r.Name, truncate(r.Output, 80))
			}
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(fetchCmd)
}
