package proxmoxlxc

import (
	"slices"
	"strings"

	proxmox "github.com/luthermonson/go-proxmox"
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
