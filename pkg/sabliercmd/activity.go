package sabliercmd

import (
	"fmt"
	"io"
	"log/slog"
	"net/netip"
	"strings"

	"github.com/sablierapp/sablier/pkg/activity/bpf"
	"github.com/sablierapp/sablier/pkg/activity/local"
	"github.com/sablierapp/sablier/pkg/config"
	"github.com/sablierapp/sablier/pkg/sablier"
)

// setupActivity builds the activity source behind --activity.enabled and the
// closer that unloads its eBPF programs. It fails, and Sablier refuses to
// start, when the provider is not Docker, the configuration is invalid, or the
// eBPF programs cannot load.
func setupActivity(conf config.Activity, providerName string, p sablier.Provider, logger *slog.Logger) (sablier.ActivitySource, io.Closer, error) {
	// Only Docker: the Podman provider embeds the Docker one, so it implements
	// sablier.ActivityHost, but it would compute Docker's cgroup paths.
	host, ok := p.(sablier.ActivityHost)
	if providerName != "docker" || !ok {
		return nil, nil, fmt.Errorf("--activity.enabled: provider %q does not support activity tracking: activity tracking is Docker-only", providerName)
	}

	ignore := make([]netip.Prefix, 0, len(conf.IgnoreCIDRs))
	for _, cidr := range conf.IgnoreCIDRs {
		prefix, err := netip.ParsePrefix(strings.TrimSpace(cidr))
		if err != nil {
			return nil, nil, fmt.Errorf("--activity.enabled: invalid --activity.ignore-cidrs entry %q: %w", cidr, err)
		}
		ignore = append(ignore, prefix)
	}

	// A ticker panics on a non-positive interval.
	if conf.Interval <= 0 {
		return nil, nil, fmt.Errorf("--activity.enabled: --activity.interval must be greater than 0, got %s", conf.Interval)
	}

	tracker, err := bpf.New()
	if err != nil {
		return nil, nil, fmt.Errorf("--activity.enabled: %w", err)
	}
	return local.New(tracker, host, ignore, logger), tracker, nil
}
