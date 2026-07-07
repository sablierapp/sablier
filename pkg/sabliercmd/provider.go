package sabliercmd

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"

	proxmox "github.com/luthermonson/go-proxmox"
	"github.com/moby/moby/client"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"

	"github.com/sablierapp/sablier/pkg/config"
	"github.com/sablierapp/sablier/pkg/provider/docker"
	"github.com/sablierapp/sablier/pkg/provider/dockerswarm"
	"github.com/sablierapp/sablier/pkg/provider/kubernetes"
	"github.com/sablierapp/sablier/pkg/provider/podman"
	"github.com/sablierapp/sablier/pkg/provider/proxmoxlxc"
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
		cli, err := newDockerClient(config.Docker)
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
	}
	return nil, fmt.Errorf("unimplemented provider %s", config.Name)
}

// newDockerClient builds a moby client from the Docker provider configuration.
//
// Each connection setting (host, API version, TLS) is taken from config first
// (populated by a CLI flag or SABLIER_PROVIDER_DOCKER_* env var) and, when unset,
// falls back to the standard Docker environment variable (DOCKER_HOST,
// DOCKER_API_VERSION, DOCKER_CERT_PATH, DOCKER_TLS_VERIFY) via the moby client's
// per-setting *FromEnv readers. This replaces the aggregate client.FromEnv option
// so every Docker connection setting is an explicit, documented Sablier option
// while the standard Docker variables keep working as a fallback.
func newDockerClient(cfg config.Docker) (*client.Client, error) {
	// client.WithTraceProvider instruments all Docker API calls via the built-in
	// otelhttp transport wrapper in the moby client.
	opts := []client.Opt{client.WithTraceProvider(otel.GetTracerProvider())}

	// TLS must be configured before the host so WithHost can reconfigure the
	// transport we install here (this mirrors the order moby uses in FromEnv).
	// TLS material comes from the Sablier option or, as a fallback, the standard
	// DOCKER_CERT_PATH variable. Verification is enabled when either the Sablier
	// flag (provider.docker.tls-verify) or DOCKER_TLS_VERIFY requests it, so the
	// flag takes effect no matter how the certificates are provided. Without this
	// the flag would be ignored whenever certificates came from DOCKER_CERT_PATH,
	// and a user relying on DOCKER_TLS_VERIFY alone would silently skip verification.
	certPath := cfg.CertPath
	if certPath == "" {
		certPath = os.Getenv("DOCKER_CERT_PATH")
	}
	if certPath != "" {
		verify := cfg.TLSVerify || os.Getenv("DOCKER_TLS_VERIFY") != ""
		httpClient, err := dockerTLSClient(certPath, verify)
		if err != nil {
			return nil, err
		}
		opts = append(opts, client.WithHTTPClient(httpClient))
	}

	if cfg.Host != "" {
		opts = append(opts, client.WithHost(cfg.Host))
	} else {
		opts = append(opts, client.WithHostFromEnv())
	}

	if cfg.APIVersion != "" {
		opts = append(opts, client.WithAPIVersion(cfg.APIVersion))
	} else {
		opts = append(opts, client.WithAPIVersionFromEnv())
	}

	return client.New(opts...)
}

// dockerTLSClient builds an HTTP client configured for a TLS-protected Docker
// daemon, loading ca.pem, cert.pem and key.pem from certPath. It mirrors the
// behavior of the moby client's WithTLSClientConfigFromEnv: mutual TLS is always
// set up, and the daemon's certificate is only verified when verify is true.
func dockerTLSClient(certPath string, verify bool) (*http.Client, error) {
	cert, err := tls.LoadX509KeyPair(filepath.Join(certPath, "cert.pem"), filepath.Join(certPath, "key.pem"))
	if err != nil {
		return nil, fmt.Errorf("load docker TLS client certificate from %q: %w", certPath, err)
	}

	tlsConfig := &tls.Config{
		Certificates:       []tls.Certificate{cert},
		MinVersion:         tls.VersionTLS12,
		InsecureSkipVerify: !verify, //nolint:gosec // verification is user-controlled via provider.docker.tls-verify / DOCKER_TLS_VERIFY
	}
	if verify {
		caPEM, err := os.ReadFile(filepath.Join(certPath, "ca.pem"))
		if err != nil {
			return nil, fmt.Errorf("read docker TLS CA certificate from %q: %w", certPath, err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(caPEM) {
			return nil, fmt.Errorf("no valid certificates found in %q", filepath.Join(certPath, "ca.pem"))
		}
		tlsConfig.RootCAs = pool
	}

	// Clone the default transport so its proxy, timeout and keep-alive settings are
	// preserved; only the TLS configuration is overridden.
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = tlsConfig
	return &http.Client{
		Transport:     transport,
		CheckRedirect: client.CheckRedirect,
	}, nil
}
