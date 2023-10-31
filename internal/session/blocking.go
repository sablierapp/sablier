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

func (s *SessionManager) RequestBlocking(ctx context.Context, name string, opts RequestBlockingOptions) (Instance, error) {
	p, _ := s.startBlocking(ctx, name, opts)

	instance, err := p.Await(ctx)
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
		p, ok := s.startBlocking(ctx, name, opts)
		promises = append(promises, p)
		if !ok {
			promise.Then[Instance](p, ctx, func(data Instance) (any, error) {
				s.instances.Put(name, string(data.Status), opts.SessionDuration)
				return nil, nil
			})
			promise.Catch[Instance](p, ctx, func(err error) error {
				s.deleteSync(name)
				return err
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

func (s *SessionManager) startBlocking(ctx context.Context, name string, opts RequestBlockingOptions) (*promise.Promise[Instance], bool) {
	s.lock.Lock()
	defer s.lock.Unlock()

	p, ok := s.promises[name]
	if !ok {
		p = StartInstance(ctx, name, StartOptions{DesiredReplicas: opts.DesiredReplicas}, s.provider)
		s.promises[name] = p
	}

	return p, ok
}
