package cmd

import (
	"fmt"
	"os"

	"github.com/pinealctx/mrepo/internal/config"

	"github.com/spf13/cobra"
)

var (
	rootDir    string
	format     string
	jsonOutput bool
	groupName  string
)

var rootCmd = &cobra.Command{
	Use:           "mrepo",
	Short:         "Monorepo multi-repository manager",
	Long:          "mrepo manages multiple Git repositories within a monorepo workspace.",
	SilenceUsage:  true,
	SilenceErrors: true,
}

func init() {
	rootCmd.PersistentFlags().StringVar(&rootDir, "root", ".", "root directory of the monorepo")
	rootCmd.PersistentFlags().StringVar(&format, "format", "", "config file format (toml, yaml). auto-detected if omitted")
	rootCmd.PersistentFlags().StringVar(&groupName, "group", "", "filter repos by group name")
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// filterRepos returns the repos to operate on, filtered by --group if set.
func filterRepos(cfg *config.Config) map[string]*config.Repo {
	if groupName == "" {
		return cfg.Repos
	}

	names, err := cfg.RepoNamesForGroup(groupName)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	filtered := make(map[string]*config.Repo, len(names))
	for _, n := range names {
		if r, ok := cfg.Repos[n]; ok {
			filtered[n] = r
		}
	}
	return filtered
}
