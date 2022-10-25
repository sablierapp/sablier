package providers

import log "github.com/sirupsen/logrus"

type InstanceState struct {
	Name            string
	CurrentReplicas int
	Status          string
	Error           string
}

var Ready = "ready"
var NotReady = "not-ready"
var Error = "error"

type Provider interface {
	Start(name string) (InstanceState, error)
	Stop(name string) (InstanceState, error)
	GetState(name string) (InstanceState, error)
}

func (instance InstanceState) IsReady() bool {
	return instance.Status == Ready
}

func (instance InstanceState) HasError() bool {
	return instance.Status == Error
}

func errorInstanceState(name string, err error) (InstanceState, error) {
	log.Error(err.Error())
	return InstanceState{
		Name:            name,
		CurrentReplicas: 0,
		Status:          Error,
		Error:           err.Error(),
	}, err
}

func unrecoverableInstanceState(name string, err string) (InstanceState, error) {
	log.Warn(err)
	return InstanceState{
		Name:            name,
		CurrentReplicas: 0,
		Status:          Error,
		Error:           err,
	}, nil
}

func readyInstanceState(name string) (InstanceState, error) {
	return InstanceState{
		Name:            name,
		CurrentReplicas: 1,
		Status:          Ready,
	}, nil
}

func readyInstanceStateOfReplicas(name string, replicas int) (InstanceState, error) {
	return InstanceState{
		Name:            name,
		CurrentReplicas: replicas,
		Status:          Ready,
	}, nil
}

func notReadyInstanceState(name string) (InstanceState, error) {
	return InstanceState{
		Name:            name,
		CurrentReplicas: 0,
		Status:          NotReady,
	}, nil
}

func notReadyInstanceStateOfReplicas(name string, replicas int) (InstanceState, error) {
	return InstanceState{
		Name:            name,
		CurrentReplicas: replicas,
		Status:          NotReady,
	}, nil
}
