package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/pinealctx/mrepo/internal/config"

	"github.com/spf13/cobra"
)

var (
	rootDir    string
	format     string
	jsonOutput bool
	groupName  string
)

// rootRepoName is the special name used for the monorepo root itself.
const rootRepoName = "."

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
// It always includes the root repo (".").
func filterRepos(cfg *config.Config) map[string]*config.Repo {
	repos := cfg.Repos

	if groupName != "" {
		names, err := cfg.RepoNamesForGroup(groupName)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}

		filtered := make(map[string]*config.Repo, len(names)+1)
		for _, n := range names {
			if r, ok := cfg.Repos[n]; ok {
				filtered[n] = r
			}
		}
		repos = filtered
	}

	// Always include the root repo itself.
	if isRootGitRepo() {
		result := make(map[string]*config.Repo, len(repos)+1)
		result[rootRepoName] = &config.Repo{Path: "."}
		for name, repo := range repos {
			result[name] = repo
		}
		return result
	}

	return repos
}

// isRootGitRepo checks if the root directory contains a .git directory.
func isRootGitRepo() bool {
	_, err := os.Stat(filepath.Join(rootDir, ".git"))
	return err == nil
}
