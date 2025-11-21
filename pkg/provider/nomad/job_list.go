package nomad

import (
	"context"

	"github.com/hashicorp/nomad/api"
	"github.com/sablierapp/sablier/pkg/provider"
	"github.com/sablierapp/sablier/pkg/sablier"
)

const (
	enableLabel = "sablier.enable"
	groupLabel  = "sablier.group"
)

// InstanceGroups returns a map of group names to instance names
// It scans all jobs in the namespace looking for the sablier.enable and sablier.group labels
func (p *Provider) InstanceGroups(ctx context.Context) (map[string][]string, error) {
	groups := make(map[string][]string)

	jobs := p.Client.Jobs()
	jobList, _, err := jobs.List(&api.QueryOptions{
		Namespace: p.namespace,
	})
	if err != nil {
		return nil, err
	}

	for _, jobStub := range jobList {
		// Get full job details to access task group metadata
		job, _, err := jobs.Info(jobStub.ID, &api.QueryOptions{
			Namespace: p.namespace,
		})
		if err != nil {
			p.l.WarnContext(ctx, "cannot get job info", "job_id", jobStub.ID, "error", err)
			continue
		}

		// Check each task group for sablier labels
		for _, tg := range job.TaskGroups {
			if tg.Name == nil {
				continue
			}

			// Check meta tags for sablier.enable
			if tg.Meta == nil {
				continue
			}

			enabled, hasEnable := tg.Meta[enableLabel]
			if !hasEnable || enabled != "true" {
				continue
			}

			groupName := "default"
			if gn, hasGroup := tg.Meta[groupLabel]; hasGroup && gn != "" {
				groupName = gn
			}

			instanceName := formatJobName(*job.ID, *tg.Name)
			groups[groupName] = append(groups[groupName], instanceName)
		}
	}

	return groups, nil
}

// InstanceList returns a list of all instances (task groups) that have Sablier enabled
func (p *Provider) InstanceList(ctx context.Context, options provider.InstanceListOptions) ([]sablier.InstanceConfiguration, error) {
	var instances []sablier.InstanceConfiguration

	jobs := p.Client.Jobs()
	jobList, _, err := jobs.List(&api.QueryOptions{
		Namespace: p.namespace,
	})
	if err != nil {
		return nil, err
	}

	for _, jobStub := range jobList {
		// Get full job details
		job, _, err := jobs.Info(jobStub.ID, &api.QueryOptions{
			Namespace: p.namespace,
		})
		if err != nil {
			p.l.WarnContext(ctx, "cannot get job info", "job_id", jobStub.ID, "error", err)
			continue
		}

		// Check each task group
		for _, tg := range job.TaskGroups {
			if tg.Name == nil {
				continue
			}

			// If All flag is not set, only return enabled instances
			if !options.All {
				if tg.Meta == nil {
					continue
				}
				enabled, hasEnable := tg.Meta[enableLabel]
				if !hasEnable || enabled != "true" {
					continue
				}
			}

			groupName := "default"
			if tg.Meta != nil {
				if gn, hasGroup := tg.Meta[groupLabel]; hasGroup && gn != "" {
					groupName = gn
				}
			}

			instanceName := formatJobName(*job.ID, *tg.Name)
			instances = append(instances, sablier.InstanceConfiguration{
				Name:  instanceName,
				Group: groupName,
			})
		}
	}

	return instances, nil
}
