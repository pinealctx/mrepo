package cmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync/atomic"
	"unicode/utf8"

	"github.com/pinealctx/mrepo/internal/config"

	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
)

type forallResult struct {
	Name   string `json:"name"`
	Path   string `json:"path"`
	Output string `json:"output,omitempty"`
	Error  string `json:"error,omitempty"`
}

var forallCmd = &cobra.Command{
	Use:   "forall -- <command> [args...]",
	Short: "Run a command in each repo",
	Example: `  mrepo forall -- make build
  mrepo forall -- git tag v1.0
  mrepo forall --group backend -- go test ./...`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return fmt.Errorf("usage: mrepo forall -- <command> [args...]")
		}

		_, cfg, err := loadConfig(rootDir)
		if err != nil {
			return err
		}

		repos := filterRepos(cfg)

		ctx, cancel := context.WithTimeout(context.Background(), forallTimeout)
		defer cancel()

		results := runForall(ctx, repos, args, runtime.NumCPU())
		sort.Slice(results, func(i, j int) bool {
			return results[i].Name < results[j].Name
		})

		if jsonOutput {
			return printJSON(results)
		}

		for _, r := range results {
			dn := displayRepoName(r.Name)
			if r.Error != "" {
				fmt.Printf("  %s %s %s\n", errorIcon(), padRight(dn, 20), errorStyle.Render(truncate(r.Error, 80)))
			} else if r.Output != "" {
				fmt.Printf("  %s %s:\n%s\n", successIcon(), accentStyle.Render(dn), dimStyle.Render(indent(r.Output)))
			} else {
				fmt.Printf("  %s %s %s\n", successIcon(), padRight(dn, 20), dimStyle.Render("(no output)"))
			}
		}
		return nil
	},
}

func runForall(ctx context.Context, repos map[string]*config.Repo, args []string, parallel int) []forallResult {
	if parallel <= 0 {
		parallel = 4
	}

	results := make([]forallResult, len(repos))
	var idx atomic.Int64

	eg, egCtx := errgroup.WithContext(ctx)
	eg.SetLimit(parallel)

	for name := range repos {
		name := name
		i := int(idx.Add(1)) - 1

		eg.Go(func() error {
			absPath := filepath.Join(rootDir, repos[name].Path)
			r := forallResult{Name: name, Path: repos[name].Path}

			if _, statErr := os.Stat(absPath); os.IsNotExist(statErr) {
				r.Error = "directory not found"
				results[i] = r
				return nil
			}

			cmd := exec.CommandContext(egCtx, args[0], args[1:]...)
			cmd.Dir = absPath
			out, err := cmd.CombinedOutput()
			r.Output = strings.TrimSpace(string(out))
			if err != nil {
				r.Error = err.Error()
			}
			results[i] = r
			return nil
		})
	}

	_ = eg.Wait()
	return results
}

func indent(s string) string {
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lines[i] = "    " + line
	}
	return strings.Join(lines, "\n")
}

// padRight pads s with spaces to the given visible width.
func padRight(s string, width int) string {
	n := utf8.RuneCountInString(s)
	if n >= width {
		return s
	}
	return s + strings.Repeat(" ", width-n)
}

func init() {
	forallCmd.Flags().BoolVar(&jsonOutput, "json", false, "output as JSON")
	rootCmd.AddCommand(forallCmd)
}
