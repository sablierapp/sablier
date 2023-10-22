package session

import (
	"context"
	"errors"
	"time"

	"github.com/acouvreur/sablier/pkg/promise"
)

type RequestOptions struct {
	DesiredReplicas uint32 `json:"desiredReplicas"`
	SessionDuration time.Duration
}

func (s *SessionManager) Request(ctx context.Context, name string, opts RequestOptions) (Instance, error) {
	p, ok := s.promise(ctx, name, opts)
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

	switch p.State {
	case promise.Pending:
		return Instance{
			Name:   name,
			Status: InstanceStarting,
		}, nil
	case promise.Fulfilled:
		instance, _ := p.Await(ctx)
		s.instances.Put(name, string(instance.Status), opts.SessionDuration)
		return Instance{
			Name:   name,
			Status: InstanceRunning,
		}, nil
	case promise.Rejected:
		_, err := p.Await(ctx)
		return Instance{}, err
	default:
		return Instance{}, errors.New("unknown state")
	}
}

func (s *SessionManager) promise(ctx context.Context, name string, opts RequestOptions) (*promise.Promise[Instance], bool) {
	s.lock.Lock()
	defer s.lock.Unlock()

	p, ok := s.promises[name]
	if !ok {
		p = StartInstance(ctx, name, StartOptions{DesiredReplicas: opts.DesiredReplicas}, s.provider)
		s.promises[name] = p
	}

	return p, ok
}
