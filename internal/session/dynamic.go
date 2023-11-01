package session

import (
	"context"
	"errors"
	"time"

	"github.com/acouvreur/sablier/pkg/promise"
)

type RequestDynamicOptions struct {
	DesiredReplicas uint32 `json:"desiredReplicas"`
	SessionDuration time.Duration
}

func (s *Manager) RequestDynamic(ctx context.Context, name string, opts RequestDynamicOptions) (Instance, error) {
	p, ok := s.startDynamic(ctx, name, opts)
	if !ok {
		promise.Then[Instance](p, ctx, func(data Instance) (any, error) {
			s.timeouts.Put(name, string(data.Status), opts.SessionDuration)
			return nil, nil
		})
		promise.Catch[Instance](p, ctx, func(err error) error {
			s.deleteSync(name)
			return nil
		})
	}

	switch p.Status {
	case promise.Pending:
		return Instance{
			Name:   name,
			Status: InstanceStarting,
		}, nil
	case promise.Fulfilled:
		instance, _ := p.Await(ctx)
		s.timeouts.Put(name, string(instance.Status), opts.SessionDuration)
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

func (s *Manager) RequestDynamicAll(ctx context.Context, names []string, opts RequestDynamicOptions) ([]Instance, error) {
	promisesByName := make(map[string]*promise.Promise[Instance], len(names))
	promises := make([]*promise.Promise[Instance], len(names))

	for idx, name := range names {
		p, ok := s.startDynamic(ctx, name, opts)
		promisesByName[name] = p
		promises[idx] = p
		if !ok {
			promise.Then[Instance](p, ctx, func(data Instance) (any, error) {
				s.timeouts.Put(name, string(data.Status), opts.SessionDuration)
				return nil, nil
			})
			promise.Catch[Instance](p, ctx, func(err error) error {
				s.deleteSync(name)
				return err
			})
		}
	}

	all := promise.AllSettled(ctx, promises...)

	_, err := all.Await(ctx)
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

func (s *Manager) startDynamic(ctx context.Context, name string, opts RequestDynamicOptions) (*promise.Promise[Instance], bool) {
	s.lock.Lock()
	defer s.lock.Unlock()

	p, ok := s.promises[name]
	if !ok {
		p = StartInstance(ctx, name, StartOptions{DesiredReplicas: opts.DesiredReplicas}, s.provider)
		s.promises[name] = p
	}

	return p, ok
}
