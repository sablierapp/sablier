package sabliercmd

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/coreos/go-systemd/v22/dbus"
	nomadapi "github.com/hashicorp/nomad/api"
	proxmox "github.com/luthermonson/go-proxmox"
	"github.com/moby/moby/client"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"

	"github.com/sablierapp/sablier/pkg/config"
	"github.com/sablierapp/sablier/pkg/provider/docker"
	"github.com/sablierapp/sablier/pkg/provider/dockerswarm"
	"github.com/sablierapp/sablier/pkg/provider/kubernetes"
	"github.com/sablierapp/sablier/pkg/provider/nomad"
	"github.com/sablierapp/sablier/pkg/provider/podman"
	"github.com/sablierapp/sablier/pkg/provider/proxmoxlxc"
	"github.com/sablierapp/sablier/pkg/provider/systemd"
	"github.com/sablierapp/sablier/pkg/sablier"
	"k8s.io/client-go/dynamic"
	k8s "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func setupProvider(ctx context.Context, logger *slog.Logger, config config.Provider) (sablier.Provider, error) {
	if err := config.IsValid(); err != nil {
		return nil, err
	}

	switch config.Name {
	case "swarm", "docker_swarm":
		// client.WithTraceProvider instruments all Docker API calls via the
		// built-in otelhttp transport wrapper in the moby client.
		cli, err := client.New(client.FromEnv, client.WithTraceProvider(otel.GetTracerProvider()))
		if err != nil {
			return nil, fmt.Errorf("cannot create docker swarm client: %v", err)
		}
		return dockerswarm.New(ctx, cli, logger)
	case "docker":
		// The Docker client is configured from the standard Docker environment
		// variables (DOCKER_HOST, DOCKER_API_VERSION, DOCKER_CERT_PATH,
		// DOCKER_TLS_VERIFY) through client.FromEnv. client.WithTraceProvider
		// instruments all Docker API calls via the moby otelhttp transport wrapper.
		cli, err := client.New(client.FromEnv, client.WithTraceProvider(otel.GetTracerProvider()))
		if err != nil {
			return nil, fmt.Errorf("cannot create docker client: %v", err)
		}
		p, err := docker.New(ctx, cli, logger, config.Docker.Strategy)
		if err != nil {
			return nil, err
		}
		//nolint:staticcheck // Intentionally wiring the deprecated transitional flag until it becomes the default in v2.
		p.HonorRestartPolicy = config.Docker.HonorRestartPolicy
		return p, nil
	case "kubernetes":
		kubeclientConfig, err := rest.InClusterConfig()
		if err != nil {
			return nil, err
		}
		kubeclientConfig.QPS = config.Kubernetes.QPS
		kubeclientConfig.Burst = config.Kubernetes.Burst
		// Wrap the Kubernetes API transport with OpenTelemetry so every
		// call to the API server is captured as a child span.
		kubeclientConfig.WrapTransport = func(rt http.RoundTripper) http.RoundTripper {
			return otelhttp.NewTransport(rt)
		}

		cli, err := k8s.NewForConfig(kubeclientConfig)
		if err != nil {
			return nil, err
		}
		// dynamicCli manages Custom Resources (e.g. CloudNativePG Clusters) that
		// the typed clientset cannot reach. It shares the same instrumented config.
		dynamicCli, err := dynamic.NewForConfig(kubeclientConfig)
		if err != nil {
			return nil, err
		}
		return kubernetes.New(ctx, cli, dynamicCli, logger, config.Kubernetes)
	case "podman":
		opts := []client.Opt{client.FromEnv}
		if config.Podman.Uri != "" {
			opts = append(opts, client.WithHost(config.Podman.Uri))
		}
		opts = append(opts, client.WithTraceProvider(otel.GetTracerProvider()))
		cli, err := client.New(opts...)
		if err != nil {
			return nil, fmt.Errorf("cannot create podman client: %w", err)
		}
		return podman.New(ctx, cli, logger)
	case "proxmox_lxc":
		opts := []proxmox.Option{
			proxmox.WithAPIToken(config.ProxmoxLXC.TokenID, config.ProxmoxLXC.TokenSecret),
		}
		baseTransport := http.DefaultTransport
		if config.ProxmoxLXC.TLSInsecure {
			transport := http.DefaultTransport.(*http.Transport).Clone()
			transport.TLSClientConfig = &tls.Config{
				InsecureSkipVerify: true, //nolint:gosec // user-configured option for self-signed certs
			}
			baseTransport = transport
		}
		opts = append(opts, proxmox.WithHTTPClient(&http.Client{
			Transport: otelhttp.NewTransport(baseTransport),
		}))
		cli := proxmox.NewClient(config.ProxmoxLXC.URL, opts...)
		return proxmoxlxc.New(ctx, cli, logger)
	case "systemd":
		var con *dbus.Conn
		var err error
		if config.Systemd.UserInstance {
			con, err = dbus.NewUserConnectionContext(ctx)
		} else {
			con, err = dbus.NewSystemConnectionContext(ctx)
		}
		if err != nil {
			return nil, fmt.Errorf("cannot connect to systemd dbus: %w", err)
		}

		provider, err := systemd.New(ctx, con, logger, config.Systemd.UnitPatterns)
		if err != nil {
			con.Close()
			return nil, err
		}
		return provider, nil
	case "nomad":
		cli, namespace, err := newNomadClient(config.Nomad)
		if err != nil {
			return nil, err
		}
		return nomad.New(ctx, cli, namespace, logger)
	}
	return nil, fmt.Errorf("unimplemented provider %s", config.Name)
}

// newNomadClient builds the Nomad API client and returns its namespace. The
// Sablier settings override the NOMAD_* environment variables read by DefaultConfig.
func newNomadClient(config config.Nomad) (*nomadapi.Client, string, error) {
	cfg := nomadapi.DefaultConfig()
	if config.Address != "" {
		cfg.Address = config.Address
	}
	if config.Token != "" {
		cfg.SecretID = config.Token
	}
	if config.Namespace != "" {
		cfg.Namespace = config.Namespace
	}
	if config.Region != "" {
		cfg.Region = config.Region
	}
	if cfg.Namespace == "" {
		cfg.Namespace = nomadapi.DefaultNamespace
	}

	// The custom HTTP client keeps the Nomad TLS settings and wraps the
	// transport with OpenTelemetry so every API call becomes a child span.
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	httpClient := &http.Client{Transport: transport}
	if err := nomadapi.ConfigureTLS(httpClient, cfg.TLSConfig); err != nil {
		return nil, "", fmt.Errorf("cannot configure nomad TLS: %w", err)
	}
	httpClient.Transport = otelhttp.NewTransport(httpClient.Transport)
	cfg.HttpClient = httpClient

	cli, err := nomadapi.NewClient(cfg)
	if err != nil {
		return nil, "", fmt.Errorf("cannot create nomad client: %w", err)
	}
	return cli, cfg.Namespace, nil
}
