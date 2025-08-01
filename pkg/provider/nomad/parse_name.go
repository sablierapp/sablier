package nomad

import (
	"errors"
	"strconv"
	"strings"
)

type Config struct {
	OriginalName string
	Namespace    string
	Job          string
	Group        string
	Replicas     int
}

// convertName parses name into the target Namespace, Job and Group as "job@namespace/taskgroup/replicas"
// replicas defaults to 1; eg, "job@namespace/taskgroup" is valid
func (p *Provider) convertName(name string) (*Config, error) {
	config := Config{
		OriginalName: name,
		Replicas:     1,
	}

	// Split the first part based on '/'
	parts := strings.Split(name, "/")
	if len(parts) < 2 {
		return &config, errors.New("invalid name, should be: job@namespace/taskgroup/1")
	}

	config.Group = parts[1]

	// parts[0] contains "job@namespace" and parts[1] contains "taskgroup"
	subParts := strings.Split(parts[0], "@")
	if len(subParts) != 2 {
		return &config, errors.New("invalid name, should be: job@namespace/taskgroup/1")
	}
	config.Job = subParts[0]
	config.Namespace = subParts[1]

	// if replicas are defined, set them
	if len(parts) == 3 {
		var err error
		config.Replicas, err = strconv.Atoi(parts[2])
		if err != nil {
			return &config, errors.New("invalid name, error parsing replicas. should be: job@namespace/taskgroup/1")
		}
	}

	return &config, nil
}
