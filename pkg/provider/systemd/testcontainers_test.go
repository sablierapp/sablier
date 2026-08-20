package systemd_test

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/coreos/go-systemd/v22/dbus"
	godbus "github.com/godbus/dbus/v5"
	"github.com/moby/moby/api/types/container"
	"github.com/neilotoole/slogt"
	"github.com/sablierapp/sablier/pkg/provider/systemd"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// systemdContainer is the shared container with a systemd user manager used
// by the integration tests. It is initialized by TestMain, which avoids the
// overhead of starting the manager per test.
type systemdContainer struct {
	testcontainers.Container
	busSocket string
	unitDir   string
	uid       string
}

// shared is the single systemd container shared across all tests in this
// package. It is initialized by TestMain.
var shared *systemdContainer

func setupSystemd(t *testing.T) *systemd.Provider {
	t.Helper()
	shared.CreateUnit(t, testUnit, "Enable=true\nGroup=test")
	return shared.Provider(t)
}

func (c *systemdContainer) Provider(t *testing.T) *systemd.Provider {
	t.Helper()
	con, err := dbus.NewConnection(func() (*godbus.Conn, error) {
		return godbus.Connect("unix:path=" + c.busSocket)
	})
	if err != nil {
		t.Fatalf("cannot connect to systemd dbus: %v", err)
	}
	t.Cleanup(con.Close)
	p, err := systemd.NewForTest(t.Context(), con, slogt.New(t), 500*time.Millisecond, "sablier-*.service")
	if err != nil {
		t.Fatalf("cannot create provider: %v", err)
	}
	return p
}

func (c *systemdContainer) CreateUnit(t *testing.T, name, sablierSection string) {
	t.Helper()
	unit := fmt.Sprintf(`[Unit]
Description=Sablier integration test unit

[Service]
ExecStart=/bin/sleep infinity

[Install]
WantedBy=default.target

[X-Sablier]
%s
`, sablierSection)

	if err := os.WriteFile(filepath.Join(c.unitDir, "systemd/user", name), []byte(unit), 0o644); err != nil {
		t.Fatalf("cannot write unit file: %v", err)
	}
	t.Cleanup(func() {
		ctx := context.Background()
		c.systemctlBestEffort(ctx, "stop", name)
		c.systemctlBestEffort(ctx, "reset-failed", name)
		c.systemctlBestEffort(ctx, "disable", name)
		_ = os.Remove(filepath.Join(c.unitDir, "systemd/user", name))
		c.systemctlBestEffort(ctx, "daemon-reload")
	})

	c.systemctl(t, "daemon-reload")
	c.systemctl(t, "enable", name)
	c.systemctl(t, "daemon-reload")
}

func (c *systemdContainer) systemctl(t *testing.T, args ...string) {
	t.Helper()
	code, output, err := c.runSystemctl(t.Context(), args...)
	if err != nil || code != 0 {
		t.Fatalf("systemctl %v failed (code=%d): %v\n%s", args, code, err, output)
	}
}

func (c *systemdContainer) systemctlBestEffort(ctx context.Context, args ...string) {
	code, output, err := c.runSystemctl(ctx, args...)
	if err != nil || code != 0 {
		log.Printf("systemctl %v failed (code=%d): %v\n%s", args, code, err, output)
	}
}

func (c *systemdContainer) runSystemctl(ctx context.Context, args ...string) (int, string, error) {
	cmd := fmt.Sprintf("env XDG_RUNTIME_DIR=%s XDG_CONFIG_HOME=%s systemctl --user %s",
		c.unitDir, filepath.Join(c.unitDir, ".config"), strings.Join(args, " "))
	exec := []string{"setpriv", "--reuid", c.uid, "--regid", c.uid, "--clear-groups", "sh", "-c", cmd}
	code, output, err := c.Exec(ctx, exec)
	if output == nil {
		return code, "", err
	}
	b, readErr := io.ReadAll(output)
	if err == nil {
		err = readErr
	}
	return code, string(b), err
}

func TestMain(m *testing.M) {
	// flag.Parse must be called before testing.Short() is usable.
	flag.Parse()

	// Skip the expensive container setup when running in short mode.
	if testing.Short() || os.Getenv("SABLIER_SYSTEMD_INTEGRATION") == "" {
		os.Exit(m.Run())
	}
	log.Print("systemd integration tests mount the host cgroup filesystem read-write; run them only on a disposable development host")

	ctx := context.Background()

	unitDir, err := os.MkdirTemp("", "sablier-units-")
	if err != nil {
		log.Fatalf("failed to create unit dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(unitDir) }()
	if err := os.MkdirAll(filepath.Join(unitDir, "systemd/user"), 0o755); err != nil {
		log.Fatalf("failed to create unit path: %v", err)
	}

	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		log.Fatalf("cannot determine test source path")
	}
	initScript := filepath.Join(filepath.Dir(thisFile), "testdata", "init.sh")

	req := testcontainers.ContainerRequest{
		Image: "jrei/systemd-ubuntu:24.04",
		Cmd:   []string{"/sd-init.sh"},
		Env: map[string]string{
			"TEST_UID":   fmt.Sprint(os.Getuid()),
			"TEST_UNITS": unitDir,
		},
		HostConfigModifier: func(hc *container.HostConfig) {
			// systemd manages cgroups itself: the container sees the real
			// cgroup filesystem, and init.sh delegates the container's cgroup
			// subtree to the test uid. No privileges are required.
			hc.CgroupnsMode = container.CgroupnsModeHost
			hc.Binds = append(
				hc.Binds,
				"/sys/fs/cgroup:/sys/fs/cgroup:rw",
				unitDir+":"+unitDir,
				initScript+":/sd-init.sh:ro",
			)
		},
		WaitingFor: wait.ForExec([]string{"test", "-S", unitDir + "/bus"}),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		log.Fatalf("failed to start systemd container: %v", err)
	}

	shared = &systemdContainer{
		Container: container,
		busSocket: filepath.Join(unitDir, "bus"),
		unitDir:   unitDir,
		uid:       fmt.Sprint(os.Getuid()),
	}
	if err := waitForManager(shared.busSocket); err != nil {
		_ = container.Terminate(ctx)
		log.Fatalf("systemd manager not reachable: %v", err)
	}

	code := m.Run()
	_ = container.Terminate(ctx)
	os.Exit(code)
}

func waitForManager(busSocket string) error {
	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(busSocket); err != nil {
			time.Sleep(500 * time.Millisecond)
			continue
		}
		con, err := dbus.NewConnection(func() (*godbus.Conn, error) {
			return godbus.Connect("unix:path=" + busSocket)
		})
		if err == nil {
			if _, err := con.GetManagerProperty("Version"); err == nil {
				con.Close()
				return nil
			}
			con.Close()
		}
		time.Sleep(500 * time.Millisecond)
	}
	return fmt.Errorf("timed out waiting for systemd manager")
}
