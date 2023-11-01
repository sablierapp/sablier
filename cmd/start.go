package cmd

import (
	app "github.com/acouvreur/sablier/internal"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var newStartCommand = func() *cobra.Command {
	return &cobra.Command{
		Use:   "start",
		Short: "Start the Sablier app",
		RunE: func(cmd *cobra.Command, args []string) error {
			err := viper.Unmarshal(&conf)
			if err != nil {
				return err
			}

			return app.Start(conf)
		},
	}
}
