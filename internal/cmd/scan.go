package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/pinealctx/mrepo/internal/config"
	"github.com/pinealctx/mrepo/internal/git"

	"github.com/spf13/cobra"
)

var scanCmd = &cobra.Command{
	Use:   "scan",
	Short: "Scan for Git repos not yet in config",
	RunE: func(cmd *cobra.Command, args []string) error {
		addAll, _ := cmd.Flags().GetBool("add")

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		found, err := git.ScanGitRepos(ctx, rootDir)
		if err != nil {
			return err
		}

		cfgPath, cfgErr := config.FindConfigFile(rootDir)
		cfg := config.New()
		if cfgErr == nil {
			loaded, loadErr := config.Load(cfgPath)
			if loadErr != nil {
				return fmt.Errorf("parse config: %w", loadErr)
			}
			cfg = loaded
		}

		var newRepos []string
		for _, path := range found {
			name := config.RepoNameFromPath(path)
			if _, exists := cfg.Repos[name]; !exists {
				newRepos = append(newRepos, path)
			}
		}

		if jsonOutput {
			return printScanJSON(found, newRepos)
		}

		if len(newRepos) == 0 {
			fmt.Println("All Git repos are already tracked.")
			return nil
		}

		fmt.Printf("Found %d untracked repo(s):\n", len(newRepos))
		for _, path := range newRepos {
			fmt.Printf("  - %s\n", path)
		}

		if addAll {
			return addScannedRepos(ctx, cfgPath, cfg, newRepos)
		}

		fmt.Println("\nUse --add to add them all, or 'mrepo add <path>' individually.")
		return nil
	},
}

func addScannedRepos(ctx context.Context, cfgPath string, cfg *config.Config, repos []string) error {
	for _, path := range repos {
		name := config.RepoNameFromPath(path)
		absPath := filepath.Join(rootDir, path)

		// Auto-detect remote URL and current branch.
		info := git.GetRepoInfo(ctx, absPath)

		_ = cfg.AddRepo(name, path, info.Remote, info.Branch, "")
		fmt.Printf("  Added %s", name)
		if info.Remote != "" {
			fmt.Printf(" (remote: %s", info.Remote)
			if info.Branch != "" {
				fmt.Printf(", branch: %s", info.Branch)
			}
			fmt.Print(")")
		}
		fmt.Println()
	}

	if cfgPath == "" {
		cfgPath = config.ConfigPath(rootDir, config.FormatTOML)
	}
	return cfg.Save(cfgPath)
}

func printScanJSON(found []string, newRepos []string) error {
	type scanResult struct {
		All      []string `json:"all"`
		New      []string `json:"new"`
		Count    int      `json:"count"`
		NewCount int      `json:"new_count"`
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(scanResult{
		All:      found,
		New:      newRepos,
		Count:    len(found),
		NewCount: len(newRepos),
	})
}

func init() {
	scanCmd.Flags().BoolVar(&jsonOutput, "json", false, "output as JSON")
	scanCmd.Flags().Bool("add", false, "add all found repos to config (auto-detect remote and branch)")
	rootCmd.AddCommand(scanCmd)
}
