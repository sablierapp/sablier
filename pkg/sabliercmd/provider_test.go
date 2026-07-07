package sabliercmd

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"log/slog"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/sablierapp/sablier/pkg/config"
)

func TestNewDockerClient(t *testing.T) {
	// Every branch of newDockerClient. moby's client.New is lazy (it does not
	// dial the daemon), so a client is returned without a running Docker.
	// Isolate from any ambient Docker TLS environment so the branches are deterministic.
	t.Setenv("DOCKER_CERT_PATH", "")
	t.Setenv("DOCKER_TLS_VERIFY", "")

	t.Run("env fallback", func(t *testing.T) {
		cli, err := newDockerClient(config.Docker{})
		if err != nil {
			t.Fatalf("newDockerClient: %v", err)
		}
		if cli == nil {
			t.Fatal("expected a client")
		}
	})

	t.Run("explicit host and api version", func(t *testing.T) {
		cli, err := newDockerClient(config.Docker{Host: "tcp://127.0.0.1:2375", APIVersion: "1.45"})
		if err != nil {
			t.Fatalf("newDockerClient: %v", err)
		}
		if cli == nil {
			t.Fatal("expected a client")
		}
	})

	t.Run("with TLS cert path", func(t *testing.T) {
		dir := t.TempDir()
		writeTestCerts(t, dir)
		cli, err := newDockerClient(config.Docker{Host: "tcp://127.0.0.1:2376", CertPath: dir, TLSVerify: true})
		if err != nil {
			t.Fatalf("newDockerClient: %v", err)
		}
		if cli == nil {
			t.Fatal("expected a client")
		}
	})

	t.Run("cert path from DOCKER_CERT_PATH env", func(t *testing.T) {
		dir := t.TempDir()
		writeTestCerts(t, dir)
		// Certificates provided via the standard env var, verification via the flag.
		t.Setenv("DOCKER_CERT_PATH", dir)
		cli, err := newDockerClient(config.Docker{Host: "tcp://127.0.0.1:2376", TLSVerify: true})
		if err != nil {
			t.Fatalf("newDockerClient: %v", err)
		}
		if cli == nil {
			t.Fatal("expected a client")
		}
	})

	t.Run("invalid cert path", func(t *testing.T) {
		if _, err := newDockerClient(config.Docker{CertPath: filepath.Join(t.TempDir(), "nope")}); err == nil {
			t.Fatal("expected an error for a missing cert path")
		}
	})
}

func TestDockerTLSClient(t *testing.T) {
	dir := t.TempDir()
	writeTestCerts(t, dir)

	t.Run("verify loads the CA", func(t *testing.T) {
		c, err := dockerTLSClient(dir, true)
		if err != nil {
			t.Fatalf("dockerTLSClient: %v", err)
		}
		if c == nil {
			t.Fatal("expected an http client")
		}
	})

	t.Run("without verify skips the CA", func(t *testing.T) {
		c, err := dockerTLSClient(dir, false)
		if err != nil {
			t.Fatalf("dockerTLSClient: %v", err)
		}
		if c == nil {
			t.Fatal("expected an http client")
		}
	})

	t.Run("missing key pair errors", func(t *testing.T) {
		if _, err := dockerTLSClient(filepath.Join(t.TempDir(), "empty"), true); err == nil {
			t.Fatal("expected an error loading the key pair")
		}
	})

	t.Run("invalid CA errors", func(t *testing.T) {
		bad := t.TempDir()
		writeTestCerts(t, bad)
		// Overwrite the CA with a file that holds no valid certificate.
		if err := os.WriteFile(filepath.Join(bad, "ca.pem"), []byte("not a certificate"), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := dockerTLSClient(bad, true); err == nil {
			t.Fatal("expected an error for an invalid CA")
		}
	})
}

func TestSetupProviderInvalidConfig(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	if _, err := setupProvider(context.Background(), logger, config.Provider{Name: ""}); err == nil {
		t.Fatal("expected an error for an invalid provider configuration")
	}
}

// writeTestCerts writes a self-signed cert.pem, key.pem and ca.pem into dir so
// dockerTLSClient can load a valid mutual-TLS key pair and CA.
func writeTestCerts(t *testing.T, dir string) {
	t.Helper()

	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "sablier-test"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &priv.PublicKey, priv)
	if err != nil {
		t.Fatal(err)
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})

	keyDER, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		t.Fatal(err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})

	for name, data := range map[string][]byte{"cert.pem": certPEM, "key.pem": keyPEM, "ca.pem": certPEM} {
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
}
