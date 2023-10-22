package cmd

import (
	app "github.com/acouvreur/sablier/internal"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var newStartCommand = func() *cobra.Command {
	return &cobra.Command{
		Use:   "start",
		Short: "Start the Sablier server",
		Run: func(cmd *cobra.Command, args []string) {
			viper.Unmarshal(&conf)

			err := app.Start(conf)
			if err != nil {
				panic(err)
			}
		},
	}
}
