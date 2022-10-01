package cmd

import (
	"fmt"

	"github.com/acouvreur/sablier/version"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(versionCmd)
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number of Sablier",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(version.Info())
	},
}
