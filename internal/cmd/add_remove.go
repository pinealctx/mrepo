package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/pinealctx/mrepo/internal/config"
	"github.com/pinealctx/mrepo/internal/git"

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

		// Auto-detect remote and branch from the existing repo.
		absPath := filepath.Join(rootDir, repoPath)
		var remote, branch string
		if _, statErr := os.Stat(absPath); statErr == nil {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			info := git.GetRepoInfo(ctx, absPath)
			remote = info.Remote
			branch = info.Branch
		}

		if err := cfg.AddRepo(name, repoPath, remote, branch, desc); err != nil {
			if !force {
				return err
			}
			delete(cfg.Repos, name)
			if err := cfg.AddRepo(name, repoPath, remote, branch, desc); err != nil {
				return err
			}
		}

		if err := cfg.Save(cfgPath); err != nil {
			return err
		}

		// Add to group if --group is specified.
		if groupName != "" {
			if err := cfg.AddRepoToGroup(groupName, name); err != nil {
				fmt.Printf("  %s %s\n", warnIcon(), dimStyle.Render("warning: "+err.Error()))
			} else {
				if err := cfg.Save(cfgPath); err != nil {
					return err
				}
			}
		}

		// Add to .gitignore so root repo doesn't track the sub-repo.
		if err := ensureGitignore(rootDir, repoPath); err != nil {
			fmt.Printf("  %s %s\n", warnIcon(), dimStyle.Render("warning: could not update .gitignore: "+err.Error()))
		}

		fmt.Printf("  %s %s %s %s", successIcon(), boldStyle.Render("Added"), accentStyle.Render(name), dimStyle.Render("→ "+repoPath))
		if remote != "" {
			fmt.Printf(" %s", dimStyle.Render(fmt.Sprintf("(remote: %s", remote)))
			if branch != "" {
				fmt.Printf(", branch: %s", branch)
			}
			fmt.Print(dimStyle.Render(")"))
		}
		if groupName != "" {
			fmt.Printf(" %s", dimStyle.Render(fmt.Sprintf("→ group: %s", groupName)))
		}
		fmt.Println()
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

		cfgPath, cfg, err := loadConfig(rootDir)
		if err != nil {
			return err
		}

		repo, err := cfg.RemoveRepo(name)
		if err != nil {
			return err
		}

		if deleteDir && force {
			absPath := filepath.Join(rootDir, repo.Path)
			fmt.Printf("  %s %s\n", warnIcon(), warnStyle.Render("Removing directory: "+absPath))
			if err := os.RemoveAll(absPath); err != nil {
				return fmt.Errorf("remove directory: %w", err)
			}
		} else if deleteDir {
			// Re-add the repo so the user can retry with --force.
			if err := cfg.AddRepo(name, repo.Path, repo.Remote, repo.Branch, repo.Description); err != nil {
				return fmt.Errorf("failed to restore repo config after abort: %w", err)
			}
			if err := cfg.Save(cfgPath); err != nil {
				return fmt.Errorf("failed to save config after abort: %w", err)
			}
			return fmt.Errorf("--delete requires --force to actually remove the directory (repo untouched)")
		}

		if err := cfg.Save(cfgPath); err != nil {
			return err
		}

		// Remove from .gitignore.
		if err := removeFromGitignore(rootDir, repo.Path); err != nil {
			fmt.Printf("  %s %s\n", warnIcon(), dimStyle.Render("warning: could not update .gitignore: "+err.Error()))
		}

		fmt.Printf("  %s %s %s\n", errorIcon(), boldStyle.Render("Removed"), accentStyle.Render(name))
		return nil
	},
}

func init() {
	addCmd.Flags().String("desc", "", "repo description")
	addCmd.Flags().Bool("force", false, "overwrite if already exists")
	removeCmd.Flags().Bool("delete", false, "also remove the directory (requires --force)")
	removeCmd.Flags().Bool("force", false, "confirm directory deletion (required by --delete)")

	rootCmd.AddCommand(addCmd)
	rootCmd.AddCommand(removeCmd)
}
