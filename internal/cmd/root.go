package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	rootDir    string
	format     string
	jsonOutput bool
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
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
