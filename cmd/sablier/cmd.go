package main

import (
	"os"
	_ "time/tzdata"

	"github.com/sablierapp/sablier/pkg/sabliercmd"
)

func main() {
	cmd := sabliercmd.NewRootCommand()
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
