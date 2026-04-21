package cmd

import (
	"github.com/pinealctx/mrepo/internal/config"
	"github.com/pinealctx/mrepo/internal/tui"

	"github.com/spf13/cobra"
)

var tuiCmd = &cobra.Command{
	Use:   "tui",
	Short: "Launch interactive TUI dashboard",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfgPath, err := config.FindConfigFile(rootDir)
		if err != nil {
			return err
		}
		cfg, err := config.Load(cfgPath)
		if err != nil {
			return err
		}

		return tui.Run(rootDir, cfg)
	},
}

func init() {
	rootCmd.AddCommand(tuiCmd)
}
