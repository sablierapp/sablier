package cmd

import (
	"fmt"
	"os"

	"github.com/sablierapp/sablier/app/http/healthcheck"
	"github.com/spf13/cobra"
)

var newHealthCommand = func() *cobra.Command {
	return &cobra.Command{
		Use:   "health",
		Short: "Calls the health endpoint of a Sablier instance",
		Run: func(cmd *cobra.Command, args []string) {
			details, healthy := healthcheck.Health(cmd.Flag("url").Value.String())

			if healthy {
				fmt.Fprintf(os.Stderr, "healthy: %v\n", details)
				os.Exit(0)
			} else {
				fmt.Fprintf(os.Stderr, "unhealthy: %v\n", details)
				os.Exit(1)
			}
		},
	}
}
