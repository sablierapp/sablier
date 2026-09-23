package awsecs

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"

	"github.com/sablierapp/sablier/pkg/sablier"
)

func (p *Provider) InstanceInspect(ctx context.Context, name string) (sablier.InstanceInfo, error) {
	svc, err := p.describeService(ctx, name)
	if err != nil {
		return sablier.InstanceInfo{}, err
	}
	return serviceToInfo(svc)
}

// checkReplica rejects the services that have no desired count to scale.
func checkReplica(svc *types.Service) error {
	if svc.SchedulingStrategy == types.SchedulingStrategyDaemon {
		return fmt.Errorf("service %q uses the DAEMON scheduling strategy, only REPLICA services can be scaled", aws.ToString(svc.ServiceName))
	}
	return nil
}

// serviceToInfo maps the ECS service state to the sablier instance state.
// A service with no desired task is stopped. A failed primary deployment is
// an error. Fewer running than desired tasks, or a primary deployment still
// in progress (tasks not healthy yet), is starting. Anything else is ready.
func serviceToInfo(svc *types.Service) (sablier.InstanceInfo, error) {
	if err := checkReplica(svc); err != nil {
		return sablier.InstanceInfo{}, err
	}

	labels := tagsToLabels(svc.Tags)
	sc := sablier.ScaleConfigFromLabels(labels)

	info := sablier.InstanceInfo{
		Name:            aws.ToString(svc.ServiceName),
		CurrentReplicas: svc.RunningCount,
		DesiredReplicas: sc.Active.Replicas,
		Provider:        sablier.ProviderECS,
	}

	primary := primaryDeployment(svc.Deployments)
	switch {
	case svc.DesiredCount == 0:
		info.CurrentReplicas = 0
		info.Status = sablier.InstanceStatusStopped
	case primary != nil && primary.RolloutState == types.DeploymentRolloutStateFailed:
		info.Status = sablier.InstanceStatusError
		info.Message = fmt.Sprintf("deployment %s failed: %s", aws.ToString(primary.Id), aws.ToString(primary.RolloutStateReason))
	case svc.RunningCount < svc.DesiredCount:
		info.Status = sablier.InstanceStatusStarting
	case primary != nil && primary.RolloutState == types.DeploymentRolloutStateInProgress:
		info.Status = sablier.InstanceStatusStarting
	default:
		info.Status = sablier.InstanceStatusReady
	}

	sablier.PopulateEnabledAndGroup(&info, labels)
	return info, nil
}

// primaryDeployment returns the deployment that runs the current task definition.
func primaryDeployment(deployments []types.Deployment) *types.Deployment {
	for i := range deployments {
		if aws.ToString(deployments[i].Status) == deploymentPrimary {
			return &deployments[i]
		}
	}
	return nil
}
