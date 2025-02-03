package instance

var Ready = "ready"
var NotReady = "not-ready"
var Unrecoverable = "unrecoverable"

type State struct {
	Name            string `json:"name"`
	CurrentReplicas int32  `json:"currentReplicas"`
	DesiredReplicas int32  `json:"desiredReplicas"`
	Status          string `json:"status"`
	Message         string `json:"message,omitempty"`
}

func (instance State) IsReady() bool {
	return instance.Status == Ready
}

func ErrorInstanceState(name string, err error, desiredReplicas int32) (State, error) {
	return State{
		Name:            name,
		CurrentReplicas: 0,
		DesiredReplicas: desiredReplicas,
		Status:          Unrecoverable,
		Message:         err.Error(),
	}, err
}

func UnrecoverableInstanceState(name string, message string, desiredReplicas int32) State {
	return State{
		Name:            name,
		CurrentReplicas: 0,
		DesiredReplicas: desiredReplicas,
		Status:          Unrecoverable,
		Message:         message,
	}
}

func ReadyInstanceState(name string, replicas int32) State {
	return State{
		Name:            name,
		CurrentReplicas: replicas,
		DesiredReplicas: replicas,
		Status:          Ready,
	}
}

func NotReadyInstanceState(name string, currentReplicas int32, desiredReplicas int32) State {
	return State{
		Name:            name,
		CurrentReplicas: currentReplicas,
		DesiredReplicas: desiredReplicas,
		Status:          NotReady,
	}
}
