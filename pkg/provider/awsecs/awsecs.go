package awsecs

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"

	"github.com/sablierapp/sablier/pkg/sablier"
)

// Interface guard
var _ sablier.Provider = (*Provider)(nil)

// API is the subset of the ECS client the provider uses. *ecs.Client satisfies it.
type API interface {
	DescribeClusters(ctx context.Context, params *ecs.DescribeClustersInput, optFns ...func(*ecs.Options)) (*ecs.DescribeClustersOutput, error)
	ListServices(ctx context.Context, params *ecs.ListServicesInput, optFns ...func(*ecs.Options)) (*ecs.ListServicesOutput, error)
	DescribeServices(ctx context.Context, params *ecs.DescribeServicesInput, optFns ...func(*ecs.Options)) (*ecs.DescribeServicesOutput, error)
	UpdateService(ctx context.Context, params *ecs.UpdateServiceInput, optFns ...func(*ecs.Options)) (*ecs.UpdateServiceOutput, error)
}

// describeServicesBatchSize is the maximum number of services one
// DescribeServices call accepts.
// https://docs.aws.amazon.com/AmazonECS/latest/APIReference/API_DescribeServices.html
const describeServicesBatchSize = 10

const (
	clusterStatusActive   = "ACTIVE"
	serviceStatusInactive = "INACTIVE"
	deploymentPrimary     = "PRIMARY"
)

// Provider implements sablier.Provider for Amazon ECS services in one cluster.
type Provider struct {
	client       API
	cluster      string
	l            *slog.Logger
	tracer       trace.Tracer
	pollInterval time.Duration
}

// New creates the ECS provider and verifies that the cluster exists and is active.
func New(ctx context.Context, client API, cluster string, logger *slog.Logger) (*Provider, error) {
	logger = logger.With(slog.String("provider", "ecs"))

	out, err := client.DescribeClusters(ctx, &ecs.DescribeClustersInput{Clusters: []string{cluster}})
	if err != nil {
		return nil, fmt.Errorf("cannot connect to the ECS API: %w", err)
	}
	if len(out.Clusters) == 0 {
		return nil, fmt.Errorf("ECS cluster %q not found: %s", cluster, failureDetails(out.Failures))
	}
	c := out.Clusters[0]
	if status := aws.ToString(c.Status); status != clusterStatusActive {
		return nil, fmt.Errorf("ECS cluster %q is %s, expected %s", cluster, status, clusterStatusActive)
	}

	logger.InfoContext(ctx, "connection established with ECS",
		slog.String("cluster", aws.ToString(c.ClusterName)),
		slog.String("cluster_arn", aws.ToString(c.ClusterArn)),
	)

	return &Provider{
		client:       client,
		cluster:      cluster,
		l:            logger,
		tracer:       otel.Tracer("github.com/sablierapp/sablier/pkg/provider/awsecs"),
		pollInterval: 10 * time.Second,
	}, nil
}

// describeService returns the service called name (a name or an ARN) with its tags.
func (p *Provider) describeService(ctx context.Context, name string) (*types.Service, error) {
	out, err := p.client.DescribeServices(ctx, &ecs.DescribeServicesInput{
		Cluster:  aws.String(p.cluster),
		Services: []string{name},
		Include:  []types.ServiceField{types.ServiceFieldTags},
	})
	if err != nil {
		return nil, fmt.Errorf("cannot describe service %q: %w", name, err)
	}
	if len(out.Services) == 0 {
		return nil, fmt.Errorf("service %q not found in cluster %q: %s", name, p.cluster, failureDetails(out.Failures))
	}
	svc := out.Services[0]
	if aws.ToString(svc.Status) == serviceStatusInactive {
		return nil, fmt.Errorf("service %q not found in cluster %q: service is inactive", name, p.cluster)
	}

	p.l.DebugContext(ctx, "service described",
		slog.String("service", aws.ToString(svc.ServiceName)),
		slog.Int("desired_count", int(svc.DesiredCount)),
		slog.Int("running_count", int(svc.RunningCount)),
		slog.Int("pending_count", int(svc.PendingCount)),
	)
	return &svc, nil
}

// listEnabledServices returns the replica services of the cluster tagged
// sablier.enable=true, with their tags.
func (p *Provider) listEnabledServices(ctx context.Context) ([]types.Service, error) {
	var arns []string
	paginator := ecs.NewListServicesPaginator(p.client, &ecs.ListServicesInput{
		Cluster:            aws.String(p.cluster),
		SchedulingStrategy: types.SchedulingStrategyReplica,
	})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("cannot list services: %w", err)
		}
		arns = append(arns, page.ServiceArns...)
	}

	services := make([]types.Service, 0, len(arns))
	for batch := range slices.Chunk(arns, describeServicesBatchSize) {
		out, err := p.client.DescribeServices(ctx, &ecs.DescribeServicesInput{
			Cluster:  aws.String(p.cluster),
			Services: batch,
			Include:  []types.ServiceField{types.ServiceFieldTags},
		})
		if err != nil {
			return nil, fmt.Errorf("cannot describe services: %w", err)
		}
		for _, f := range out.Failures {
			p.l.WarnContext(ctx, "cannot describe service, skipping",
				slog.String("arn", aws.ToString(f.Arn)),
				slog.String("reason", aws.ToString(f.Reason)),
			)
		}
		for _, svc := range out.Services {
			if aws.ToString(svc.Status) == serviceStatusInactive {
				continue
			}
			if svc.SchedulingStrategy != types.SchedulingStrategyReplica {
				continue
			}
			if tagsToLabels(svc.Tags)[sablier.LabelEnable] != "true" {
				continue
			}
			services = append(services, svc)
		}
	}

	p.l.DebugContext(ctx, "services listed", slog.Int("count", len(services)))
	return services, nil
}

// failureDetails formats the API failures of a describe call for an error message.
func failureDetails(failures []types.Failure) string {
	if len(failures) == 0 {
		return "no details"
	}
	parts := make([]string, 0, len(failures))
	for _, f := range failures {
		parts = append(parts, fmt.Sprintf("%s: %s", aws.ToString(f.Arn), aws.ToString(f.Reason)))
	}
	return strings.Join(parts, ", ")
}
