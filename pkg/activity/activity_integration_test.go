//go:build linux && (amd64 || arm64)

// Package activity_test runs activity tracking end to end: the Docker
// provider, the eBPF tracker and Sablier's WatchActivity loop, against a real
// Docker daemon and a real kernel. It needs root:
//
//	go test -tags=nomsgpack -exec sudo -run TestActivity ./pkg/activity/
package activity_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/link"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"
	"github.com/sablierapp/sablier/pkg/activity/bpf"
	"github.com/sablierapp/sablier/pkg/activity/local"
	"github.com/sablierapp/sablier/pkg/provider/docker"
	"github.com/sablierapp/sablier/pkg/sablier"
	"github.com/sablierapp/sablier/pkg/store"
	"github.com/sablierapp/sablier/pkg/store/inmemory"
	"github.com/testcontainers/testcontainers-go"
	tcexec "github.com/testcontainers/testcontainers-go/exec"
	"github.com/testcontainers/testcontainers-go/network"
	"github.com/testcontainers/testcontainers-go/wait"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/poll"
)

const (
	nginxImage = "nginx:alpine"
	curlImage  = "curlimages/curl:8.17.0"

	// interval is the WatchActivity interval: how often traffic is drained.
	interval = time.Second
)

// env is a Sablier tracking activity on the local Docker daemon, with its own
// tracker. It only runs WatchActivity: it never stops containers.
type env struct {
	cli      *client.Client
	provider *docker.Provider
	store    sablier.Store
	sablier  *sablier.Sablier
	network  *testcontainers.DockerNetwork
}

func setup(t *testing.T) *env {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}
	if os.Geteuid() != 0 {
		t.Skip("needs root: go test -exec sudo")
	}
	ctx := t.Context()
	logger := testLogger(t)

	cli, err := client.New(client.FromEnv)
	assert.NilError(t, err)
	t.Cleanup(func() { _ = cli.Close() })

	provider, err := docker.New(ctx, cli, logger, "stop")
	assert.NilError(t, err)

	nw, err := network.New(ctx)
	assert.NilError(t, err)
	testcontainers.CleanupNetwork(t, nw)

	sessions := inmemory.NewInMemory()
	s := sablier.New(logger, sessions, provider)

	tracker, err := bpf.New()
	assert.NilError(t, err)
	src := local.New(tracker, provider, nil, logger)

	watchCtx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		s.WatchActivity(watchCtx, src, interval)
	}()
	t.Cleanup(func() {
		cancel()
		<-done
		assert.NilError(t, tracker.Close())
	})

	return &env{cli: cli, provider: provider, store: sessions, sablier: s, network: nw}
}

// nginx is an activity-tracked nginx container.
type nginx struct {
	*testcontainers.DockerContainer
	name string
}

// startNginx starts an nginx opted into activity tracking, in group, and
// registers the groups with Sablier so activity can refresh group sessions.
func (e *env) startNginx(t *testing.T, group string) nginx {
	t.Helper()
	ctx := t.Context()

	c, err := testcontainers.Run(ctx, nginxImage,
		testcontainers.WithLabels(map[string]string{
			sablier.LabelEnable:                  "true",
			sablier.LabelGroup:                   group,
			sablier.LabelActivityEnable:          "true",
			sablier.LabelActivityMinBytes:        "1",
			sablier.LabelActivitySessionDuration: "1m",
		}),
		network.WithNetwork(nil, e.network),
		testcontainers.WithWaitStrategy(wait.ForLog("start worker process")),
	)
	testcontainers.CleanupContainer(t, c)
	assert.NilError(t, err)

	insp, err := c.Inspect(ctx)
	assert.NilError(t, err)

	groups, err := e.provider.InstanceGroups(ctx)
	assert.NilError(t, err)
	e.sablier.SetGroups(groups)

	return nginx{DockerContainer: c, name: strings.TrimPrefix(insp.Name, "/")}
}

// ip returns the container's current address on the test network.
func (e *env) ip(t *testing.T, c *testcontainers.DockerContainer) string {
	t.Helper()
	insp, err := e.cli.ContainerInspect(t.Context(), c.GetContainerID(), client.ContainerInspectOptions{})
	assert.NilError(t, err)
	ep, ok := insp.Container.NetworkSettings.Networks[e.network.Name]
	assert.Assert(t, ok && ep.IPAddress.IsValid(), "container is not on network %s", e.network.Name)
	return ep.IPAddress.String()
}

// startCurl starts an idle curl container on the test network.
func (e *env) startCurl(t *testing.T, labels map[string]string) *testcontainers.DockerContainer {
	t.Helper()
	c, err := testcontainers.Run(t.Context(), curlImage,
		testcontainers.WithLabels(labels),
		testcontainers.WithEntrypoint("sleep", "3600"),
		// An init process makes sleep exit on the stop signal.
		testcontainers.WithHostConfigModifier(func(hc *container.HostConfig) {
			useInit := true
			hc.Init = &useInit
		}),
		network.WithNetwork(nil, e.network),
	)
	testcontainers.CleanupContainer(t, c)
	assert.NilError(t, err)
	return c
}

// get requests http://ip/ from the curl container, retrying while nginx
// isn't listening yet.
func get(t *testing.T, curl *testcontainers.DockerContainer, ip string) {
	t.Helper()
	code, out, err := curl.Exec(t.Context(), []string{
		"curl", "-sS", "-o", "/dev/null", "--max-time", "5",
		"--retry", "5", "--retry-connrefused", "--retry-delay", "1",
		"http://" + ip + "/",
	}, tcexec.Multiplexed())
	assert.NilError(t, err)
	if code != 0 {
		b, _ := io.ReadAll(out)
		t.Fatalf("GET http://%s/ exited with %d: %s", ip, code, b)
	}
}

// session reports whether the store holds a session for name.
func (e *env) session(ctx context.Context, name string) (bool, error) {
	_, err := e.store.Get(ctx, name)
	if errors.Is(err, store.ErrKeyNotFound) {
		return false, nil
	}
	return err == nil, err
}

func (e *env) hasSession(t *testing.T, name string) bool {
	t.Helper()
	ok, err := e.session(t.Context(), name)
	assert.NilError(t, err)
	return ok
}

// waitSession waits up to 10s for a session for name.
func (e *env) waitSession(t *testing.T, name string) {
	t.Helper()
	poll.WaitOn(t, func(poll.LogT) poll.Result {
		ok, err := e.session(t.Context(), name)
		switch {
		case err != nil:
			return poll.Error(err)
		case !ok:
			return poll.Continue("no session for %s", name)
		}
		return poll.Success()
	}, poll.WithTimeout(10*time.Second))
}

// counterLinks returns the IDs of the BPF links whose program is one of the
// tracker's counters, across the whole kernel.
func counterLinks() ([]link.ID, error) {
	it := new(link.Iterator)
	defer it.Close()
	var ids []link.ID
	for it.Next() {
		info, err := it.Link.Info()
		if err != nil {
			return nil, err
		}
		prog, err := ebpf.NewProgramFromID(info.Program)
		if errors.Is(err, os.ErrNotExist) {
			continue // detached since the iterator found it
		}
		if err != nil {
			return nil, err
		}
		pinfo, err := prog.Info()
		_ = prog.Close()
		if err != nil {
			return nil, err
		}
		if pinfo.Name == "count_ingress" || pinfo.Name == "count_egress" {
			ids = append(ids, it.ID)
		}
	}
	return ids, it.Err()
}

// waitLinks waits for exactly n counter links, none of them in old, and
// returns them.
func waitLinks(t *testing.T, n int, old []link.ID, timeout time.Duration) []link.ID {
	t.Helper()
	var ids []link.ID
	poll.WaitOn(t, func(poll.LogT) poll.Result {
		var err error
		ids, err = counterLinks()
		switch {
		case err != nil:
			return poll.Error(err)
		case len(ids) != n || slices.ContainsFunc(ids, func(id link.ID) bool { return slices.Contains(old, id) }):
			return poll.Continue("counter links %v, want %d new ones (old %v)", ids, n, old)
		}
		return poll.Success()
	}, poll.WithTimeout(timeout), poll.WithDelay(100*time.Millisecond))
	return ids
}

func TestActivity_TrafficOpensSession(t *testing.T) {
	e := setup(t)
	web := e.startNginx(t, "activity-traffic")
	waitLinks(t, 2, nil, 10*time.Second)
	curl := e.startCurl(t, nil)
	assert.Assert(t, !e.hasSession(t, web.name), "no traffic yet, no session")

	get(t, curl, e.ip(t, web.DockerContainer))

	e.waitSession(t, web.name)
}

func TestActivity_IgnoredSourceDoesNotOpenSession(t *testing.T) {
	e := setup(t)
	web := e.startNginx(t, "activity-ignored")
	waitLinks(t, 2, nil, 10*time.Second)
	ip := e.ip(t, web.DockerContainer)
	ignored := e.startCurl(t, map[string]string{sablier.LabelActivityIgnoreSource: "true"})
	// The ignore list is refreshed every interval: let it pick up the poller.
	time.Sleep(3 * interval)

	for range 5 {
		get(t, ignored, ip)
		time.Sleep(time.Second)
	}
	time.Sleep(2 * interval) // let the last request be drained

	assert.Assert(t, !e.hasSession(t, web.name), "traffic from an ignored source opened a session")

	// The same requests from a container that is not ignored do count.
	get(t, e.startCurl(t, nil), ip)
	e.waitSession(t, web.name)
}

func TestActivity_StopDetaches(t *testing.T) {
	e := setup(t)
	web := e.startNginx(t, "activity-stop")
	waitLinks(t, 2, nil, 10*time.Second)

	timeout := 10 * time.Second
	assert.NilError(t, web.Stop(t.Context(), &timeout))

	waitLinks(t, 0, nil, 3*time.Second)
}

func TestActivity_RestartReattaches(t *testing.T) {
	e := setup(t)
	web := e.startNginx(t, "activity-restart")
	before := waitLinks(t, 2, nil, 10*time.Second)
	curl := e.startCurl(t, nil)
	get(t, curl, e.ip(t, web.DockerContainer))
	e.waitSession(t, web.name)

	// Forget the session, as if it expired, then restart the container: its
	// cgroup is recreated, so only a new watch can count its traffic.
	assert.NilError(t, e.store.Delete(t.Context(), web.name))
	_, err := e.cli.ContainerRestart(t.Context(), web.GetContainerID(), client.ContainerRestartOptions{})
	assert.NilError(t, err)
	waitLinks(t, 2, before, 10*time.Second)
	assert.Assert(t, !e.hasSession(t, web.name), "no traffic since the restart, no session")

	get(t, curl, e.ip(t, web.DockerContainer))

	e.waitSession(t, web.name)
}

// testLogger logs through t until the test's cleanup ends, then drops the
// records: Sablier's async starts can log after the test returns.
func testLogger(t *testing.T) *slog.Logger {
	w := &testWriter{t: t}
	t.Cleanup(w.stop)
	return slog.New(slog.NewTextHandler(w, &slog.HandlerOptions{Level: slog.LevelInfo}))
}

type testWriter struct {
	mu   sync.Mutex
	t    *testing.T
	done bool
}

func (w *testWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.done {
		w.t.Log(strings.TrimSuffix(string(p), "\n"))
	}
	return len(p), nil
}

func (w *testWriter) stop() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.done = true
}
