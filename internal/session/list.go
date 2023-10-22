package session

import (
	"github.com/acouvreur/sablier/pkg/promise"
)

func (m *SessionManager) List() []Instance {
	m.lock.Lock()
	defer m.lock.Unlock()

	instances := make([]Instance, len(m.promises))

	for name, p := range m.promises {
		var instance Instance
		switch p.State {
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
