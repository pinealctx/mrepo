package cmd

import (
	"github.com/pinealctx/mrepo/internal/tui"
	"github.com/pinealctx/mrepo/internal/version"

	"github.com/spf13/cobra"
)

var tuiCmd = &cobra.Command{
	Use:   "tui",
	Short: "Launch interactive TUI dashboard",
	RunE: func(cmd *cobra.Command, args []string) error {
		_, cfg, err := loadConfig(rootDir)
		if err != nil {
			return err
		}

		return tui.Run(rootDir, cfg, filterRepos(cfg), version.Version)
	},
}

func init() {
	rootCmd.AddCommand(tuiCmd)
}
