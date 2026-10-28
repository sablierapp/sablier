package sablier

import (
	"log/slog"
	"strconv"
	"time"

	"github.com/docker/go-units"
)

// InstanceConfig is the typed, parsed form of the sablier.* labels an
// instance carries. It is produced in one place, InstanceConfigFromLabels, at
// the provider boundary; nothing else should interpret raw label strings.
//
// This is the first stage of splitting InstanceInfo (which today mixes
// provider-reported state, parsed label configuration, the API wire format
// and the persisted session record): the configuration now has one typed
// home, while InstanceInfo keeps its flat legacy fields byte-identical on the
// wire for API and store compatibility.
type InstanceConfig struct {
	// Enabled reports whether the instance opted into Sablier management
	// (sablier.enable set to exactly "true").
	Enabled bool `json:"enabled,omitempty"`
	// Groups the instance belongs to (sablier.group). Only populated for
	// enabled instances; defaults to ["default"] when the label is absent.
	Groups []string `json:"groups,omitempty"`
	// ReadyAfter is the settling delay after the instance first reports ready
	// (sablier.ready-after). Zero means no extra wait.
	ReadyAfter time.Duration `json:"readyAfter,omitempty"`
	// ReadyOnStart marks the instance ready as soon as its start is
	// dispatched, skipping the health check (sablier.ready-on-start).
	ReadyOnStart bool `json:"readyOnStart,omitempty"`
	// RunningHours is the validated daily keep-warm window
	// (sablier.running-hours, "HH:MM-HH:MM"). It is kept in its validated
	// string form because that is the serializable canonical representation;
	// parse it with ParseRunningHours where the window is evaluated.
	RunningHours string `json:"runningHours,omitempty"`
	// RunningDays restricts RunningHours to specific weekdays
	// (sablier.running-days). Empty means every day.
	RunningDays string `json:"runningDays,omitempty"`
	// AntiAffinity lists the groups this instance backs off from
	// (sablier.anti-affinity).
	AntiAffinity []string `json:"antiAffinity,omitempty"`
	// Scale holds the idle/active resource profiles when any non-default
	// scale-mode label is present (sablier.idle.* / sablier.active.*).
	Scale *ScaleConfig `json:"scale,omitempty"`
	// Activity holds the eBPF activity-tracking configuration. It is nil
	// unless sablier.enable and sablier.activity.enable are both "true".
	Activity *ActivityConfig `json:"activity,omitempty"`
}

// InstanceConfigFromLabels parses every sablier.* label into a typed
// InstanceConfig. Invalid values are logged through l and ignored, keeping
// the previous per-label warn-and-skip behavior. A nil logger falls back to
// slog.Default().
func InstanceConfigFromLabels(labels map[string]string, l *slog.Logger) InstanceConfig {
	if l == nil {
		l = slog.Default()
	}

	var cfg InstanceConfig
	cfg.Enabled = labels[LabelEnable] == "true"
	if cfg.Enabled {
		cfg.Groups = ParseGroups(labels[LabelGroup])
	}
	if v := labels[LabelReadyAfter]; v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.ReadyAfter = d
		} else {
			l.Warn("invalid sablier.ready-after label value, ignoring",
				slog.String("value", v),
				slog.Any("error", err),
			)
		}
	}
	if v := labels[LabelRunningHours]; v != "" {
		if _, err := ParseRunningHours(v); err == nil {
			cfg.RunningHours = v
		} else {
			l.Warn("invalid sablier.running-hours label value, ignoring",
				slog.String("value", v),
				slog.Any("error", err),
			)
		}
	}
	if v := labels[LabelRunningDays]; v != "" {
		if _, err := ParseRunningDays(v); err == nil {
			cfg.RunningDays = v
		} else {
			l.Warn("invalid sablier.running-days label value, ignoring",
				slog.String("value", v),
				slog.Any("error", err),
			)
		}
	}
	if v := labels[LabelReadyOnStart]; v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			l.Warn("invalid sablier.ready-on-start label value, ignoring",
				slog.String("value", v),
				slog.Any("error", err),
			)
		} else {
			cfg.ReadyOnStart = b
		}
	}
	if v := labels[LabelAntiAffinity]; v != "" {
		cfg.AntiAffinity = ParseAntiAffinity(v)
	}
	// Only expose Scale when at least one non-default scale label is present,
	// detected by values that differ from the zero-value defaults
	// (Idle.Replicas=0, Active.Replicas=1).
	sc := ScaleConfigFromLabels(labels)
	if sc.Idle.Replicas > 0 || sc.Idle.HasResources() ||
		sc.Active.Replicas > 1 || sc.Active.HasResources() {
		cfg.Scale = &sc
	}
	if cfg.Enabled && labels[LabelActivityEnable] == "true" {
		cfg.Activity = activityConfigFromLabels(labels, l)
	}
	return cfg
}

func activityConfigFromLabels(labels map[string]string, l *slog.Logger) *ActivityConfig {
	a := &ActivityConfig{MinBytes: DefaultActivityMinBytes}
	if v := labels[LabelActivityPorts]; v != "" {
		if ports, err := ParseActivityPorts(v); err == nil {
			a.Ports = ports
		} else {
			l.Warn("invalid sablier.activity.ports label value, ignoring",
				slog.String("value", v),
				slog.Any("error", err),
			)
		}
	}
	if v := labels[LabelActivitySessionDuration]; v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			a.SessionDuration = d
		} else {
			l.Warn("invalid sablier.activity.session-duration label value, ignoring",
				slog.String("value", v),
				slog.Any("error", err),
			)
		}
	}
	if v := labels[LabelActivityMinBytes]; v != "" {
		if n, err := units.RAMInBytes(v); err == nil && n >= 0 {
			a.MinBytes = uint64(n)
		} else {
			l.Warn("invalid sablier.activity.min-bytes label value, ignoring",
				slog.String("value", v),
				slog.Any("error", err),
			)
		}
	}
	if labels[LabelGroup] != "" {
		a.Groups = ParseGroups(labels[LabelGroup])
	}
	return a
}
