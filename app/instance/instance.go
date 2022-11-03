package instance

import log "github.com/sirupsen/logrus"

var Ready = "ready"
var NotReady = "not-ready"
var Unrecoverable = "unrecoverable"

type State struct {
	Name            string `json:"name"`
	CurrentReplicas int    `json:"currentReplicas"`
	DesiredReplicas int    `json:"desiredReplicas"`
	Status          string `json:"status"`
	Message         string `json:"message,omitempty"`
}

func (instance State) IsReady() bool {
	return instance.Status == Ready
}

func ErrorInstanceState(name string, err error, desiredReplicas int) (State, error) {
	log.Error(err.Error())
	return State{
		Name:            name,
		CurrentReplicas: 0,
		DesiredReplicas: desiredReplicas,
		Status:          Unrecoverable,
		Message:         err.Error(),
	}, err
}

func UnrecoverableInstanceState(name string, message string, desiredReplicas int) (State, error) {
	log.Warn(message)
	return State{
		Name:            name,
		CurrentReplicas: 0,
		DesiredReplicas: desiredReplicas,
		Status:          Unrecoverable,
		Message:         message,
	}, nil
}

func ReadyInstanceState(name string, replicas int) (State, error) {
	return State{
		Name:            name,
		CurrentReplicas: replicas,
		DesiredReplicas: replicas,
		Status:          Ready,
	}, nil
}

func NotReadyInstanceState(name string, currentReplicas int, desiredReplicas int) (State, error) {
	return State{
		Name:            name,
		CurrentReplicas: currentReplicas,
		DesiredReplicas: desiredReplicas,
		Status:          NotReady,
	}, nil
}
