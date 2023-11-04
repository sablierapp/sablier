package session

import (
	"context"
	"time"

	"github.com/acouvreur/sablier/pkg/promise"
)

type RequestDynamicOptions struct {
	DesiredReplicas uint32 `json:"desiredReplicas"`
	SessionDuration time.Duration
}

func (s *Manager) RequestDynamic(ctx context.Context, names []string, opts RequestDynamicOptions) ([]Instance, error) {
	promisesByName := make(map[string]*promise.Promise[Instance], len(names))
	promises := make([]*promise.Promise[Instance], len(names))

	for idx, name := range names {
		p, _ := s.startDynamic(name, opts)
		promisesByName[name] = p
		promises[idx] = p
	}

	instances := make([]Instance, len(names))
	idx := 0
	for name, p := range promisesByName {
		if p.Status == promise.Pending {
			instances[idx] = Instance{
				Name:   name,
				Status: InstanceStarting,
				Error:  nil,
			}
		} else {
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
		}
		idx++
	}

	return instances, nil
}

func (s *Manager) startDynamic(name string, opts RequestDynamicOptions) (*promise.Promise[Instance], bool) {
	s.lock.Lock()
	defer s.lock.Unlock()

	p, ok := s.promises[name]
	if !ok {
		p = StartInstance(name, StartOptions{DesiredReplicas: opts.DesiredReplicas}, s.provider)
		s.promises[name] = p
	} else if p.Status == promise.Rejected {
		// Here we are deleting the current rejected promise but still we're returning this one.
		// it's important so users can have a feedback on the last request. Next request will retry to start the
		// instance
		delete(s.promises, name)
	}
	promise.Then(p, context.Background(), func(i Instance) (Instance, error) {
		s.timeouts.Put(name, NewSession(opts.SessionDuration), opts.SessionDuration)
		return i, nil
	})

	return p, ok
}
