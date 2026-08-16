package systemd

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path"
	"sync"
	"time"

	"github.com/coreos/go-systemd/v22/dbus"
	"github.com/sablierapp/sablier/pkg/sablier"
	"gopkg.in/ini.v1"
)

var _ sablier.Provider = (*Provider)(nil)

type Provider struct {
	Con          *dbus.Conn
	l            *slog.Logger
	pollInterval time.Duration

	// unitPatterns optionally restricts the units Sablier deals with to
	// those matching any of the given glob patterns.
	unitPatterns []string

	// labelCache avoids re-reading and re-parsing unit files on every
	// listing cycle; entries are invalidated when the file's mtime changes.
	labelCache labelCache
}

func New(ctx context.Context, con *dbus.Conn, logger *slog.Logger, unitPatterns []string) (*Provider, error) {
	logger = logger.With(slog.String("provider", "systemd"))

	if !con.Connected() {
		return nil, fmt.Errorf("no connection to systemd dbus")
	}

	logger.InfoContext(ctx, "connection established with systemd dbus")
	if len(unitPatterns) > 0 {
		logger.InfoContext(ctx, "restricting systemd provider to unit patterns", slog.Any("patterns", unitPatterns))
	}
	return &Provider{
		Con:          con,
		l:            logger,
		pollInterval: 10 * time.Second,
		unitPatterns: append([]string(nil), unitPatterns...),
		labelCache: labelCache{
			units: make(map[string]labelCacheEntry),
		},
	}, nil
}

// matchUnitPattern reports whether name matches any of the configured unit
// patterns. With no patterns configured every name matches.
func (p *Provider) matchUnitPattern(name string) bool {
	if len(p.unitPatterns) == 0 {
		return true
	}
	for _, pattern := range p.unitPatterns {
		if ok, err := path.Match(pattern, name); err == nil && ok {
			return true
		}
	}
	return false
}

// getLabels returns the parsed [X-Sablier] labels for the named unit. It
// fetches the unit's D-Bus properties for the fragment path and serves the
// parsed labels through the label cache.
func (p *Provider) getLabels(ctx context.Context, name string) (map[string]string, error) {
	prop, err := p.Con.GetUnitPropertyContext(ctx, name, "FragmentPath")
	if err != nil {
		return nil, err
	}

	fragmentPath, ok := prop.Value.Value().(string)
	if !ok || fragmentPath == "" {
		return nil, nil
	}

	return p.labelsFromUnitFileCached(fragmentPath)
}

// labelCacheEntry holds the parsed labels of a unit file together with the
// file metadata they were parsed from.
type labelCacheEntry struct {
	labels map[string]string
	mtime  time.Time
	size   int64
}

// labelCache caches parsed [X-Sablier] sections per unit file, keyed by
// fragment path. Entries are invalidated when the file's mtime or size
// changes, so unchanged unit files are parsed only once.
type labelCache struct {
	mu    sync.Mutex
	units map[string]labelCacheEntry
}

func (p *Provider) labelsFromUnitFileCached(path string) (map[string]string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	p.labelCache.mu.Lock()
	defer p.labelCache.mu.Unlock()

	if entry, ok := p.labelCache.units[path]; ok && entry.mtime.Equal(info.ModTime()) && entry.size == info.Size() {
		return entry.labels, nil
	}

	labels, err := labelsFromUnitFile(path)
	if err != nil {
		return nil, err
	}

	p.labelCache.units[path] = labelCacheEntry{labels: labels, mtime: info.ModTime(), size: info.Size()}
	return labels, nil
}

func labelsFromUnitFile(path string) (map[string]string, error) {
	cfg, err := ini.Load(path)
	if err != nil {
		return nil, err
	}

	section, err := cfg.GetSection("X-Sablier")
	if err != nil {
		return nil, nil
	}

	var labels map[string]string
	for _, key := range section.Keys() {
		label, ok := bareKeyLabels[key.Name()]
		if !ok {
			continue
		}
		if labels == nil {
			labels = make(map[string]string)
		}
		labels[label] = key.Value()
	}

	return labels, nil
}

// bareKeyLabels maps the bare [X-Sablier] section keys to their sablier.*
// label names.
var bareKeyLabels = map[string]string{
	"Enable":                     sablier.LabelEnable,
	"Group":                      sablier.LabelGroup,
	"ReadyAfter":                 sablier.LabelReadyAfter,
	"ReadyOnStart":               sablier.LabelReadyOnStart,
	"RunningHours":               sablier.LabelRunningHours,
	"RunningDays":                sablier.LabelRunningDays,
	"AntiAffinity":               sablier.LabelAntiAffinity,
	"IdleReplicas":               sablier.LabelIdleReplicas,
	"IdleCPU":                    sablier.LabelIdleCPU,
	"IdleMemory":                 sablier.LabelIdleMemory,
	"ActiveReplicas":             sablier.LabelActiveReplicas,
	"ActiveCPU":                  sablier.LabelActiveCPU,
	"ActiveMemory":               sablier.LabelActiveMemory,
	"IdleBlkioWeight":            sablier.LabelIdleBlkioWeight,
	"ActiveBlkioWeight":          sablier.LabelActiveBlkioWeight,
	"IdleBlkioWeightDevice":      sablier.LabelIdleBlkioWeightDevice,
	"ActiveBlkioWeightDevice":    sablier.LabelActiveBlkioWeightDevice,
	"IdleBlkioDeviceReadBps":     sablier.LabelIdleBlkioReadBps,
	"ActiveBlkioDeviceReadBps":   sablier.LabelActiveBlkioReadBps,
	"IdleBlkioDeviceWriteBps":    sablier.LabelIdleBlkioWriteBps,
	"ActiveBlkioDeviceWriteBps":  sablier.LabelActiveBlkioWriteBps,
	"IdleBlkioDeviceReadIOps":    sablier.LabelIdleBlkioReadIOps,
	"ActiveBlkioDeviceReadIOps":  sablier.LabelActiveBlkioReadIOps,
	"IdleBlkioDeviceWriteIOps":   sablier.LabelIdleBlkioWriteIOps,
	"ActiveBlkioDeviceWriteIOps": sablier.LabelActiveBlkioWriteIOps,
}

func (p *Provider) InstanceDependencies(_ context.Context, _ string) ([]sablier.InstanceDependency, error) {
	return nil, nil
}
