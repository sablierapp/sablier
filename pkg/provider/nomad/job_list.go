package nomad

import (
	"cmp"
	"context"
	"log/slog"
	"slices"

	"github.com/sablierapp/sablier/pkg/provider"
	"github.com/sablierapp/sablier/pkg/sablier"
)

// discoveredGroup is a task group that opted into Sablier management.
type discoveredGroup struct {
	name    string
	meta    map[string]string
	running bool
}

// discover lists the task groups carrying sablier.enable=true, sorted by name.
func (p *Provider) discover(ctx context.Context) ([]discoveredGroup, error) {
	jobs, _, err := p.listJobs(ctx)
	if err != nil {
		return nil, err
	}

	var groups []discoveredGroup
	for _, job := range jobs {
		for _, tg := range job.TaskGroups {
			meta := groupMeta(job, tg)
			if meta[sablier.LabelEnable] != "true" {
				continue
			}
			groups = append(groups, discoveredGroup{
				name:    instanceName(deref(job.ID), deref(tg.Name)),
				meta:    meta,
				running: deref(tg.Count) > 0 && !deref(job.Stop),
			})
		}
	}
	slices.SortFunc(groups, func(a, b discoveredGroup) int { return cmp.Compare(a.name, b.name) })

	p.l.DebugContext(ctx, "task groups listed", slog.Int("jobs", len(jobs)), slog.Int("enabled", len(groups)))
	return groups, nil
}

func (p *Provider) InstanceList(ctx context.Context, options provider.InstanceListOptions) ([]sablier.InstanceConfiguration, error) {
	groups, err := p.discover(ctx)
	if err != nil {
		return nil, err
	}

	instances := make([]sablier.InstanceConfiguration, 0, len(groups))
	for _, g := range groups {
		if !options.All && !g.running {
			continue
		}
		instances = append(instances, sablier.InstanceConfiguration{
			Name:    g.name,
			Groups:  sablier.ParseGroups(g.meta[sablier.LabelGroup]),
			Enabled: g.meta[sablier.LabelEnable],
		})
	}
	return instances, nil
}

func (p *Provider) InstanceGroups(ctx context.Context) (map[string][]string, error) {
	discovered, err := p.discover(ctx)
	if err != nil {
		return nil, err
	}

	groups := make(map[string][]string)
	for _, g := range discovered {
		for _, groupName := range sablier.ParseGroups(g.meta[sablier.LabelGroup]) {
			groups[groupName] = append(groups[groupName], g.name)
		}
	}
	return groups, nil
}
