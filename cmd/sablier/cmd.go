package main

import (
	"os"
	_ "time/tzdata"

	// The official image is built FROM scratch, without CA certificates. Go
	// uses these embedded roots only when the system has none.
	_ "golang.org/x/crypto/x509roots/fallback"

	"github.com/sablierapp/sablier/pkg/sabliercmd"
)

func main() {
	cmd := sabliercmd.NewRootCommand()
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
