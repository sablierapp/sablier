package cmd

import (
	"fmt"

	"github.com/sablierapp/sablier/version"
	"github.com/spf13/cobra"
)

var newVersionCommand = func() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the version Sablier",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println(version.Info())
		},
	}
}
