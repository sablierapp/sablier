package instance

import log "github.com/sirupsen/logrus"

var Ready = "ready"
var NotReady = "not-ready"
var Unrecoverable = "unrecoverable"

type State struct {
	Name            string
	CurrentReplicas int
	Status          string
	Message         string
}

func (instance State) IsReady() bool {
	return instance.Status == Ready
}

func ErrorInstanceState(name string, err error) (State, error) {
	log.Error(err.Error())
	return State{
		Name:            name,
		CurrentReplicas: 0,
		Status:          Unrecoverable,
		Message:         err.Error(),
	}, err
}

func UnrecoverableInstanceState(name string, message string) (State, error) {
	log.Warn(message)
	return State{
		Name:            name,
		CurrentReplicas: 0,
		Status:          Unrecoverable,
		Message:         message,
	}, nil
}

func ReadyInstanceState(name string) (State, error) {
	return State{
		Name:            name,
		CurrentReplicas: 1,
		Status:          Ready,
	}, nil
}

func ReadyInstanceStateOfReplicas(name string, replicas int) (State, error) {
	return State{
		Name:            name,
		CurrentReplicas: replicas,
		Status:          Ready,
	}, nil
}

func NotReadyInstanceState(name string) (State, error) {
	return State{
		Name:            name,
		CurrentReplicas: 0,
		Status:          NotReady,
	}, nil
}

func NotReadyInstanceStateOfReplicas(name string, replicas int) (State, error) {
	return State{
		Name:            name,
		CurrentReplicas: replicas,
		Status:          NotReady,
	}, nil
}
