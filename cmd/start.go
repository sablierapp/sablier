package cmd

import (
	"github.com/sablierapp/sablier/app"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var newStartCommand = func() *cobra.Command {
	return &cobra.Command{
		Use:   "start",
		Short: "InstanceStart the Sablier server",
		Run: func(cmd *cobra.Command, args []string) {
			viper.Unmarshal(&conf)

			err := app.Start(cmd.Context(), conf)
			if err != nil {
				panic(err)
			}
		},
	}
}
