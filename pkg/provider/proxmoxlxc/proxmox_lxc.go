package proxmoxlxc

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"time"

	proxmox "github.com/luthermonson/go-proxmox"
	"github.com/sablierapp/sablier/pkg/sablier"
)

// Interface guard
var _ sablier.Provider = (*Provider)(nil)

// containerRef holds the mapping from an instance name to the actual Proxmox container location.
type containerRef struct {
	node string
	vmid int
	name string // LXC hostname
}

// Provider implements the sablier.Provider interface for Proxmox VE LXC containers.
type Provider struct {
	client          *proxmox.Client
	l               *slog.Logger
	desiredReplicas int32
	pollInterval    time.Duration

	mu    sync.RWMutex
	cache map[string]containerRef // hostname or VMID string → ref
}

// New creates a new Proxmox LXC provider and verifies the connection.
func New(ctx context.Context, client *proxmox.Client, logger *slog.Logger) (*Provider, error) {
	logger = logger.With(slog.String("provider", "proxmox_lxc"))

	version, err := client.Version(ctx)
	if err != nil {
		return nil, fmt.Errorf("cannot connect to Proxmox VE API: %w", err)
	}

	logger.InfoContext(ctx, "connection established with Proxmox VE",
		slog.String("version", version.Version),
		slog.String("release", version.Release),
	)

	return &Provider{
		client:          client,
		l:               logger,
		desiredReplicas: 1,
		pollInterval:    10 * time.Second,
		cache:           make(map[string]containerRef),
	}, nil
}

// resolve looks up a container by hostname, VMID string, or "node/vmid" format.
// It first checks the cache, then rescans all nodes if not found.
func (p *Provider) resolve(ctx context.Context, name string) (containerRef, error) {
	// Handle "node/vmid" format (e.g. "pve/111")
	if node, vmidStr, ok := strings.Cut(name, "/"); ok {
		vmid, err := strconv.Atoi(vmidStr)
		if err != nil {
			return containerRef{}, fmt.Errorf("invalid VMID in %q: %w", name, err)
		}
		// Try to resolve hostname from cache via VMID so that ref.name matches
		// the hostname used by stop-event detection in NotifyInstanceStopped.
		if ref, ok := p.lookupCache(vmidStr); ok && ref.node == node {
			return ref, nil
		}
		// Cache miss for node/vmid — rescan containers to refresh hostname mapping.
		if _, err := p.scanContainers(ctx); err != nil {
			return containerRef{}, fmt.Errorf("cannot scan containers: %w", err)
		}
		if ref, ok := p.lookupCache(vmidStr); ok && ref.node == node {
			return ref, nil
		}
		// Fall back to a best-effort reference when the container cannot be
		// discovered via scan; name will be the original "node/vmid" string.
		return containerRef{node: node, vmid: vmid, name: name}, nil
	}

	if ref, ok := p.lookupCache(name); ok {
		return ref, nil
	}

	// Cache miss — rescan all nodes
	if _, err := p.scanContainers(ctx); err != nil {
		return containerRef{}, fmt.Errorf("cannot scan containers: %w", err)
	}

	if ref, ok := p.lookupCache(name); ok {
		return ref, nil
	}

	return containerRef{}, fmt.Errorf("container %q not found", name)
}

func (p *Provider) lookupCache(key string) (containerRef, bool) {
	p.mu.RLock()
	ref, ok := p.cache[key]
	p.mu.RUnlock()
	return ref, ok
}

// getContainer fetches a proxmox.Container from the API for the given ref.
func (p *Provider) getContainer(ctx context.Context, ref containerRef) (*proxmox.Container, error) {
	node, err := p.client.Node(ctx, ref.node)
	if err != nil {
		return nil, fmt.Errorf("cannot get node %q: %w", ref.node, err)
	}
	ct, err := node.Container(ctx, ref.vmid)
	if err != nil {
		return nil, fmt.Errorf("cannot get container %d on node %q: %w", ref.vmid, ref.node, err)
	}
	return ct, nil
}

const defaultTaskTimeout = 60 * time.Second

// taskTimeout returns the remaining time from the context deadline, or defaultTaskTimeout if none is set.
func taskTimeout(ctx context.Context) time.Duration {
	if deadline, ok := ctx.Deadline(); ok {
		if remaining := time.Until(deadline); remaining > 0 {
			return remaining
		}
	}
	return defaultTaskTimeout
}

type discoveredContainer struct {
	ref    containerRef
	tags   []string
	status string
}

// scanContainers scans all nodes for sablier-tagged LXC containers and rebuilds the cache.
// Returns the list of discovered containers with their tags.
func (p *Provider) scanContainers(ctx context.Context) ([]discoveredContainer, error) {
	nodes, err := p.client.Nodes(ctx)
	if err != nil {
		return nil, fmt.Errorf("cannot list nodes: %w", err)
	}

	var discovered []discoveredContainer
	newCache := make(map[string]containerRef)

	for _, ns := range nodes {
		node, err := p.client.Node(ctx, ns.Node)
		if err != nil {
			p.l.WarnContext(ctx, "cannot access node, skipping", slog.String("node", ns.Node), slog.Any("error", err))
			continue
		}

		containers, err := node.Containers(ctx)
		if err != nil {
			p.l.WarnContext(ctx, "cannot list containers on node, skipping", slog.String("node", ns.Node), slog.Any("error", err))
			continue
		}

		for _, c := range containers {
			tags := parseTags(c.Tags)
			if !hasSablierTag(tags) {
				continue
			}

			ref := containerRef{
				node: ns.Node,
				vmid: int(c.VMID),
				name: c.Name,
			}

			// Check for hostname collision
			if existing, ok := newCache[c.Name]; ok {
				return nil, fmt.Errorf("duplicate hostname %q found on node %q (VMID %d) and node %q (VMID %d) among sablier-managed containers",
					c.Name, existing.node, existing.vmid, ns.Node, int(c.VMID))
			}

			newCache[c.Name] = ref
			newCache[strconv.Itoa(int(c.VMID))] = ref

			discovered = append(discovered, discoveredContainer{
				ref:    ref,
				tags:   tags,
				status: c.Status,
			})
		}
	}

	p.mu.Lock()
	p.cache = newCache
	p.mu.Unlock()

	return discovered, nil
}
