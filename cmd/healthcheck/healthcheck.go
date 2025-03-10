package healthcheck

import (
	"fmt"
	"github.com/spf13/cobra"
	"io"
	"net/http"
	"os"
)

const (
	healthy   = true
	unhealthy = false
)

func Health(url string) (string, bool) {
	resp, err := http.Get(url)

	if err != nil {
		return err.Error(), unhealthy
	}

	body, err := io.ReadAll(resp.Body)

	if err != nil {
		return err.Error(), unhealthy
	}

	if resp.StatusCode >= 400 {
		return string(body), unhealthy
	}

	return string(body), healthy
}

func NewCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "health",
		Short: "Calls the health endpoint of a Sablier instance",
		Run: func(cmd *cobra.Command, args []string) {
			details, healthy := Health(cmd.Flag("url").Value.String())

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
