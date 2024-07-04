package kubernetes

import (
	"fmt"
	"strings"

	v1 "k8s.io/api/apps/v1"
)

type ParsedName struct {
	Original  string
	Kind      string // deployment or statefulset
	Namespace string
	Name      string
}

type ParseOptions struct {
	Delimiter string
}

func ParseName(name string, opts ParseOptions) (ParsedName, error) {

	split := strings.Split(name, opts.Delimiter)
	if len(split) < 3 {
		return ParsedName{}, fmt.Errorf("invalid name should be: kind%snamespace%sname (have %s)", opts.Delimiter, opts.Delimiter, name)
	}

	return ParsedName{
		Original:  name,
		Kind:      split[0],
		Namespace: split[1],
		Name:      split[2],
	}, nil
}

func DeploymentName(deployment v1.Deployment, opts ParseOptions) ParsedName {
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
	}
}

func StatefulSetName(statefulSet v1.StatefulSet, opts ParseOptions) ParsedName {
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
	}
}
