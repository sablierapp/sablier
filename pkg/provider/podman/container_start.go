package podman

import (
	"context"
	"fmt"

	"github.com/containers/podman/v5/pkg/bindings/containers"
)

func (p *Provider) InstanceStart(ctx context.Context, name string) error {
	// TODO: Create a context from the ctx argument with the p.conn

	err := containers.Start(p.conn, name, nil)
	if err != nil {
		return fmt.Errorf("cannot start container %s: %w", name, err)
	}
	return nil
}
