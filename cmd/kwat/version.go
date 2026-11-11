package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	Version   = "0.1.0-dev"
	GitCommit = "none"
	BuildDate = "unknown"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version information of KWAT",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("KWAT CLI Agent v%s (commit: %s, built: %s)\n", Version, GitCommit, BuildDate)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}

