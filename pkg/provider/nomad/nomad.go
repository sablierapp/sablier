package nomad

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/hashicorp/nomad/api"
	"github.com/sablierapp/sablier/pkg/sablier"
)

// Interface guard
var _ sablier.Provider = (*Provider)(nil)

type Provider struct {
	Client          *api.Client
	namespace       string
	desiredReplicas int32
	l               *slog.Logger
}

func New(ctx context.Context, client *api.Client, namespace string, logger *slog.Logger) (*Provider, error) {
	logger = logger.With(slog.String("provider", "nomad"))

	if namespace == "" {
		namespace = "default"
	}

	// Test connection by getting agent self info
	agent := client.Agent()
	info, err := agent.Self()
	if err != nil {
		return nil, fmt.Errorf("cannot connect to nomad: %v", err)
	}

	version := "unknown"
	address := "unknown"

	if info != nil && info.Stats != nil {
		if nomadStats, ok := info.Stats["nomad"]; ok {
			if versionStr, exists := nomadStats["version"]; exists {
				version = versionStr
			}
		}
	}

	if info != nil && info.Config != nil {
		if addr, ok := info.Config["AdvertiseAddrs"]; ok {
			if addrMap, ok := addr.(map[string]interface{}); ok {
				if httpAddr, ok := addrMap["HTTP"].(string); ok {
					address = httpAddr
				}
			}
		}
	}

	logger.InfoContext(ctx, "connection established with nomad",
		slog.String("version", version),
		slog.String("namespace", namespace),
		slog.String("address", address),
	)

	return &Provider{
		Client:          client,
		namespace:       namespace,
		desiredReplicas: 1,
		l:               logger,
	}, nil
}
