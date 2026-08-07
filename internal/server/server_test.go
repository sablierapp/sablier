package server

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/neilotoole/slogt"
	"github.com/sablierapp/sablier/internal/api"
	"github.com/sablierapp/sablier/internal/api/apitest"
	"github.com/sablierapp/sablier/pkg/config"
	"github.com/sablierapp/sablier/pkg/metrics"
	"github.com/sablierapp/sablier/pkg/sablier"
	"github.com/sablierapp/sablier/pkg/theme"
	"go.uber.org/mock/gomock"
	"gotest.tools/v3/assert"
)

func testStrategy(t *testing.T) *api.ServeStrategy {
	t.Helper()
	th, err := theme.New(slogt.New(t))
	assert.NilError(t, err)
	return &api.ServeStrategy{
		Theme:          th,
		Metrics:        metrics.Noop{},
		StrategyConfig: config.NewStrategyConfig(),
		SessionsConfig: config.NewSessionsConfig(),
	}
}

func readySession() *sablier.SessionState {
	return &sablier.SessionState{Instances: map[string]sablier.InstanceInfoWithError{
		"test": {Instance: sablier.InstanceInfo{Name: "test", Status: sablier.InstanceStatusReady, CurrentReplicas: 1, DesiredReplicas: 1}},
	}}
}

// freePort reserves an ephemeral port and releases it for the server to bind.
func freePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	assert.NilError(t, err)
	port := l.Addr().(*net.TCPAddr).Port
	assert.NilError(t, l.Close())
	return port
}

func waitReachable(url string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		resp, err := http.Get(url) //nolint:gosec // local test URL
		if err == nil {
			_ = resp.Body.Close()
			return nil
		}
		time.Sleep(25 * time.Millisecond)
	}
	return fmt.Errorf("%s not reachable within %s", url, timeout)
}

// TestStartReturnsOnServeError pins the port-conflict behavior: when the
// listener cannot bind, Start must return the error instead of logging it and
// leaving the caller blocked forever with no HTTP server (a zombie process
// that looks healthy but serves nothing).
func TestStartReturnsOnServeError(t *testing.T) {
	// Occupy the wildcard address, which is what the server binds.
	l, err := net.Listen("tcp", ":0")
	assert.NilError(t, err)
	defer l.Close() //nolint:errcheck
	port := l.Addr().(*net.TCPAddr).Port

	conf := config.NewServerConfig()
	conf.Port = port // already taken by l

	done := make(chan error, 1)
	go func() {
		done <- Start(t.Context(), slogt.New(t), conf, config.NewTracingConfig(), testStrategy(t))
	}()

	select {
	case err := <-done:
		assert.ErrorContains(t, err, "server:")
	case <-time.After(5 * time.Second):
		t.Fatal("Start did not return after a bind failure")
	}
}

// TestStartDrainsInFlightRequestsOnShutdown pins graceful shutdown: a request
// being processed when ctx is cancelled must complete with a full response.
// The previous implementation called server.Close, which resets live
// connections; the blocking strategy holds requests open by design, so every
// restart cut them off mid-wait.
func TestStartDrainsInFlightRequestsOnShutdown(t *testing.T) {
	port := freePort(t)
	conf := config.NewServerConfig()
	conf.Port = port

	strategy := testStrategy(t)
	ctrl := gomock.NewController(t)
	mock := apitest.NewMockSablier(ctrl)
	strategy.Sablier = mock

	inFlight := make(chan struct{}) // closed when the handler enters the session call
	release := make(chan struct{})  // closed by the test after shutdown has started
	mock.EXPECT().RequestReadySession(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(context.Context, []string, time.Duration, time.Duration) (*sablier.SessionState, error) {
			close(inFlight)
			<-release
			return readySession(), nil
		})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		done <- Start(ctx, slogt.New(t), conf, config.NewTracingConfig(), strategy)
	}()

	base := fmt.Sprintf("http://127.0.0.1:%d", port)
	assert.NilError(t, waitReachable(base+"/health", 5*time.Second))

	respC := make(chan error, 1)
	go func() {
		resp, err := http.Get(base + "/api/strategies/blocking?names=test")
		if err != nil {
			respC <- err
			return
		}
		defer resp.Body.Close() //nolint:errcheck
		if _, err := io.ReadAll(resp.Body); err != nil {
			respC <- err
			return
		}
		if resp.StatusCode != http.StatusOK {
			respC <- fmt.Errorf("unexpected status %d", resp.StatusCode)
			return
		}
		respC <- nil
	}()

	<-inFlight // the request is provably being processed
	cancel()   // shutdown begins while the request is held

	// Give the shutdown path time to act on the connection: the old Close-based
	// implementation resets it here, the drain-based one waits.
	time.Sleep(100 * time.Millisecond)
	close(release) // let the handler finish

	assert.NilError(t, <-respC, "in-flight request must complete during the drain window")

	select {
	case err := <-done:
		assert.NilError(t, err)
	case <-time.After(30 * time.Second):
		t.Fatal("Start did not return after shutdown")
	}
}

func TestDrainTimeout(t *testing.T) {
	// The floor applies when the blocking timeout is short.
	assert.Equal(t, minDrainTimeout, drainTimeout(time.Second))
	// Long blocking timeouts extend the window by the grace period.
	assert.Equal(t, 5*time.Minute+drainGrace, drainTimeout(5*time.Minute))
}
