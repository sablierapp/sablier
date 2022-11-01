package cmd

import (
	"github.com/acouvreur/sablier/app"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var newStartCommand = func() *cobra.Command {
	cmd := &cobra.Command{

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
	return cmd
}
