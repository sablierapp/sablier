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
	promisesByName := make(map[string]*promise.Promise[Instance], len(names))
	promises := make([]*promise.Promise[Instance], len(names))

	for idx, name := range names {
		p, _ := s.startBlocking(name, opts)
		promisesByName[name] = p
		promises[idx] = p
	}

	p := promise.AllSettled[Instance](ctx, promises...)

	_, err := p.Await(ctx)
	if err != nil {
		return nil, err
	}

	instances := make([]Instance, len(names))
	idx := 0
	for name, p := range promisesByName {
		instance, err := p.Await(ctx)
		if err != nil {
			instances[idx] = Instance{
				Name:   name,
				Status: InstanceError,
				Error:  err,
			}
		} else {
			instances[idx] = *instance
		}
		idx++
	}

	return instances, err
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
