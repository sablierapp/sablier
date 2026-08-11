package systemd

import (
	"bufio"
	"cmp"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	dbus "github.com/coreos/go-systemd/v22/dbus"
	godbus "github.com/godbus/dbus/v5"
	"github.com/neilotoole/slogt"
	"gotest.tools/v3/assert"
)

// mockUnitConfig describes a unit served by the mock systemd daemon.
type mockUnitConfig struct {
	name         string
	active       bool
	fragmentPath string
	// status overrides the derived ActiveState when set.
	status string
}

// mockSystemd implements a minimal org.freedesktop.systemd1.Manager over a
// private dbus-daemon, mimicking just enough of the API for the provider.
type mockSystemd struct {
	t       *testing.T
	conn    *godbus.Conn
	addr    string
	mu      sync.Mutex
	units   map[string]*mockUnit
	files   map[string]string
	unitDir string
	started []string
	stopped []string
	props   map[string][]dbus.Property
	setErr  error
	getErr  error
	nextJob uint32
}

type mockUnit struct {
	active       bool
	fragmentPath string
	status       string
}

// unitStatus is the wire encoding of a single ListUnits entry (a(ssssssouso)).
type unitStatus struct {
	Name        string
	Description string
	LoadState   string
	ActiveState string
	SubState    string
	Followed    string
	Path        godbus.ObjectPath
	JobId       uint32
	JobType     string
	JobPath     godbus.ObjectPath
}

// unitFile is the wire encoding of a single ListUnitFiles entry (a(ss)).
type unitFile struct {
	Path string
	Type string
}

// busConfig runs a private session bus without relying on distro config files.
const busConfig = `<!DOCTYPE busconfig PUBLIC "-//freedesktop//DTD D-Bus Bus Configuration 1.0//EN"
 "http://www.freedesktop.org/standards/dbus/1.0/busconfig.dtd">
<busconfig>
  <type>session</type>
  <listen>unix:tmpdir=/tmp</listen>
  <auth>EXTERNAL</auth>
  <policy context="default">
    <allow own="*"/>
    <allow send_destination="*"/>
    <allow eavesdrop="true"/>
  </policy>
</busconfig>
`

const systemdManagerPath = "/org/freedesktop/systemd1"

func unitObjectPath(name string) godbus.ObjectPath {
	return godbus.ObjectPath("/org/freedesktop/systemd1/unit/" + dbus.PathBusEscape(name))
}

// newMockSystemd spawns a private dbus-daemon and exports the mock Manager.
func newMockSystemd(t *testing.T, configs []mockUnitConfig) *mockSystemd {
	t.Helper()
	if _, err := exec.LookPath("dbus-daemon"); err != nil {
		t.Skip("dbus-daemon not available")
	}

	configPath := filepath.Join(t.TempDir(), "bus.conf")
	assert.NilError(t, os.WriteFile(configPath, []byte(busConfig), 0o644))

	cmd := exec.Command("dbus-daemon", "--nofork", "--print-address=1", "--config-file="+configPath)
	stdout, err := cmd.StdoutPipe()
	assert.NilError(t, err)
	cmd.Stderr = os.Stderr
	assert.NilError(t, cmd.Start())
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
	})

	addrCh := make(chan string, 1)
	go func() {
		line, _ := bufio.NewReader(stdout).ReadString('\n')
		addrCh <- strings.TrimSpace(line)
	}()
	var addr string
	select {
	case addr = <-addrCh:
	case <-time.After(10 * time.Second):
		t.Fatal("dbus-daemon did not print an address")
	}

	var conn *godbus.Conn
	for i := 0; i < 20; i++ {
		conn, err = godbus.Connect(addr)
		if err == nil {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	assert.NilError(t, err)

	m := &mockSystemd{
		t:       t,
		conn:    conn,
		addr:    addr,
		units:   make(map[string]*mockUnit),
		files:   make(map[string]string),
		unitDir: t.TempDir(),
		props:   make(map[string][]dbus.Property),
	}

	_, err = conn.RequestName("org.freedesktop.systemd1", godbus.NameFlagDoNotQueue)
	assert.NilError(t, err)
	assert.NilError(t, conn.ExportAll(m, systemdManagerPath, "org.freedesktop.systemd1.Manager"))

	for _, c := range configs {
		m.AddUnit(c)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return m
}

// newProviderForTest creates a provider connected to the mock bus.
func newProviderForTest(t *testing.T, m *mockSystemd, interval time.Duration, logger *slog.Logger) *Provider {
	t.Helper()
	if logger == nil {
		logger = slogt.New(t)
	}
	con, err := dbus.NewConnection(func() (*godbus.Conn, error) {
		// go-systemd requires a fully initialised connection (auth + Hello).
		return godbus.Connect(m.addr)
	})
	assert.NilError(t, err)
	t.Cleanup(con.Close)
	p, err := New(t.Context(), con, logger)
	assert.NilError(t, err)
	p.pollInterval = interval
	return p
}

func writeUnitFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "unit.conf")
	assert.NilError(t, os.WriteFile(path, []byte(content), 0o644))
	return path
}

// SetActive toggles a unit's active state for the next poll.
func (m *mockSystemd) SetActive(name string, active bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if u, ok := m.units[name]; ok {
		u.active = active
	}
}

func (m *mockSystemd) SetStatus(name, status string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if u, ok := m.units[name]; ok {
		u.status = status
	}
}

func (m *mockSystemd) SetGetPropertiesError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.getErr = err
}

func (m *mockSystemd) SetUnitPropertiesError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.setErr = err
}

// AddUnit registers a new unit and its unit file.
func (m *mockSystemd) AddUnit(c mockUnitConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	fragmentPath := c.fragmentPath
	if c.fragmentPath != "" {
		content, err := os.ReadFile(c.fragmentPath)
		assert.NilError(m.t, err)
		fragmentPath = filepath.Join(m.unitDir, c.name)
		assert.NilError(m.t, os.WriteFile(fragmentPath, content, 0o644))
		m.files[c.name] = fragmentPath
	}
	m.units[c.name] = &mockUnit{active: c.active, fragmentPath: fragmentPath, status: c.status}
	assert.NilError(m.t, m.conn.ExportAll(&mockUnitObject{m: m, name: c.name}, unitObjectPath(c.name), "org.freedesktop.DBus.Properties"))
}

// RemoveUnit deletes a unit and its unit file.
func (m *mockSystemd) RemoveUnit(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.units, name)
	delete(m.files, name)
}

// UnloadUnit removes a unit from the listing while its file stays on disk.
func (m *mockSystemd) UnloadUnit(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.units, name)
}

func (m *mockSystemd) UnloadUnitObject(name string) {
	m.UnloadUnit(name)
	assert.NilError(m.t, m.conn.ExportAll(nil, unitObjectPath(name), "org.freedesktop.DBus.Properties"))
}

func (m *mockSystemd) Started() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]string(nil), m.started...)
}

func (m *mockSystemd) Stopped() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]string(nil), m.stopped...)
}

func (m *mockSystemd) SetProps(name string) []dbus.Property {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]dbus.Property(nil), m.props[name]...)
}

func (m *mockSystemd) ClearSetProps(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.props, name)
}

// unitState derives the ActiveState/SubState pair from a mock unit.
func unitState(u *mockUnit) (string, string) {
	if u.status != "" {
		return u.status, "unknown"
	}
	if u.active {
		return "active", "running"
	}
	return "inactive", "dead"
}

func (m *mockSystemd) statusOf(name string, u *mockUnit) unitStatus {
	activeState, subState := unitState(u)
	return unitStatus{
		Name:        name,
		Description: name,
		LoadState:   "loaded",
		ActiveState: activeState,
		SubState:    subState,
		Path:        unitObjectPath(name),
		JobPath:     "/org/freedesktop/systemd1/job/0",
	}
}

func (m *mockSystemd) ListUnits() ([]unitStatus, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]unitStatus, 0, len(m.units))
	for name, u := range m.units {
		out = append(out, m.statusOf(name, u))
	}
	slices.SortFunc(out, func(a, b unitStatus) int { return cmp.Compare(a.Name, b.Name) })
	return out, nil
}

func (m *mockSystemd) ListUnitsByNames(names []string) ([]unitStatus, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]unitStatus, 0, len(names))
	for _, name := range names {
		if u, ok := m.units[name]; ok {
			out = append(out, m.statusOf(name, u))
			continue
		}
		if fragmentPath, ok := m.files[name]; ok {
			u := &mockUnit{fragmentPath: fragmentPath}
			m.units[name] = u
			assert.NilError(m.t, m.conn.ExportAll(&mockUnitObject{m: m, name: name}, unitObjectPath(name), "org.freedesktop.DBus.Properties"))
			out = append(out, m.statusOf(name, u))
			continue
		}
		out = append(out, unitStatus{
			Name:        name,
			Description: name,
			LoadState:   "not-found",
			ActiveState: "inactive",
			SubState:    "dead",
			Path:        unitObjectPath(name),
			JobPath:     "/org/freedesktop/systemd1/job/0",
		})
	}
	return out, nil
}

func (m *mockSystemd) ListUnitsFiltered(states []string) ([]unitStatus, error) {
	units, err := m.ListUnits()
	if err != nil {
		return nil, err
	}
	out := make([]unitStatus, 0)
	for _, u := range units {
		for _, s := range states {
			if u.ActiveState == s {
				out = append(out, u)
			}
		}
	}
	return out, nil
}

func (m *mockSystemd) ListUnitFiles() ([]unitFile, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]unitFile, 0, len(m.files))
	for _, path := range m.files {
		out = append(out, unitFile{Path: path, Type: "enabled"})
	}
	return out, nil
}

func (m *mockSystemd) emitJob(jobID uint32, jobPath godbus.ObjectPath, unit, result string) {
	go func() {
		for range 5 {
			time.Sleep(5 * time.Millisecond)
			_ = m.conn.Emit(systemdManagerPath, "org.freedesktop.systemd1.Manager.JobRemoved", jobID, jobPath, unit, result)
		}
	}()
}

func (m *mockSystemd) StartUnit(name, mode string) (godbus.ObjectPath, error) {
	m.mu.Lock()
	m.started = append(m.started, name)
	if u, ok := m.units[name]; ok {
		u.active = true
	}
	m.nextJob++
	jobID := m.nextJob
	jobPath := godbus.ObjectPath(fmt.Sprintf("/org/freedesktop/systemd1/job/%d", jobID))
	m.mu.Unlock()

	m.emitJob(jobID, jobPath, name, "done")
	return jobPath, nil
}

func (m *mockSystemd) StopUnit(name, mode string) (godbus.ObjectPath, error) {
	m.mu.Lock()
	m.stopped = append(m.stopped, name)
	if u, ok := m.units[name]; ok {
		u.active = false
	}
	m.nextJob++
	jobID := m.nextJob
	jobPath := godbus.ObjectPath(fmt.Sprintf("/org/freedesktop/systemd1/job/%d", jobID))
	m.mu.Unlock()

	m.emitJob(jobID, jobPath, name, "done")
	return jobPath, nil
}

func (m *mockSystemd) SetUnitProperties(name string, runtime bool, props []dbus.Property) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.setErr != nil {
		return m.setErr
	}
	m.props[name] = append(m.props[name], props...)
	return nil
}

// mockUnitObject serves org.freedesktop.DBus.Properties.GetAll for a unit.
type mockUnitObject struct {
	m    *mockSystemd
	name string
}

func (o *mockUnitObject) GetAll(iface string) (map[string]godbus.Variant, error) {
	if iface != "org.freedesktop.systemd1.Unit" {
		return nil, nil
	}
	o.m.mu.Lock()
	defer o.m.mu.Unlock()
	if o.m.getErr != nil {
		return nil, o.m.getErr
	}
	u, ok := o.m.units[o.name]
	if !ok {
		return nil, fmt.Errorf("unit %s not loaded", o.name)
	}
	activeState, subState := unitState(u)
	return map[string]godbus.Variant{
		"Id":           godbus.MakeVariant(o.name),
		"LoadState":    godbus.MakeVariant("loaded"),
		"ActiveState":  godbus.MakeVariant(activeState),
		"SubState":     godbus.MakeVariant(subState),
		"FragmentPath": godbus.MakeVariant(u.fragmentPath),
	}, nil
}
