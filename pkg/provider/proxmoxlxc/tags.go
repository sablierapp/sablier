package proxmoxlxc

import (
	"slices"
	"strings"

	proxmox "github.com/luthermonson/go-proxmox"
	"github.com/sablierapp/sablier/pkg/sablier"
)

const (
	enableTag   = "sablier"
	groupPrefix = "sablier-group-"
)

// parseTags splits a semicolon-separated tag string into individual tags.
func parseTags(tagString string) []string {
	if tagString == "" {
		return nil
	}
	return strings.Split(tagString, proxmox.TagSeperator)
}

// hasSablierTag returns true if the "sablier" tag is present in the list.
func hasSablierTag(tags []string) bool {
	return slices.Contains(tags, enableTag)
}

// extractGroups returns all group names from "sablier-group-<name>" tags.
// Returns []string{"default"} if no group tag is found.
func extractGroups(tags []string) []string {
	var groups []string
	for _, t := range tags {
		if after, ok := strings.CutPrefix(t, groupPrefix); ok {
			group := after
			if group != "" {
				groups = append(groups, group)
			}
		}
	}
	if len(groups) == 0 {
		return []string{"default"}
	}
	return groups
}

// containerTags returns the tags of a container, read from its configuration
// and, failing that, from its status.
func containerTags(ct *proxmox.Container) []string {
	if ct.ContainerConfig != nil && ct.ContainerConfig.Tags != "" {
		return parseTags(ct.ContainerConfig.Tags)
	}
	return parseTags(ct.Tags)
}

// setEnabledAndGroups reports a container with the "sablier" tag as enabled,
// with the groups of its "sablier-group-<name>" tags, like the labels of other
// providers. A container without the tag is left as not enabled.
func setEnabledAndGroups(info *sablier.InstanceInfo, tags []string) {
	if !hasSablierTag(tags) {
		return
	}
	info.Enabled = "true"
	info.Groups = extractGroups(tags)
}
