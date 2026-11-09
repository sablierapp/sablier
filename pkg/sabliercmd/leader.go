package sabliercmd

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/sablierapp/sablier/pkg/config"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	k8s "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
)

const (
	serviceAccountNamespaceFile = "/var/run/secrets/kubernetes.io/serviceaccount/namespace"

	// LeaderLabel is set to "true" on the leader's pod. A Service that selects
	// it routes traffic to the leader only, while every replica stays ready,
	// so Deployment rollouts and `helm --wait` are unaffected.
	LeaderLabel = "sablierapp.dev/leader"
)

// errLeadershipLost is returned by Start when this replica stops being the
// leader while it is still supposed to run. Exiting, and letting the
// orchestrator restart the process as a follower, is simpler and safer than
// tearing down and rebuilding every watcher in place.
var errLeadershipLost = errors.New("leader election: leadership lost")

type leaderElection struct {
	conf      config.LeaderElection
	cli       k8s.Interface
	l         *slog.Logger
	pod       string
	namespace string
}

// newLeaderElection connects to the API server and removes the leader label
// from this replica's pod. A restarted leader keeps its pod, and with it the
// label, so the label must go before the server starts accepting traffic.
func newLeaderElection(ctx context.Context, logger *slog.Logger, conf config.LeaderElection) (*leaderElection, error) {
	restConfig, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("leader election: %w", err)
	}
	cli, err := k8s.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("leader election: %w", err)
	}

	namespace, err := podNamespace()
	if err != nil {
		return nil, err
	}
	pod := os.Getenv("POD_NAME")
	if pod == "" {
		if pod, err = os.Hostname(); err != nil {
			return nil, fmt.Errorf("leader election: cannot determine pod name: %w", err)
		}
	}
	if conf.LeaseNamespace == "" {
		conf.LeaseNamespace = namespace
	}

	le := &leaderElection{
		conf:      conf,
		cli:       cli,
		pod:       pod,
		namespace: namespace,
		l: logger.With(
			slog.String("lease", conf.LeaseNamespace+"/"+conf.LeaseName),
			slog.String("identity", pod),
		),
	}
	if err := le.setLeaderLabel(ctx, false); err != nil {
		return nil, err
	}
	return le, nil
}

// run blocks until ctx is cancelled or leadership is lost. lead is called with
// a context that is cancelled when leadership ends; it must return quickly
// (start goroutines tied to that context). lost is closed when leadership was
// lost while ctx is still alive.
func (le *leaderElection) run(ctx context.Context, lead func(ctx context.Context), lost chan<- struct{}) error {
	elector, err := leaderelection.NewLeaderElector(leaderelection.LeaderElectionConfig{
		Lock: &resourcelock.LeaseLock{
			LeaseMeta:  metav1.ObjectMeta{Name: le.conf.LeaseName, Namespace: le.conf.LeaseNamespace},
			Client:     le.cli.CoordinationV1(),
			LockConfig: resourcelock.ResourceLockConfig{Identity: le.pod},
		},
		LeaseDuration: le.conf.LeaseDuration,
		RenewDeadline: le.conf.RenewDeadline,
		RetryPeriod:   le.conf.RetryPeriod,
		// Release the Lease on shutdown so a rolling update hands over
		// immediately instead of waiting for the lease to expire.
		ReleaseOnCancel: true,
		Name:            le.conf.LeaseName,
		Callbacks: leaderelection.LeaderCallbacks{
			OnStartedLeading: func(ctx context.Context) {
				le.l.InfoContext(ctx, "leader election: acquired leadership")
				lead(ctx)
			},
			OnStoppedLeading: func() {
				if ctx.Err() != nil {
					le.l.Info("leader election: released leadership on shutdown")
					return
				}
				le.l.Error("leader election: lost leadership")
				close(lost)
			},
			OnNewLeader: func(id string) {
				if id != le.pod {
					le.l.Info("leader election: following", slog.String("leader", id))
				}
			},
		},
	})
	if err != nil {
		return fmt.Errorf("leader election: %w", err)
	}

	le.l.Info("leader election: waiting for leadership")
	elector.Run(ctx)
	return nil
}

// setLeaderLabel adds or removes LeaderLabel on this replica's pod.
func (le *leaderElection) setLeaderLabel(ctx context.Context, leader bool) error {
	value := "null"
	if leader {
		value = `"true"`
	}
	patch := fmt.Sprintf(`{"metadata":{"labels":{%q:%s}}}`, LeaderLabel, value)
	_, err := le.cli.CoreV1().Pods(le.namespace).Patch(ctx, le.pod, types.MergePatchType, []byte(patch), metav1.PatchOptions{})
	if err != nil {
		return fmt.Errorf("leader election: cannot label pod %s/%s (needs patch on pods): %w", le.namespace, le.pod, err)
	}
	return nil
}

// podNamespace returns $POD_NAMESPACE, then the mounted service account namespace.
func podNamespace() (string, error) {
	if ns := os.Getenv("POD_NAMESPACE"); ns != "" {
		return ns, nil
	}
	b, err := os.ReadFile(serviceAccountNamespaceFile)
	if err != nil {
		return "", fmt.Errorf("leader election: cannot determine the pod namespace, set POD_NAMESPACE: %w", err)
	}
	if ns := strings.TrimSpace(string(b)); ns != "" {
		return ns, nil
	}
	return "", errors.New("leader election: service account namespace file is empty")
}
