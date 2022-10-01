package cmd

import (
	"fmt"

	"github.com/acouvreur/sablier/config"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func init() {
	rootCmd.AddCommand(versionCmd)
}

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the Sablier server",
	Run: func(cmd *cobra.Command, args []string) {
		conf := config.NewConfig()
		viper.Unmarshal(&conf)

		fmt.Printf("In Start: %v\n", conf)
	},
}
