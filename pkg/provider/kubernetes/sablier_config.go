package kubernetes

import (
	"maps"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/sablierapp/sablier/pkg/sablier"
)

// sablierConfigPrefix is the common prefix for all Sablier configuration keys.
const sablierConfigPrefix = "sablier."

// sablierPublicPrefix makes "sablierapp.dev/<name>" the public form of "sablier.<name>".
// https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/#syntax-and-character-set
const sablierPublicPrefix = "sablierapp.dev/"

// enableSelectors match the public and the legacy enable label.
// A label selector cannot express OR, thus each selector needs its own list call.
var enableSelectors = []string{
	sablierPublicPrefix + "enable=true",
	sablier.LabelEnable + "=true",
}

// sablierConfig merges a workload's labels and annotations into a single
// configuration map from which Sablier reads its "sablier.*" keys.
//
// On Kubernetes, label values are restricted (max 63 chars, only
// [A-Za-z0-9._-], no commas or colons), which makes several Sablier values
// impossible to express as labels — for example a comma-separated
// "sablier.group" / "sablier.running-days", or the colon-based
// "sablier.running-hours=09:00-18:00". Annotations have no such restriction, so
// they are supported as an alternative source for every Sablier key.
//
// Only Sablier keys are copied from annotations, so unrelated (and potentially
// large) annotations such as kubectl.kubernetes.io/last-applied-configuration
// are never included.
// Annotations take precedence over labels when the same key is present in both.
//
// A "sablierapp.dev/<name>" key is read as "sablier.<name>". In the same source,
// the public key takes precedence over the legacy key.
//
// Note: the enable key should still be set as a label because workload
// discovery relies on a server-side label selector; annotations are not
// selectable. Setting it only as an annotation keeps it out of discovery.
func sablierConfig(labels, annotations map[string]string) map[string]string {
	merged := make(map[string]string, len(labels)+len(annotations))
	maps.Copy(merged, labels)
	copyPublicKeys(merged, labels)
	for k, v := range annotations {
		if strings.HasPrefix(k, sablierConfigPrefix) {
			merged[k] = v
		}
	}
	copyPublicKeys(merged, annotations)
	return merged
}

// copyPublicKeys copies each "sablierapp.dev/<name>" key of src to dst as "sablier.<name>".
func copyPublicKeys(dst, src map[string]string) {
	for k, v := range src {
		if name, ok := strings.CutPrefix(k, sablierPublicPrefix); ok {
			dst[sablierConfigPrefix+name] = v
		}
	}
}

// listEnabled calls list one time for each enable selector and removes the duplicates.
func listEnabled[T any, PT interface {
	*T
	metav1.Object
}](list func(metav1.ListOptions) ([]T, error)) ([]T, error) {
	var items []T
	seen := make(map[string]bool)
	for _, selector := range enableSelectors {
		found, err := list(metav1.ListOptions{LabelSelector: selector})
		if err != nil {
			return nil, err
		}
		for i := range found {
			obj := PT(&found[i])
			key := obj.GetNamespace() + "/" + obj.GetName()
			if seen[key] {
				continue
			}
			seen[key] = true
			items = append(items, found[i])
		}
	}
	return items, nil
}
