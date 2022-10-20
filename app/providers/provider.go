package providers

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
