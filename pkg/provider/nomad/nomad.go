package nomad

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/nomad/api"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"

	"github.com/sablierapp/sablier/pkg/sablier"
)

// Interface guard
var _ sablier.Provider = (*Provider)(nil)

// Provider implements the sablier.Provider interface for Nomad task groups.
// An instance is one task group named "job/group", started and stopped by scaling its count.
type Provider struct {
	Client    *api.Client
	namespace string
	l         *slog.Logger
	tracer    trace.Tracer

	// backoffMin and backoffMax bound the delay between two event stream
	// connection attempts.
	backoffMin time.Duration
	backoffMax time.Duration

	jobs jobCache
}

// New creates a new Nomad provider and verifies the connection. An empty
// namespace selects the Nomad default namespace.
func New(ctx context.Context, client *api.Client, namespace string, logger *slog.Logger) (*Provider, error) {
	if namespace == "" {
		namespace = api.DefaultNamespace
	}
	logger = logger.With(slog.String("provider", "nomad"))

	p := &Provider{
		Client:     client,
		namespace:  namespace,
		l:          logger,
		tracer:     otel.Tracer("github.com/sablierapp/sablier/pkg/provider/nomad"),
		backoffMin: time.Second,
		backoffMax: 30 * time.Second,
		jobs:       jobCache{jobs: make(map[string]cachedJob)},
	}

	// Listing the jobs verifies the address, the ACL token and the namespace at once.
	if _, _, err := client.Jobs().List(p.queryOptions(ctx)); err != nil {
		return nil, fmt.Errorf("cannot connect to Nomad at %s: %w", client.Address(), err)
	}

	logger.InfoContext(ctx, "connection established with Nomad",
		slog.String("address", client.Address()),
		slog.String("namespace", namespace),
	)
	return p, nil
}

func (p *Provider) InstanceDependencies(_ context.Context, _ string) ([]sablier.InstanceDependency, error) {
	return nil, nil
}

func (p *Provider) queryOptions(ctx context.Context) *api.QueryOptions {
	return (&api.QueryOptions{Namespace: p.namespace}).WithContext(ctx)
}

func (p *Provider) writeOptions(ctx context.Context) *api.WriteOptions {
	return (&api.WriteOptions{Namespace: p.namespace}).WithContext(ctx)
}

// parsedName is an instance name split into its job ID and task group name.
type parsedName struct {
	JobID string
	Group string
}

// parseName splits a "job/group" instance name. Dispatched and periodic
// child jobs carry a "/" in their ID, so the split is on the last separator.
func parseName(name string) (parsedName, error) {
	i := strings.LastIndex(name, "/")
	if i <= 0 || i == len(name)-1 {
		return parsedName{}, fmt.Errorf("instance name %q must be in the form job/group", name)
	}
	return parsedName{JobID: name[:i], Group: name[i+1:]}, nil
}

func instanceName(jobID, group string) string {
	return jobID + "/" + group
}

// groupMeta returns the effective sablier labels of a task group: the job
// meta with the task group meta on top, the way Nomad merges them for tasks.
func groupMeta(job *api.Job, tg *api.TaskGroup) map[string]string {
	meta := make(map[string]string, len(job.Meta)+len(tg.Meta))
	maps.Copy(meta, job.Meta)
	maps.Copy(meta, tg.Meta)
	return meta
}

func findGroup(job *api.Job, group string) *api.TaskGroup {
	for _, tg := range job.TaskGroups {
		if deref(tg.Name) == group {
			return tg
		}
	}
	return nil
}

func groupNames(job *api.Job) []string {
	names := make([]string, 0, len(job.TaskGroups))
	for _, tg := range job.TaskGroups {
		names = append(names, deref(tg.Name))
	}
	return names
}

// getJob fetches the full job spec.
func (p *Provider) getJob(ctx context.Context, jobID string) (*api.Job, error) {
	job, _, err := p.Client.Jobs().Info(jobID, p.queryOptions(ctx))
	if err != nil {
		if isNotFound(err) {
			return nil, fmt.Errorf("job %q not found in namespace %q", jobID, p.namespace)
		}
		return nil, fmt.Errorf("cannot get job %q: %w", jobID, err)
	}
	return job, nil
}

// getGroup resolves an instance name to its job spec and task group.
func (p *Provider) getGroup(ctx context.Context, name string) (*api.Job, *api.TaskGroup, error) {
	parsed, err := parseName(name)
	if err != nil {
		// Help the user out when only the job ID was given.
		if !strings.Contains(name, "/") {
			if job, jerr := p.getJob(ctx, name); jerr == nil {
				return nil, nil, fmt.Errorf("%w (job %q has task groups %v)", err, name, groupNames(job))
			}
		}
		return nil, nil, err
	}

	job, err := p.getJob(ctx, parsed.JobID)
	if err != nil {
		return nil, nil, err
	}

	tg := findGroup(job, parsed.Group)
	if tg == nil {
		return nil, nil, fmt.Errorf("task group %q not found in job %q (task groups: %v)", parsed.Group, parsed.JobID, groupNames(job))
	}
	return job, tg, nil
}

func isNotFound(err error) bool {
	var ure api.UnexpectedResponseError
	return errors.As(err, &ure) && ure.StatusCode() == http.StatusNotFound
}

func deref[T any](p *T) T {
	if p == nil {
		var zero T
		return zero
	}
	return *p
}

// jobCache keeps the full job specs between listings. A job is fetched again
// only when its ModifyIndex changed, so a stable cluster costs one list call.
type jobCache struct {
	mu   sync.Mutex
	jobs map[string]cachedJob
}

type cachedJob struct {
	modifyIndex uint64
	job         *api.Job
}

// listJobs returns the full spec of every job of the namespace and the Raft index
// of the listing. Periodic and parameterized parent jobs never run and are skipped.
func (p *Provider) listJobs(ctx context.Context) ([]*api.Job, uint64, error) {
	stubs, meta, err := p.Client.Jobs().List(p.queryOptions(ctx))
	if err != nil {
		return nil, 0, fmt.Errorf("cannot list jobs: %w", err)
	}

	p.jobs.mu.Lock()
	defer p.jobs.mu.Unlock()

	jobs := make([]*api.Job, 0, len(stubs))
	seen := make(map[string]bool, len(stubs))
	for _, stub := range stubs {
		if stub.Periodic || stub.ParameterizedJob {
			continue
		}
		seen[stub.ID] = true
		if cached, ok := p.jobs.jobs[stub.ID]; ok && cached.modifyIndex == stub.ModifyIndex {
			jobs = append(jobs, cached.job)
			continue
		}
		job, _, err := p.Client.Jobs().Info(stub.ID, p.queryOptions(ctx))
		if err != nil {
			if isNotFound(err) {
				// The job was removed between the list and the fetch.
				continue
			}
			return nil, 0, fmt.Errorf("cannot get job %q: %w", stub.ID, err)
		}
		p.jobs.jobs[stub.ID] = cachedJob{modifyIndex: stub.ModifyIndex, job: job}
		jobs = append(jobs, job)
	}
	for id := range p.jobs.jobs {
		if !seen[id] {
			delete(p.jobs.jobs, id)
		}
	}
	return jobs, meta.LastIndex, nil
}
