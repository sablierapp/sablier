package kubernetes

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (client *Client) watchDeployment(ctx context.Context, deployment *appsv1.Deployment) error {
	labelSelector := fmt.Sprintf("app.kubernetes.io/instance=%s", deployment.Name)

	opts := metav1.ListOptions{
		TypeMeta:      metav1.TypeMeta{},
		LabelSelector: labelSelector,
		FieldSelector: "",
	}

	watcher, err := client.Client.AppsV1().Deployments(deployment.Namespace).Watch(ctx, opts)
	if err != nil {
		return err
	}

	defer watcher.Stop()

	for {
		select {
		case event := <-watcher.ResultChan():
			deployment := event.Object.(*appsv1.Deployment)
			if *deployment.Spec.Replicas == deployment.Status.ReadyReplicas {
				return nil
			}
		case <-ctx.Done():
			return nil
		}
	}
}
