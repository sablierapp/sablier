package scaler

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	log "github.com/sirupsen/logrus"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// Delimiter is used to split name into kind,namespace,name,replicacount
const Delimiter = "_"

type Config struct {
	Kind      string // deployment or statefulset
	Namespace string
	Name      string
	Replicas  int
}

type Workload interface {
	GetScale(ctx context.Context, workloadName string, options metav1.GetOptions) (*autoscalingv1.Scale, error)
	UpdateScale(ctx context.Context, workloadName string, scale *autoscalingv1.Scale, opts metav1.UpdateOptions) (*autoscalingv1.Scale, error)
}

func convertName(name string) (*Config, error) {
	// name format kind_namespace_name_replicas
	s := strings.Split(name, Delimiter)
	if len(s) < 4 {
		return nil, errors.New("invalid name should be: kind" + Delimiter + "namespace" + Delimiter + "name" + Delimiter + "replicas")
	}
	replicas, err := strconv.Atoi(s[3])
	if err != nil {
		return nil, err
	}

	return &Config{
		Kind:      s[0],
		Namespace: s[1],
		Name:      s[2],
		Replicas:  replicas,
	}, nil
}

type KubernetesScaler struct {
	Client *kubernetes.Clientset
}

func NewKubernetesScaler(client *kubernetes.Clientset) *KubernetesScaler {
	return &KubernetesScaler{
		Client: client,
	}
}

func (scaler *KubernetesScaler) ScaleUp(name string) error {
	config, err := convertName(name)
	if err != nil {
		log.Error(err.Error())
		return err
	}

	log.Infof("Scaling up %s %s in namespace %s to %d", config.Kind, config.Name, config.Namespace, config.Replicas)
	ctx := context.Background()

	var workload Workload

	switch config.Kind {
	case "deployment":
		workload = scaler.Client.AppsV1().Deployments(config.Namespace)
	case "statefulset":
		workload = scaler.Client.AppsV1().StatefulSets(config.Namespace)
	default:
		return fmt.Errorf("unsupported kind %s", config.Kind)
	}

	s, err := workload.GetScale(ctx, config.Name, metav1.GetOptions{})
	if err != nil {
		log.Error(err.Error())
		return err
	}

	sc := *s
	if sc.Spec.Replicas == 0 {
		sc.Spec.Replicas = int32(config.Replicas)
	} else {
		log.Infof("Replicas for %s %s in namespace %s are already: %d", config.Kind, config.Name, config.Namespace, sc.Spec.Replicas)
		return nil
	}

	_, err = workload.UpdateScale(ctx, config.Name, &sc, metav1.UpdateOptions{})

	if err != nil {
		log.Error(err.Error())
		return err
	}

	return nil
}

func (scaler *KubernetesScaler) ScaleDown(name string) error {
	config, err := convertName(name)
	if err != nil {
		log.Error(err.Error())
		return err
	}

	log.Infof("Scaling down %s %s in namespace %s to 0", config.Kind, config.Name, config.Namespace)
	ctx := context.Background()

	var workload Workload

	switch config.Kind {
	case "deployment":
		workload = scaler.Client.AppsV1().Deployments(config.Namespace)
	case "statefulset":
		workload = scaler.Client.AppsV1().StatefulSets(config.Namespace)
	default:
		return fmt.Errorf("unsupported kind %s", config.Kind)
	}

	s, err := workload.GetScale(ctx, config.Name, metav1.GetOptions{})
	if err != nil {
		log.Error(err.Error())
		return err
	}

	sc := *s
	if sc.Spec.Replicas != 0 {
		sc.Spec.Replicas = 0
	} else {
		log.Infof("Replicas for %s %s in namespace %s are already: 0", config.Kind, config.Name, config.Namespace)
		return nil
	}

	_, err = workload.UpdateScale(ctx, config.Name, &sc, metav1.UpdateOptions{})

	if err != nil {
		log.Error(err.Error())
		return err
	}

	return nil
}

func (scaler *KubernetesScaler) IsUp(name string) bool {
	ctx := context.Background()

	config, err := convertName(name)
	if err != nil {
		log.Error(err.Error())
		return false
	}

	switch config.Kind {
	case "deployment":
		return scaler.isDeploymentUp(ctx, config)
	case "statefulset":
		d, err := scaler.Client.AppsV1().StatefulSets(config.Namespace).
			Get(ctx, config.Name, metav1.GetOptions{})
		if err != nil {
			log.Error(err.Error())
			return false
		}
		log.Infof("Status for %s %s in namespace %s is: AvailableReplicas %d, ReadyReplicas: %d ", config.Kind, config.Name, config.Namespace, d.Status.AvailableReplicas, d.Status.ReadyReplicas)

		if d.Status.ReadyReplicas > 0 {
			return true
		}

	default:
		log.Error(fmt.Errorf("unsupported kind %s", config.Kind))
		return false
	}

	return false
}

func (scaler *KubernetesScaler) isDeploymentUp(ctx context.Context, config *Config) bool {
	deployment, err := scaler.Client.AppsV1().Deployments(config.Namespace).
		Get(ctx, config.Name, metav1.GetOptions{})
	if err != nil {
		log.Error(err.Error())
		return false
	}
	log.Infof("Status for %s %s in namespace %s is: AvailableReplicas %d, ReadyReplicas: %d ", config.Kind, config.Name, config.Namespace, deployment.Status.AvailableReplicas, deployment.Status.ReadyReplicas)

	endpoints, err := scaler.getEndpointsByLabelSelector(ctx, *config, "app", deployment.Spec.Template.Labels["app"])
	if err != nil {
		log.Error(err.Error())
		return false
	}

	if deployment.Status.ReadyReplicas > 0 && endpointsHasAtLeastNReadyAddresses(endpoints, 1) {
		return true
	}
	return false
}

func (scaler *KubernetesScaler) getEndpointsByLabelSelector(ctx context.Context, config Config, selectorKey, selectorValue string) (*[]v1.Endpoints, error) {
	// No filter because filter is not supported on Spec.Selector map
	services, err := scaler.Client.CoreV1().Services(config.Namespace).
		List(ctx, metav1.ListOptions{})

	if err != nil {
		log.Error(err.Error())
		return nil, err
	}

	var foundServices []v1.Service
	for _, service := range services.Items {
		if service.Spec.Selector["app"] == selectorValue {
			foundServices = append(foundServices, service)
		}
	}

	if len(foundServices) == 0 {
		log.Error("No service found with label selector " + selectorKey + "=" + selectorValue)
		return nil, fmt.Errorf("No service found with label selector " + selectorKey + "=" + selectorValue)
	}

	serviceNames := make(map[string]bool)
	for _, item := range services.Items {
		serviceNames[item.Name] = true
	}

	endpoints, err := scaler.Client.CoreV1().Endpoints(config.Namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		log.Error(err.Error())
		return nil, err
	}

	if len(endpoints.Items) == 0 {
		log.Error("No endpoint found")
		return nil, fmt.Errorf("no endpoint found")
	}

	var foundEndpoints []v1.Endpoints
	for _, endpoint := range endpoints.Items {
		if serviceNames[endpoint.ObjectMeta.Name] {
			foundEndpoints = append(foundEndpoints, endpoint)
		}
	}
	return &foundEndpoints, nil
}

func endpointsHasAtLeastNReadyAddresses(endpoints *[]v1.Endpoints, n int) bool {

	if len(*endpoints) == 0 {
		log.Infof("No endpoint available")
		return false
	}

	for _, endpoint := range *endpoints {
		if endpointHasAtLeastNReadyAddresses(&endpoint, n) {
			log.Infof("Endpoint %s has at least one IP!", &endpoint.Name)
			return true
		}
	}

	log.Infof("All %d endpoint(s) had no available IP", len(*endpoints))
	return false
}

func endpointHasAtLeastNReadyAddresses(endpoint *v1.Endpoints, n int) bool {

	if len(endpoint.Subsets) == 0 {
		return false
	}

	availableAddressesCount := 0
	for _, subset := range endpoint.Subsets {
		availableAddressesCount += len(subset.Addresses)
	}

	return availableAddressesCount > 0
}
