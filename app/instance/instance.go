package instance

import log "github.com/sirupsen/logrus"

var Ready = "ready"
var NotReady = "not-ready"
var Error = "error"

type State struct {
	Name            string
	CurrentReplicas int
	Status          string
	Error           string
}

func (instance State) IsReady() bool {
	return instance.Status == Ready
}

func (instance State) HasError() bool {
	return instance.Status == Error
}

func ErrorInstanceState(name string, err error) (State, error) {
	log.Error(err.Error())
	return State{
		Name:            name,
		CurrentReplicas: 0,
		Status:          Error,
		Error:           err.Error(),
	}, err
}

func UnrecoverableInstanceState(name string, err string) (State, error) {
	log.Warn(err)
	return State{
		Name:            name,
		CurrentReplicas: 0,
		Status:          Error,
		Error:           err,
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
