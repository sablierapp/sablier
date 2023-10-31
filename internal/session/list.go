package session

import (
	"github.com/acouvreur/sablier/pkg/promise"
)

func (s *SessionManager) List() []Instance {
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
			}
		case promise.Fulfilled:
			instance = Instance{
				Name:   name,
				Status: InstanceRunning,
			}
		case promise.Rejected:
			instance = Instance{
				Name:   name,
				Status: InstanceError,
			}
		}
		instances = append(instances, instance)
	}

	return instances
}
