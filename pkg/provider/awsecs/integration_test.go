package awsecs_test

import (
	"context"
	"fmt"
	"slices"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/neilotoole/slogt"
	"gotest.tools/v3/assert"

	"github.com/sablierapp/sablier/pkg/provider"
	"github.com/sablierapp/sablier/pkg/provider/awsecs"
	"github.com/sablierapp/sablier/pkg/sablier"
)

const itPollInterval = time.Second

var itSeq atomic.Int64

func uniqueName(prefix string) string {
	return fmt.Sprintf("%s-%d-%d", prefix, time.Now().UnixNano()%1_000_000, itSeq.Add(1))
}

// ecsEnv is a cluster and a task definition that runs mimic.
type ecsEnv struct {
	client         *ecs.Client
	cluster        string
	taskDefinition string
}

// newECSEnv creates the cluster and the task definition, and deletes the cluster when the test ends.
func newECSEnv(t *testing.T, client *ecs.Client) *ecsEnv {
	t.Helper()
	ctx := t.Context()
	cluster := uniqueName("sablier-it")

	_, err := client.CreateCluster(ctx, &ecs.CreateClusterInput{ClusterName: aws.String(cluster)})
	assert.NilError(t, err, "cannot create cluster")
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if _, err := client.DeleteCluster(ctx, &ecs.DeleteClusterInput{Cluster: aws.String(cluster)}); err != nil {
			t.Logf("cleanup: cannot delete cluster %q: %v", cluster, err)
		}
	})

	td, err := client.RegisterTaskDefinition(ctx, &ecs.RegisterTaskDefinitionInput{
		Family: aws.String(uniqueName("sablier-it-mimic")),
		ContainerDefinitions: []types.ContainerDefinition{{
			Name:       aws.String("mimic"),
			Image:      aws.String(mimicImage),
			EntryPoint: []string{"/mimic"},
			Command:    []string{"-running", "-running-after=1s", "-healthy=false"},
			Essential:  aws.Bool(true),
		}},
	})
	assert.NilError(t, err, "cannot register task definition")

	return &ecsEnv{
		client:         client,
		cluster:        cluster,
		taskDefinition: aws.ToString(td.TaskDefinition.TaskDefinitionArn),
	}
}

// createService creates a service with no desired task, tags it, and deletes it
// when the test ends. It returns the service ARN.
func (e *ecsEnv) createService(t *testing.T, name string, tags map[string]string) string {
	t.Helper()
	ctx := t.Context()
	out, err := e.client.CreateService(ctx, &ecs.CreateServiceInput{
		Cluster:        aws.String(e.cluster),
		ServiceName:    aws.String(name),
		TaskDefinition: aws.String(e.taskDefinition),
		DesiredCount:   aws.Int32(0),
	})
	assert.NilError(t, err, "cannot create service %q", name)
	arn := aws.ToString(out.Service.ServiceArn)
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_, err := e.client.DeleteService(ctx, &ecs.DeleteServiceInput{
			Cluster: aws.String(e.cluster),
			Service: aws.String(arn),
			Force:   aws.Bool(true),
		})
		if err != nil {
			t.Logf("cleanup: cannot delete service %q: %v", name, err)
		}
	})

	if len(tags) > 0 {
		ecsTags := make([]types.Tag, 0, len(tags))
		for k, v := range tags {
			ecsTags = append(ecsTags, types.Tag{Key: aws.String(k), Value: aws.String(v)})
		}
		_, err = e.client.TagResource(ctx, &ecs.TagResourceInput{ResourceArn: aws.String(arn), Tags: ecsTags})
		assert.NilError(t, err, "cannot tag service %q", name)
	}
	return arn
}

// waitStreamLive creates tagged probe services until the stream reports one.
// A service that exists before the first poll is part of the baseline and gives no event.
func (e *ecsEnv) waitStreamLive(t *testing.T, events <-chan sablier.InstanceEvent) {
	t.Helper()
	deadline := time.Now().Add(time.Minute)
	for time.Now().Before(deadline) {
		name := uniqueName("sablier-it-probe")
		e.createService(t, name, map[string]string{sablier.LabelEnable: "true"})
		timeout := time.After(5 * itPollInterval)
		for waiting := true; waiting; {
			select {
			case ev, ok := <-events:
				if !ok {
					t.Fatal("event stream closed")
				}
				if ev.Type == provider.InstanceEventCreated && ev.Info.Name == name {
					return
				}
			case <-timeout:
				waiting = false
			}
		}
	}
	t.Fatal("the event stream did not report a new service within 1m")
}

// waitStatus polls InstanceInspect until the service has the wanted status.
func waitStatus(t *testing.T, p *awsecs.Provider, name string, want sablier.InstanceStatus, timeout time.Duration) sablier.InstanceInfo {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for {
		info, err := p.InstanceInspect(t.Context(), name)
		assert.NilError(t, err)
		if info.Status == want {
			return info
		}
		if time.Now().After(deadline) {
			t.Fatalf("service %q is not %s within %s (last status %q, message %q)", name, want, timeout, info.Status, info.Message)
		}
		time.Sleep(500 * time.Millisecond)
	}
}

// waitEvent reads events until one of the wanted type for the named service arrives.
func waitEvent(t *testing.T, events <-chan sablier.InstanceEvent, name string, want provider.InstanceEventType, timeout time.Duration) {
	t.Helper()
	deadline := time.After(timeout)
	for {
		select {
		case ev, ok := <-events:
			if !ok {
				t.Fatal("event stream closed")
			}
			if ev.Type == want && ev.Info.Name == name {
				return
			}
		case <-deadline:
			t.Fatalf("no %s event for %q within %s", want, name, timeout)
		}
	}
}

func hasInstance(instances []sablier.InstanceConfiguration, name string) bool {
	return slices.ContainsFunc(instances, func(c sablier.InstanceConfiguration) bool { return c.Name == name })
}

func TestECSProvider_Integration(t *testing.T) {
	client := setupLocalStack(t)
	runECSIntegration(t, client)
}

// runECSIntegration runs the provider against an ECS API with the real SDK client.
func runECSIntegration(t *testing.T, client *ecs.Client) {
	env := newECSEnv(t, client)
	ctx := t.Context()

	t.Run("MissingCluster", func(t *testing.T) {
		_, err := awsecs.New(ctx, client, uniqueName("sablier-it-missing"), slogt.New(t))
		assert.ErrorContains(t, err, "not found")
	})

	p, err := awsecs.NewForTest(ctx, client, env.cluster, slogt.New(t), itPollInterval)
	assert.NilError(t, err)

	t.Run("ListAndInspect", func(t *testing.T) {
		name := uniqueName("sablier-it-list")
		arn := env.createService(t, name, map[string]string{
			sablier.LabelEnable: "true",
			sablier.LabelGroup:  "it-a it-b",
		})
		ignored := uniqueName("sablier-it-ignored")
		env.createService(t, ignored, nil)

		instances, err := p.InstanceList(ctx, provider.InstanceListOptions{All: true})
		assert.NilError(t, err)
		i := slices.IndexFunc(instances, func(c sablier.InstanceConfiguration) bool { return c.Name == name })
		assert.Assert(t, i >= 0, "expected %q in the instance list %v", name, instances)
		assert.DeepEqual(t, instances[i].Groups, []string{"it-a", "it-b"})
		assert.Equal(t, instances[i].Enabled, "true")
		assert.Assert(t, !hasInstance(instances, ignored), "a service without the enable tag is not listed")

		running, err := p.InstanceList(ctx, provider.InstanceListOptions{All: false})
		assert.NilError(t, err)
		assert.Assert(t, !hasInstance(running, name), "a service with no desired task is not running")

		groups, err := p.InstanceGroups(ctx)
		assert.NilError(t, err)
		assert.Assert(t, slices.Contains(groups["it-b"], name), "expected %q in group it-b", name)

		info, err := p.InstanceInspect(ctx, arn)
		assert.NilError(t, err)
		assert.Equal(t, info.Name, name)
		assert.Equal(t, info.Status, sablier.InstanceStatusStopped)
		assert.Equal(t, info.Provider, sablier.ProviderECS)
		assert.Assert(t, info.IsEnabled())
	})

	t.Run("StartStopWithEvents", func(t *testing.T) {
		name := uniqueName("sablier-it-web")
		env.createService(t, name, map[string]string{sablier.LabelEnable: "true"})

		stream := subscribe(t, p, provider.InstanceEventsOptions{})
		env.waitStreamLive(t, stream.Events)

		assert.NilError(t, p.InstanceStart(ctx, name))
		waitEvent(t, stream.Events, name, provider.InstanceEventStarted, 30*time.Second)
		info := waitStatus(t, p, name, sablier.InstanceStatusReady, 3*time.Minute)
		assert.Equal(t, info.CurrentReplicas, int32(1))
		assert.Equal(t, info.DesiredReplicas, int32(1))

		running, err := p.InstanceList(ctx, provider.InstanceListOptions{All: false})
		assert.NilError(t, err)
		assert.Assert(t, hasInstance(running, name), "a started service is running")

		// Starting again does not update the service.
		assert.NilError(t, p.InstanceStart(ctx, name))

		assert.NilError(t, p.InstanceStop(ctx, name))
		waitEvent(t, stream.Events, name, provider.InstanceEventStopped, 30*time.Second)
		waitStatus(t, p, name, sablier.InstanceStatusStopped, time.Minute)
	})
}
