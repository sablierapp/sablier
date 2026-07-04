package sabliercmd

// flagSince records the release each flag was introduced in. It was backfilled
// from git history (the first commit adding the flag name, then the earliest
// release tag containing that commit). "unreleased" marks flags added after the
// latest release tag; set the concrete version when they ship.
//
// cmd/docgen reads this via SinceForFlag to render "Since <version>" on the
// generated CLI reference page.
var flagSince = map[string]string{
	"configFile":                                  "v1.1.0",
	"logging.level":                               "v1.0.0",
	"provider.name":                               "v1.0.0",
	"provider.auto-stop-on-startup":               "v1.8.0",
	"provider.auto-stop-externally-started":       "v1.13.0",
	"provider.auto-warm-externally-started":       "unreleased",
	"provider.reject-unlabeled-requests":          "v1.13.0",
	"provider.verify-enabled-on-expiration":       "v1.13.0",
	"provider.kubernetes.qps":                     "v1.4.1-beta.2",
	"provider.kubernetes.burst":                   "v1.4.1-beta.2",
	"provider.kubernetes.delimiter":               "v1.7.0",
	"provider.podman.uri":                         "v1.10.0",
	"provider.docker.strategy":                    "v1.11.0",
	"provider.docker.honor-restart-policy":        "unreleased",
	"provider.proxmox-lxc.url":                    "v1.12.0",
	"provider.proxmox-lxc.token-id":               "v1.12.0",
	"provider.proxmox-lxc.token-secret":           "v1.12.0",
	"provider.proxmox-lxc.tls-insecure":           "v1.12.0",
	"server.port":                                 "v1.0.0",
	"server.base-path":                            "v1.0.0",
	"server.metrics.enabled":                      "v1.12.0",
	"tracing.enabled":                             "v1.13.0",
	"tracing.exporter-type":                       "v1.13.0",
	"tracing.endpoint":                            "v1.13.0",
	"tracing.service-name":                        "v1.13.0",
	"tracing.sampling-rate":                       "v1.13.0",
	"storage.file":                                "v1.0.0",
	"sessions.default-duration":                   "v1.0.0",
	"sessions.expiration-interval":                "v1.0.0",
	"strategy.dynamic.custom-themes-path":         "v1.0.0",
	"strategy.dynamic.default-theme":              "v1.0.0",
	"strategy.dynamic.show-details-by-default":    "v1.0.0",
	"strategy.dynamic.default-refresh-frequency":  "v1.0.0",
	"strategy.blocking.default-timeout":           "v1.0.0",
	"strategy.blocking.default-refresh-frequency": "v1.9.0",
}

// SinceForFlag returns the release a flag was introduced in, or "" if unknown.
func SinceForFlag(name string) string { return flagSince[name] }
