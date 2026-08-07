package kubernetes

import "strings"

// sablierConfigPrefix is the common prefix for all Sablier configuration keys.
const sablierConfigPrefix = "sablier."

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
// Only keys carrying the "sablier." prefix are copied from annotations, so
// unrelated (and potentially large) annotations such as
// kubectl.kubernetes.io/last-applied-configuration are never included.
// Annotations take precedence over labels when the same key is present in both.
//
// Note: "sablier.enable" should still be set as a label because workload
// discovery relies on a server-side label selector; annotations are not
// selectable. Setting it only as an annotation keeps it out of discovery.
func sablierConfig(labels, annotations map[string]string) map[string]string {
	merged := make(map[string]string, len(labels)+len(annotations))
	for k, v := range labels {
		merged[k] = v
	}
	for k, v := range annotations {
		if strings.HasPrefix(k, sablierConfigPrefix) {
			merged[k] = v
		}
	}
	return merged
}

