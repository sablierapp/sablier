package docker

import "strings"

// FormatName removes the container name `/` prefix returned by the Docker API
func FormatName(name string) string {
	return strings.TrimPrefix(name, "/")
}
