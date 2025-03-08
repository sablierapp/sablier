package sablier

type InstanceStatus string

const (
	InstanceStatusReady         = "ready"
	InstanceStatusNotReady      = "not-ready"
	InstanceStatusUnrecoverable = "unrecoverable"
)

type InstanceInfo struct {
	Name            string         `json:"name"`
	CurrentReplicas int32          `json:"currentReplicas"`
	DesiredReplicas int32          `json:"desiredReplicas"`
	Status          InstanceStatus `json:"status"`
	Message         string         `json:"message,omitempty"`
}

type InstanceConfiguration struct {
	Name  string
	Group string
}

func (instance InstanceInfo) IsReady() bool {
	return instance.Status == InstanceStatusReady
}

func UnrecoverableInstanceState(name string, message string, desiredReplicas int32) InstanceInfo {
	return InstanceInfo{
		Name:            name,
		CurrentReplicas: 0,
		DesiredReplicas: desiredReplicas,
		Status:          InstanceStatusUnrecoverable,
		Message:         message,
	}
}

func ReadyInstanceState(name string, replicas int32) InstanceInfo {
	return InstanceInfo{
		Name:            name,
		CurrentReplicas: replicas,
		DesiredReplicas: replicas,
		Status:          InstanceStatusReady,
	}
}

func NotReadyInstanceState(name string, currentReplicas int32, desiredReplicas int32) InstanceInfo {
	return InstanceInfo{
		Name:            name,
		CurrentReplicas: currentReplicas,
		DesiredReplicas: desiredReplicas,
		Status:          InstanceStatusNotReady,
	}
}
