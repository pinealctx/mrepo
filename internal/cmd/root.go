package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/table"

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

// Timeout constants for operations.
const (
	statusTimeout   = 30 * time.Second
	pullTimeout     = 60 * time.Second
	fetchTimeout    = 60 * time.Second
	cloneTimeout    = 120 * time.Second
	syncTimeout     = 120 * time.Second
	forallTimeout   = 120 * time.Second
	scanTimeout     = 10 * time.Second
	checkoutTimeout = 30 * time.Second
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
	repos := cfg.Repos

	if groupName != "" {
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
		repos = filtered
	}

	return repos
}

// loadConfig finds and loads the config file for the given root directory.
func loadConfig(rootDir string) (string, *config.Config, error) {
	cfgPath, err := config.FindConfigFile(rootDir)
	if err != nil {
		return "", nil, err
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return "", nil, err
	}
	return cfgPath, cfg, nil
}

// printJSON writes v as indented JSON to stdout.
func printJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// newResultTable creates a styled table for displaying operation results.
func newResultTable() *table.Table {
	return table.New().
		Width(80).
		Border(lipgloss.Border{}).
		StyleFunc(func(row, col int) lipgloss.Style {
			return lipgloss.NewStyle()
		})
}

// newHeaderTable creates a styled table with column headers.
func newHeaderTable(headers ...string) *table.Table {
	return table.New().
		Headers(headers...).
		Width(80).
		Border(lipgloss.NormalBorder()).
		BorderTop(false).BorderBottom(false).
		BorderLeft(false).BorderRight(false).
		BorderHeader(true).
		BorderColumn(false).
		BorderRow(false).
		StyleFunc(func(row, col int) lipgloss.Style {
			if row == table.HeaderRow {
				return boldStyle
			}
			return lipgloss.NewStyle()
		})
}

// isDirMissing checks if a repo directory exists within the root.
func isDirMissing(rootDir, relPath string) bool {
	absPath := filepath.Join(rootDir, relPath)
	_, err := os.Stat(absPath)
	return os.IsNotExist(err)
}

// partitionRepos splits filtered repos into existing and missing sets.
func partitionRepos(filtered map[string]*config.Repo) (existing, missing map[string]string) {
	existing = make(map[string]string)
	missing = make(map[string]string)
	for name, repo := range filtered {
		if isDirMissing(rootDir, repo.Path) {
			missing[name] = repo.Path
		} else {
			existing[name] = repo.Path
		}
	}
	return
}
