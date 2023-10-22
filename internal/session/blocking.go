package session

import (
	"context"
	"time"

	"github.com/acouvreur/sablier/pkg/promise"
)

type RequestBlockingOptions struct {
	DesiredReplicas uint32 `json:"desiredReplicas"`
	SessionDuration time.Duration
	Timeout         time.Duration
}

func (s *SessionManager) RequestBlocking(ctx context.Context, name string, opts RequestBlockingOptions) (Instance, error) {
	p, _ := s.promiseBlocking(ctx, name, opts)

	timeout, cancel := context.WithTimeout(ctx, opts.Timeout)
	defer cancel()

	instance, err := p.Await(timeout)

	if err != nil {
		s.deleteSync(name)
		return Instance{}, err
	}

	s.instances.Put(name, string(instance.Status), opts.SessionDuration)

	return *instance, err
}

func (s *SessionManager) RequestBlockingAll(ctx context.Context, names []string, opts RequestBlockingOptions) ([]Instance, error) {
	promises := make([]*promise.Promise[Instance], 0)

	for _, name := range names {
		p, ok := s.promiseBlocking(ctx, name, opts)
		promises = append(promises, p)
		if !ok {
			promise.Then[Instance](p, ctx, func(data Instance) (any, error) {
				err := s.instances.Put(name, string(data.Status), opts.SessionDuration)
				return nil, err
			})
			promise.Catch[Instance](p, ctx, func(err error) error {
				s.deleteSync(name)
				return nil
			})
		}
	}

	p := promise.All[Instance](ctx, promises...)

	instances, err := p.Await(ctx)
	if err != nil {
		return nil, err
	}

	return *instances, err
}

func (s *SessionManager) promiseBlocking(ctx context.Context, name string, opts RequestBlockingOptions) (*promise.Promise[Instance], bool) {
	s.lock.Lock()
	defer s.lock.Unlock()

	p, ok := s.promises[name]
	if !ok {
		p = StartInstance(ctx, name, StartOptions{DesiredReplicas: opts.DesiredReplicas}, s.provider)
		s.promises[name] = p
	}

	return p, ok
}
