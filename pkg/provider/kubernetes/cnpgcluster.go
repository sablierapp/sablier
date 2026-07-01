package kubernetes

import (
	"fmt"

	"k8s.io/apimachinery/pkg/runtime/schema"
)

// KindCNPGCluster is the workload kind used in instance names to identify a
// CloudNativePG Cluster custom resource (e.g. "cnpgcluster_namespace_name_1").
const KindCNPGCluster = "cnpgcluster"

// cnpgClusterGVR is the GroupVersionResource of CloudNativePG Cluster resources.
// See https://cloudnative-pg.io/documentation/current/cloudnative-pg.v1/#postgresql-cnpg-io-v1-Cluster
var cnpgClusterGVR = schema.GroupVersionResource{
	Group:    "postgresql.cnpg.io",
	Version:  "v1",
	Resource: "clusters",
}

// cnpgHibernationAnnotation is the annotation CloudNativePG watches to declaratively
// hibernate ("on") or resume ("off") a Cluster. Hibernation scales the cluster down
// and deletes the underlying resources while keeping the PVCs intact.
// See https://cloudnative-pg.io/documentation/current/declarative_hibernation/
const cnpgHibernationAnnotation = "cnpg.io/hibernation"

const (
	cnpgHibernationOn  = "on"
	cnpgHibernationOff = "off"
)

// ClusterName builds the ParsedName identifier for a CloudNativePG Cluster,
// mirroring DeploymentName and StatefulSetName.
func ClusterName(namespace, name string, opts ParseOptions) ParsedName {
	original := fmt.Sprintf("%s%s%s%s%s%s%d", KindCNPGCluster, opts.Delimiter, namespace, opts.Delimiter, name, opts.Delimiter, 1)

	return ParsedName{
		Original:  original,
		Kind:      KindCNPGCluster,
		Namespace: namespace,
		Name:      name,
		Replicas:  1,
	}
}
