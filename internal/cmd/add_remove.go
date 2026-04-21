package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/pinealctx/mrepo/internal/config"

	"github.com/spf13/cobra"
)

var addCmd = &cobra.Command{
	Use:   "add [path]",
	Short: "Add a repo to the config",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		repoPath := filepath.ToSlash(args[0])
		name := config.RepoNameFromPath(repoPath)

		desc, _ := cmd.Flags().GetString("desc")
		force, _ := cmd.Flags().GetBool("force")

		cfgPath, err := config.EnsureConfig(rootDir, format)
		if err != nil {
			return err
		}

		cfg, err := config.Load(cfgPath)
		if err != nil {
			return err
		}

		if err := cfg.AddRepo(name, repoPath, "", "", desc); err != nil {
			if !force {
				return err
			}
			delete(cfg.Repos, name)
			_ = cfg.AddRepo(name, repoPath, "", "", desc)
		}

		if err := cfg.Save(cfgPath); err != nil {
			return err
		}

		fmt.Printf("Added repo %q -> %s\n", name, repoPath)
		return nil
	},
}

var removeCmd = &cobra.Command{
	Use:   "remove [name]",
	Short: "Remove a repo from the config",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		force, _ := cmd.Flags().GetBool("force")
		deleteDir, _ := cmd.Flags().GetBool("delete")

		cfgPath, err := config.FindConfigFile(rootDir)
		if err != nil {
			return err
		}

		cfg, err := config.Load(cfgPath)
		if err != nil {
			return err
		}

		repo, err := cfg.RemoveRepo(name)
		if err != nil {
			return err
		}

		if deleteDir {
			absPath := filepath.Join(rootDir, repo.Path)
			if force {
				fmt.Printf("Removing directory: %s\n", absPath)
				if err := os.RemoveAll(absPath); err != nil {
					return fmt.Errorf("remove directory: %w", err)
				}
			} else {
				fmt.Printf("Directory not removed (use --force to delete): %s\n", absPath)
			}
		}

		if err := cfg.Save(cfgPath); err != nil {
			return err
		}

		fmt.Printf("Removed repo %q\n", name)
		return nil
	},
}

func init() {
	addCmd.Flags().String("desc", "", "repo description")
	addCmd.Flags().Bool("force", false, "overwrite if already exists")
	removeCmd.Flags().Bool("delete", false, "also remove the directory")
	removeCmd.Flags().Bool("force", false, "actually delete the directory (requires --delete)")

	rootCmd.AddCommand(addCmd)
	rootCmd.AddCommand(removeCmd)
}
