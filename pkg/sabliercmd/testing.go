package sabliercmd

import (
	"github.com/sablierapp/sablier/pkg/config"
	"github.com/spf13/cobra"
)

// defaultStartCommand stores the original start command function
var defaultStartCommand = newStartCommand

// NewStartCommand is a variable that allows mocking the start command for testing
var NewStartCommand = newStartCommand

// SetStartCommand allows tests to override the start command
func SetStartCommand(cmd func() *cobra.Command) {
	newStartCommand = cmd
}

// ResetStartCommand resets the start command to the default
func ResetStartCommand() {
	newStartCommand = defaultStartCommand
}

// GetConfig returns the current configuration (for testing)
func GetConfig() *config.Config {
	return &conf
}

// ResetConfig resets the configuration to a new instance (for testing)
func ResetConfig() {
	conf = config.NewConfig()
}
