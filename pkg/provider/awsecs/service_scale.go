package awsecs

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"

	"github.com/sablierapp/sablier/pkg/sablier"
)

// updateDesiredCount scales the service to the replicas of the profile. The
// CPU, memory and blkio values of the profile are ignored because on ECS they
// belong to the task definition. The update is skipped when the service
// already has the wanted desired count.
func (p *Provider) updateDesiredCount(ctx context.Context, svc *types.Service, profile sablier.ResourceProfile) error {
	if err := checkReplica(svc); err != nil {
		return err
	}

	name := aws.ToString(svc.ServiceName)
	if profile.HasResources() {
		p.l.WarnContext(ctx, "cpu, memory and blkio labels are not supported on ECS, only the replicas are applied",
			slog.String("service", name))
	}
	if svc.DesiredCount == profile.Replicas {
		p.l.DebugContext(ctx, "service already has the wanted desired count, skipping update",
			slog.String("service", name),
			slog.Int("desired_count", int(profile.Replicas)))
		return nil
	}

	_, err := p.client.UpdateService(ctx, &ecs.UpdateServiceInput{
		Cluster:      aws.String(p.cluster),
		Service:      svc.ServiceArn,
		DesiredCount: aws.Int32(profile.Replicas),
	})
	if err != nil {
		return fmt.Errorf("cannot update service %q desired count: %w", name, err)
	}
	return nil
}
