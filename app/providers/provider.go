package providers

import "github.com/acouvreur/sablier/app/instance"

type Provider interface {
	Start(name string) (instance.State, error)
	Stop(name string) (instance.State, error)
	GetState(name string) (instance.State, error)
}
