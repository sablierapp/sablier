package docker

import (
	"context"
	"fmt"
	"net/netip"
	"path"
	"strings"

	"github.com/moby/moby/client"
	"github.com/sablierapp/sablier/pkg/sablier"
)

// Interface guard
var _ sablier.ActivityHost = (*Provider)(nil)

// cgroupRoot is where the cgroup v2 hierarchy is mounted, as seen by Sablier.
const cgroupRoot = "/sys/fs/cgroup"

// InstanceActivityTarget returns the cgroup v2 path and the exposed ports of a container.
func (p *Provider) InstanceActivityTarget(ctx context.Context, name string) (sablier.ActivityTarget, error) {
	spec, err := p.Client.ContainerInspect(ctx, name, client.ContainerInspectOptions{})
	if err != nil {
		return sablier.ActivityTarget{}, fmt.Errorf("cannot inspect container: %w", err)
	}

	driver, err := p.cgroupDriverName(ctx)
	if err != nil {
		return sablier.ActivityTarget{}, err
	}

	var parent string
	if spec.Container.HostConfig != nil {
		parent = spec.Container.HostConfig.CgroupParent
	}

	var ports []sablier.Port
	if spec.Container.Config != nil {
		for port := range spec.Container.Config.ExposedPorts {
			ports = append(ports, sablier.Port{Number: port.Num(), Proto: string(port.Proto())})
		}
	}

	return sablier.ActivityTarget{
		CgroupPath: cgroupPath(driver, parent, spec.Container.ID),
		Ports:      ports,
	}, nil
}

// ActivityIgnoredSources returns the IPs of the running containers labeled
// sablier.activity.ignore-source=true. Host-networked containers have no IP and
// are therefore not reported.
func (p *Provider) ActivityIgnoredSources(ctx context.Context) ([]netip.Addr, error) {
	filters := client.Filters{}
	filters.Add("label", fmt.Sprintf("%s=true", sablier.LabelActivityIgnoreSource))
	filters.Add("status", "running")

	containers, err := p.Client.ContainerList(ctx, client.ContainerListOptions{Filters: filters})
	if err != nil {
		return nil, fmt.Errorf("cannot list containers: %w", err)
	}

	var addrs []netip.Addr
	for _, c := range containers.Items {
		if c.NetworkSettings == nil {
			continue
		}
		for _, ep := range c.NetworkSettings.Networks {
			if ep == nil {
				continue
			}
			if ep.IPAddress.IsValid() {
				addrs = append(addrs, ep.IPAddress)
			}
			if ep.GlobalIPv6Address.IsValid() {
				addrs = append(addrs, ep.GlobalIPv6Address)
			}
		}
	}
	return addrs, nil
}

// cgroupDriverName returns the daemon's cgroup driver, fetched once and cached.
func (p *Provider) cgroupDriverName(ctx context.Context) (string, error) {
	p.cgroupMu.Lock()
	defer p.cgroupMu.Unlock()

	if p.cgroupDriver != "" {
		return p.cgroupDriver, nil
	}
	info, err := p.Client.Info(ctx, client.InfoOptions{})
	if err != nil {
		return "", fmt.Errorf("cannot get docker info: %w", err)
	}
	p.cgroupDriver = info.Info.CgroupDriver
	return p.cgroupDriver, nil
}

// cgroupPath computes the cgroup v2 directory of a container from the daemon's
// cgroup driver, the container's HostConfig.CgroupParent and its ID.
func cgroupPath(driver, parent, id string) string {
	if driver == "systemd" {
		if parent == "" {
			parent = "system.slice"
		}
		return path.Join(cgroupRoot, expandSlice(parent), "docker-"+id+".scope")
	}
	if parent == "" {
		parent = "/docker"
	}
	return path.Join(cgroupRoot, parent, id)
}

// expandSlice expands a systemd slice name into its nested directory path:
// "machine-sablier.slice" becomes "machine.slice/machine-sablier.slice".
func expandSlice(slice string) string {
	name := strings.TrimSuffix(slice, ".slice")
	if name == "" || name == "-" {
		return ""
	}
	parts := strings.Split(name, "-")
	segments := make([]string, 0, len(parts))
	for i := range parts {
		segments = append(segments, strings.Join(parts[:i+1], "-")+".slice")
	}
	return path.Join(segments...)
}
