package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/creativeprojects/go-selfupdate"
	"github.com/pinealctx/mrepo/internal/version"
	"github.com/spf13/cobra"
)

var (
	updateCheck      bool
	updatePrerelease bool
)

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update mrepo to the latest version",
	RunE: func(cmd *cobra.Command, args []string) error {
		current := version.Get()
		if current == "dev" {
			return fmt.Errorf("cannot update development builds")
		}

		slug := selfupdate.ParseSlug("pinealctx/mrepo")

		updater, err := selfupdate.NewUpdater(selfupdate.Config{
			Prerelease: updatePrerelease,
		})
		if err != nil {
			return fmt.Errorf("failed to create updater: %w", err)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		latest, found, err := updater.DetectLatest(ctx, slug)
		if err != nil {
			return fmt.Errorf("failed to detect latest version: %w", err)
		}
		if !found {
			return fmt.Errorf("no release found for %s/%s", latest.OS, latest.Arch)
		}

		if latest.LessOrEqual(current) {
			fmt.Printf("%s %s\n", successIcon(), boldStyle.Render(fmt.Sprintf("Already at latest version %s", current)))
			return nil
		}

		if updateCheck {
			fmt.Printf("%s Update available: %s → %s\n",
				warnIcon(),
				accentStyle.Render(current),
				successStyle.Render(latest.Version()),
			)
			return nil
		}

		exe, err := selfupdate.ExecutablePath()
		if err != nil {
			return fmt.Errorf("failed to locate executable: %w", err)
		}

		if err := updater.UpdateTo(ctx, latest, exe); err != nil {
			return fmt.Errorf("update failed: %w", err)
		}

		fmt.Printf("%s Updated to %s\n", successIcon(), successStyle.Render(latest.Version()))
		return nil
	},
}

func init() {
	updateCmd.Flags().BoolVarP(&updateCheck, "check", "c", false, "check for available update without installing")
	updateCmd.Flags().BoolVar(&updatePrerelease, "prerelease", false, "include pre-release versions")
	rootCmd.AddCommand(updateCmd)
}
