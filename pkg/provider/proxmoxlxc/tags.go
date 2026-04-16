package proxmoxlxc

import (
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
	for _, t := range tags {
		if t == enableTag {
			return true
		}
	}
	return false
}

// extractGroup returns the group name from a "sablier-group-<name>" tag.
// Returns "default" if no group tag is found.
func extractGroup(tags []string) string {
	for _, t := range tags {
		if strings.HasPrefix(t, groupPrefix) {
			group := strings.TrimPrefix(t, groupPrefix)
			if group != "" {
				return group
			}
		}
	}
	return "default"
}
