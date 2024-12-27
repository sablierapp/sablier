package docker_test

import (
	"context"
	"encoding/json"
	"github.com/sablierapp/sablier/pkg/provider"
	"github.com/sablierapp/sablier/pkg/provider/docker"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestDockerProvider_Stop(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	dind, err := setupDinD(t, ctx)
	if err != nil {
		t.Fatal(err)
	}

	p, err := docker.NewDockerProvider(dind.client)
	if err != nil {
		t.Fatal(err)
	}

	mimic, err := dind.CreateMimic(ctx, MimicOptions{
		WithHealth:   true,
		HealthyAfter: 2 * time.Second,
		RunningAfter: 1 * time.Second,
		SablierGroup: "test",
	})
	if err != nil {
		t.Fatal(err)
	}

	err = p.Start(ctx, mimic.ID, provider.StartOptions{
		DesiredReplicas:    1,
		ConsiderReadyAfter: 0,
	})
	if err != nil {
		t.Fatal(err)
	}

	err = <-p.AfterReady(ctx, mimic.ID)
	if err != nil {
		t.Fatal(err)
	}

	err = p.Stop(ctx, mimic.ID)
	if err != nil {
		t.Fatal(err)
	}

	inspect, err := dind.client.ContainerInspect(ctx, mimic.ID)
	if err != nil {
		t.Fatal(err)
	}

	resp, err := json.Marshal(inspect)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("inspect: %+v\n", string(resp))
	assert.Equal(t, inspect.State.Status, "exited")
}
