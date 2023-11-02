package session

import (
	"context"
	"github.com/acouvreur/sablier/pkg/promise"
)

func (s *Manager) Instances() []Instance {
	s.lock.Lock()
	defer s.lock.Unlock()

	instances := make([]Instance, len(s.promises))

	for name, p := range s.promises {
		var instance Instance
		switch p.Status {
		case promise.Pending:
			instance = Instance{
				Name:   name,
				Status: InstanceStarting,
				Error:  nil,
			}
		case promise.Fulfilled:
			instance = Instance{
				Name:   name,
				Status: InstanceRunning,
				Error:  nil,
			}
		case promise.Rejected:
			_, err := p.Await(context.Background())
			instance = Instance{
				Name:   name,
				Status: InstanceError,
				Error:  err,
			}
		}
		instances = append(instances, instance)
	}

	return instances
}
