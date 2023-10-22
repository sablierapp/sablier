package kubernetes

import (
	"context"

	appsv1 "k8s.io/api/apps/v1"

	"github.com/acouvreur/sablier/internal/provider"
	core_v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"
)

func (client *Client) Events(ctx context.Context) (<-chan provider.Message, <-chan error) {
	messages := make(chan provider.Message)
	closed := make(chan error, 1)

	go client.watchDeployents(messages).Run(ctx.Done())

	return messages, closed
}

func (provider *Client) SubscribeOnce(ctx context.Context, name string, action provider.EventAction, wait chan<- error) {
	// TODO
}

func (client *Client) watchDeployents(messages chan provider.Message) cache.SharedIndexInformer {
	handler := cache.ResourceEventHandlerFuncs{
		UpdateFunc: func(old, new interface{}) {
			newDeployment := new.(*appsv1.Deployment)
			oldDeployment := old.(*appsv1.Deployment)

			if *oldDeployment.Spec.Replicas > 0 && *newDeployment.Spec.Replicas == 0 {
				parsedName := DeploymentName(*newDeployment, client.ParseOptions)
				messages <- provider.Message{
					Name:   parsedName.Original,
					Action: provider.EventActionStop,
				}
			}

			if *oldDeployment.Spec.Replicas == 0 && *newDeployment.Spec.Replicas > 0 {
				parsedName := DeploymentName(*newDeployment, client.ParseOptions)
				messages <- provider.Message{
					Name:   parsedName.Original,
					Action: provider.EventActionStart,
				}
			}
		},
		DeleteFunc: func(obj interface{}) {
			deletedDeployment := obj.(*appsv1.Deployment)
			parsedName := DeploymentName(*deletedDeployment, client.ParseOptions)
			messages <- provider.Message{
				Name:   parsedName.Original,
				Action: provider.EventActionDestroy,
			}
		},
		AddFunc: func(obj interface{}) {
			addedDeployment := obj.(*appsv1.Deployment)
			parsedName := DeploymentName(*addedDeployment, client.ParseOptions)
			messages <- provider.Message{
				Name:   parsedName.Original,
				Action: provider.EventActionCreate,
			}
		},
	}

	factory := informers.NewSharedInformerFactoryWithOptions(client.Client, client.defaultResync, informers.WithNamespace(core_v1.NamespaceAll))
	deploymentInformer := factory.Apps().V1().Deployments().Informer()

	deploymentInformer.AddEventHandler(handler)
	return deploymentInformer
}
