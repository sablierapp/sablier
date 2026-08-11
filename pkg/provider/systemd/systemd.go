package systemd

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
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
}

func New(ctx context.Context, con *dbus.Conn, logger *slog.Logger) (*Provider, error) {
	logger = logger.With(slog.String("provider", "systemd"))

	if !con.Connected() {
		return nil, fmt.Errorf("no connection to systemd dbus")
	}

	logger.InfoContext(ctx, "connection established with systemd dbus")
	return &Provider{
		Con:          con,
		l:            logger,
		pollInterval: 10 * time.Second,
	}, nil
}

func (p *Provider) readLabels(ctx context.Context, name string) (map[string]string, error) {
	props, err := p.Con.GetUnitPropertiesContext(ctx, name)
	if err != nil {
		return nil, err
	}

	return labelsFromProperties(props)
}

func (p *Provider) readManagedLabels(ctx context.Context, name string) (map[string]string, error) {
	labels, err := p.readLabels(ctx, name)
	if err != nil {
		return nil, err
	}
	if labels[sablier.LabelEnable] != "true" {
		return nil, fmt.Errorf("systemd unit %q is not enabled for Sablier", name)
	}
	return labels, nil
}

func labelsFromProperties(dbusProps map[string]any) (map[string]string, error) {
	fragmentPath, ok := dbusProps["FragmentPath"].(string)
	if !ok || fragmentPath == "" {
		return nil, nil
	}

	return labelsFromUnitFile(fragmentPath)
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

func unitNameFromPath(path string) string {
	return filepath.Base(path)
}

func (p *Provider) InstanceDependencies(_ context.Context, _ string) ([]sablier.InstanceDependency, error) {
	return nil, nil
}
