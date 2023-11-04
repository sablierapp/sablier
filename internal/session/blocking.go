package session

import (
	"context"
	"time"

	"github.com/acouvreur/sablier/pkg/promise"
)

type RequestBlockingOptions struct {
	DesiredReplicas uint32 `json:"desiredReplicas"`
	SessionDuration time.Duration
}

func (s *Manager) RequestBlocking(ctx context.Context, names []string, opts RequestBlockingOptions) ([]Instance, error) {
	promises := make([]*promise.Promise[Instance], len(names))

	for idx, name := range names {
		p, _ := s.startBlocking(name, opts)
		promises[idx] = p
	}

	p := promise.All[Instance](ctx, promises...)

	instances, err := p.Await(ctx)
	if err != nil {
		return nil, err
	}

	return *instances, err
}

func (s *Manager) startBlocking(name string, opts RequestBlockingOptions) (*promise.Promise[Instance], bool) {
	s.lock.Lock()
	defer s.lock.Unlock()

	p, ok := s.promises[name]
	if !ok {
		p = StartInstance(name, StartOptions{DesiredReplicas: opts.DesiredReplicas}, s.provider)
		s.promises[name] = p
		promise.Catch(p, context.Background(), func(err error) error {
			s.deleteSync(name)
			return nil
		})
	}
	promise.Then(p, context.Background(), func(i Instance) (Instance, error) {
		s.timeouts.Put(name, NewSession(opts.SessionDuration), opts.SessionDuration)
		return i, nil
	})

	return p, ok
}
