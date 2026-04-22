package cmd

import (
	"fmt"

	"github.com/pinealctx/mrepo/internal/version"

	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(version.Get())
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
