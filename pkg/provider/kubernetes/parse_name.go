package kubernetes

import (
	"fmt"
	"strconv"
	"strings"

	v1 "k8s.io/api/apps/v1"
)

type ParsedName struct {
	Original  string
	Kind      string // deployment or statefulset
	Namespace string
	Name      string
	Replicas  int32
}

type ParseOptions struct {
	Delimiter string
}

func ParseName(name string, opts ParseOptions) (ParsedName, error) {

	split := strings.Split(name, opts.Delimiter)
	if len(split) != 4 {
		return ParsedName{}, fmt.Errorf("invalid name [%s] should be: kind%snamespace%sname%sreplicas", name, opts.Delimiter, opts.Delimiter, opts.Delimiter)
	}

	// Parse as 32 bits: replicas are an int32 in the Kubernetes API, and a
	// larger value must fail rather than wrap around.
	replicas, err := strconv.ParseInt(split[3], 10, 32)
	if err != nil {
		return ParsedName{}, fmt.Errorf("invalid replica count in [%s]: %w", name, err)
	}
	if replicas < 0 {
		return ParsedName{}, fmt.Errorf("invalid replica count in [%s]: must not be negative", name)
	}

	return ParsedName{
		Original:  name,
		Kind:      split[0],
		Namespace: split[1],
		Name:      split[2],
		Replicas:  int32(replicas),
	}, nil
}

func DeploymentName(deployment *v1.Deployment, opts ParseOptions) ParsedName {
	kind := "deployment"
	namespace := deployment.Namespace
	name := deployment.Name
	// TOOD: Use annotation for scale
	original := fmt.Sprintf("%s%s%s%s%s%s%d", kind, opts.Delimiter, namespace, opts.Delimiter, name, opts.Delimiter, 1)

	return ParsedName{
		Original:  original,
		Kind:      kind,
		Namespace: namespace,
		Name:      name,
		Replicas:  1,
	}
}

func StatefulSetName(statefulSet *v1.StatefulSet, opts ParseOptions) ParsedName {
	kind := "statefulset"
	namespace := statefulSet.Namespace
	name := statefulSet.Name
	// TOOD: Use annotation for scale
	original := fmt.Sprintf("%s%s%s%s%s%s%d", kind, opts.Delimiter, namespace, opts.Delimiter, name, opts.Delimiter, 1)

	return ParsedName{
		Original:  original,
		Kind:      kind,
		Namespace: namespace,
		Name:      name,
		Replicas:  1,
	}
}
