package kubernetes

import (
	"context"
	"fmt"
	"time"

	"github.com/acouvreur/sablier/internal/provider"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type Client struct {
	Client       kubernetes.Interface
	ParseOptions ParseOptions

	defaultResync time.Duration
}

type Workload interface {
	GetScale(ctx context.Context, workloadName string, options metav1.GetOptions) (*autoscalingv1.Scale, error)
	UpdateScale(ctx context.Context, workloadName string, scale *autoscalingv1.Scale, opts metav1.UpdateOptions) (*autoscalingv1.Scale, error)
}

func NewKubernetesClient() (*Client, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}
	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	// TODO: Set this as a config
	parseOpts := ParseOptions{
		Delimiter: ".",
	}

	return &Client{
		Client:       client,
		ParseOptions: parseOpts,
	}, nil
}

func (client *Client) Start(ctx context.Context, name string, opts provider.StartOptions) error {
	parsed, err := ParseName(name, client.ParseOptions)
	if err != nil {
		return err
	}

	workload, err := client.workload(parsed)
	if err != nil {
		return err
	}

	scale, err := workload.GetScale(ctx, parsed.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	scale.Spec.Replicas = int32(opts.DesiredReplicas)
	_, err = workload.UpdateScale(ctx, parsed.Name, scale, metav1.UpdateOptions{})
	if err != nil {
		return err
	}

	return nil
}

func (client *Client) workload(config ParsedName) (Workload, error) {
	var workload Workload

	switch config.Kind {
	case "deployment":
		workload = client.Client.AppsV1().Deployments(config.Namespace)
	case "statefulset":
		workload = client.Client.AppsV1().StatefulSets(config.Namespace)
	default:
		return nil, fmt.Errorf("unsupported kind %s", config.Kind)
	}

	return workload, nil
}

func (client *Client) Stop(ctx context.Context, name string) error {
	parsed, err := ParseName(name, client.ParseOptions)
	if err != nil {
		return err
	}

	workload, err := client.workload(parsed)
	if err != nil {
		return err
	}

	scale, err := workload.GetScale(ctx, parsed.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	scale.Spec.Replicas = int32(0)
	_, err = workload.UpdateScale(ctx, parsed.Name, scale, metav1.UpdateOptions{})
	if err != nil {
		return err
	}

	return nil
}

func (client *Client) Status(ctx context.Context, name string) (bool, error) {
	parsed, err := ParseName(name, client.ParseOptions)
	if err != nil {
		return false, err
	}

	switch parsed.Kind {
	case "deployment":
		return client.DeploymentStatus(ctx, parsed)
	case "statefulset":
		return client.StatefulSetStatus(ctx, parsed)
	default:
		return false, fmt.Errorf("unsupported kind %s", parsed.Kind)
	}
}

func (client *Client) Discover(ctx context.Context, opts provider.DiscoveryOptions) ([]provider.Discovered, error) {
	discoveredDeployments, err := client.discoverDeployments(ctx, opts)
	if err != nil {
		return nil, err
	}

	discoveredStatefulSets, err := client.discoverStatefulSets(ctx, opts)
	if err != nil {
		return nil, err
	}

	discovered := append(discoveredDeployments, discoveredStatefulSets...)
	return discovered, nil
}
