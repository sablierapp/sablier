package awsecs_test

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"

	"github.com/sablierapp/sablier/pkg/provider/awsecs"
)

const testCluster = "test-cluster"

// fakeECS is an in-memory ECS API that keeps services by name.
type fakeECS struct {
	mu            sync.Mutex
	clusterStatus string
	services      map[string]*types.Service
	pageSize      int
	listErr       error
	describeErr   error
	updateErr     error
	updates       []ecs.UpdateServiceInput
	calls         map[string]int
}

var _ awsecs.API = (*fakeECS)(nil)

func newFake(services ...*types.Service) *fakeECS {
	f := &fakeECS{
		clusterStatus: "ACTIVE",
		services:      make(map[string]*types.Service, len(services)),
		pageSize:      100,
		calls:         make(map[string]int),
	}
	for _, s := range services {
		f.services[aws.ToString(s.ServiceName)] = s
	}
	return f
}

func serviceArn(name string) string {
	return "arn:aws:ecs:eu-west-1:123456789012:service/" + testCluster + "/" + name
}

// newService builds a replica service with the given counts and tags.
func newService(name string, desired, running int32, tags map[string]string) *types.Service {
	svc := &types.Service{
		ServiceArn:         aws.String(serviceArn(name)),
		ServiceName:        aws.String(name),
		ClusterArn:         aws.String("arn:aws:ecs:eu-west-1:123456789012:cluster/" + testCluster),
		Status:             aws.String("ACTIVE"),
		DesiredCount:       desired,
		RunningCount:       running,
		SchedulingStrategy: types.SchedulingStrategyReplica,
		TaskDefinition:     aws.String("arn:aws:ecs:eu-west-1:123456789012:task-definition/" + name + ":1"),
	}
	for k, v := range tags {
		svc.Tags = append(svc.Tags, types.Tag{Key: aws.String(k), Value: aws.String(v)})
	}
	slices.SortFunc(svc.Tags, func(a, b types.Tag) int {
		return strings.Compare(aws.ToString(a.Key), aws.ToString(b.Key))
	})
	return svc
}

func withDeployment(svc *types.Service, state types.DeploymentRolloutState, reason string) *types.Service {
	svc.Deployments = []types.Deployment{{
		Id:                 aws.String("ecs-svc/1"),
		Status:             aws.String("PRIMARY"),
		RolloutState:       state,
		RolloutStateReason: aws.String(reason),
		DesiredCount:       svc.DesiredCount,
		RunningCount:       svc.RunningCount,
	}}
	return svc
}

func (f *fakeECS) set(svc *types.Service) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.services[aws.ToString(svc.ServiceName)] = svc
}

func (f *fakeECS) remove(name string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.services, name)
}

func (f *fakeECS) setListErr(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.listErr = err
}

func (f *fakeECS) callCount(op string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls[op]
}

func (f *fakeECS) recordedUpdates() []ecs.UpdateServiceInput {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.updates)
}

func (f *fakeECS) checkCluster(cluster *string) error {
	if aws.ToString(cluster) != testCluster {
		return fmt.Errorf("ClusterNotFoundException: cluster %q not found", aws.ToString(cluster))
	}
	return nil
}

func (f *fakeECS) DescribeClusters(_ context.Context, in *ecs.DescribeClustersInput, _ ...func(*ecs.Options)) (*ecs.DescribeClustersOutput, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls["DescribeClusters"]++
	if f.describeErr != nil {
		return nil, f.describeErr
	}
	out := &ecs.DescribeClustersOutput{}
	for _, c := range in.Clusters {
		if c != testCluster {
			out.Failures = append(out.Failures, types.Failure{Arn: aws.String(c), Reason: aws.String("MISSING")})
			continue
		}
		out.Clusters = append(out.Clusters, types.Cluster{
			ClusterName: aws.String(testCluster),
			ClusterArn:  aws.String("arn:aws:ecs:eu-west-1:123456789012:cluster/" + testCluster),
			Status:      aws.String(f.clusterStatus),
		})
	}
	return out, nil
}

func (f *fakeECS) ListServices(_ context.Context, in *ecs.ListServicesInput, _ ...func(*ecs.Options)) (*ecs.ListServicesOutput, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls["ListServices"]++
	if f.listErr != nil {
		return nil, f.listErr
	}
	if err := f.checkCluster(in.Cluster); err != nil {
		return nil, err
	}
	var names []string
	for name, svc := range f.services {
		if in.SchedulingStrategy != "" && svc.SchedulingStrategy != in.SchedulingStrategy {
			continue
		}
		names = append(names, name)
	}
	slices.Sort(names)

	start := 0
	if in.NextToken != nil {
		start, _ = strconv.Atoi(aws.ToString(in.NextToken))
	}
	end := min(start+f.pageSize, len(names))
	out := &ecs.ListServicesOutput{}
	for _, name := range names[start:end] {
		out.ServiceArns = append(out.ServiceArns, serviceArn(name))
	}
	if end < len(names) {
		out.NextToken = aws.String(strconv.Itoa(end))
	}
	return out, nil
}

func (f *fakeECS) DescribeServices(_ context.Context, in *ecs.DescribeServicesInput, _ ...func(*ecs.Options)) (*ecs.DescribeServicesOutput, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls["DescribeServices"]++
	if f.describeErr != nil {
		return nil, f.describeErr
	}
	if err := f.checkCluster(in.Cluster); err != nil {
		return nil, err
	}
	if len(in.Services) > 10 {
		return nil, errors.New("InvalidParameterException: services can have at most 10 items")
	}
	withTags := slices.Contains(in.Include, types.ServiceFieldTags)
	out := &ecs.DescribeServicesOutput{}
	for _, id := range in.Services {
		svc := f.find(id)
		if svc == nil {
			out.Failures = append(out.Failures, types.Failure{Arn: aws.String(id), Reason: aws.String("MISSING")})
			continue
		}
		clone := *svc
		if !withTags {
			clone.Tags = nil
		}
		out.Services = append(out.Services, clone)
	}
	return out, nil
}

// find matches a service by name or ARN. The caller holds the lock.
func (f *fakeECS) find(id string) *types.Service {
	if svc, ok := f.services[id]; ok {
		return svc
	}
	for _, svc := range f.services {
		if aws.ToString(svc.ServiceArn) == id {
			return svc
		}
	}
	return nil
}

func (f *fakeECS) UpdateService(_ context.Context, in *ecs.UpdateServiceInput, _ ...func(*ecs.Options)) (*ecs.UpdateServiceOutput, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls["UpdateService"]++
	f.updates = append(f.updates, *in)
	if f.updateErr != nil {
		return nil, f.updateErr
	}
	if err := f.checkCluster(in.Cluster); err != nil {
		return nil, err
	}
	svc := f.find(aws.ToString(in.Service))
	if svc == nil {
		return nil, fmt.Errorf("ServiceNotFoundException: service %q not found", aws.ToString(in.Service))
	}
	if in.DesiredCount != nil {
		svc.DesiredCount = *in.DesiredCount
	}
	clone := *svc
	return &ecs.UpdateServiceOutput{Service: &clone}, nil
}
