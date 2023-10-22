package session

// Ready returns true if all instance are Running
func Ready(instances []Instance) bool {
	for _, instance := range instances {
		if instance.Status != InstanceRunning {
			return false
		}
	}
	return true
}
