package sablier

import (
	"context"
	"log"
	"time"

	"github.com/sablierapp/sablier/pkg/promise"
	"github.com/sablierapp/sablier/pkg/provider"
)

type StartOptions struct {
	DesiredReplicas    uint32
	ExpiresAfter       time.Duration
	ConsiderReadyAfter time.Duration
	Timeout            time.Duration
}

// StartInstance allows you to start an instance of a workload.
func (s *Sablier) StartInstance(name string, opts StartOptions) *promise.Promise[InstanceInfo] {
	s.pmu.Lock()
	defer s.pmu.Unlock()
	log.Printf("request to start instance [%v] received", name)

	// If there is an ongoing request, return it
	// If the last request was rejected, recreate one
	pr, ok := s.promises[name]
	if ok && pr.Pending() {
		log.Printf("request to start instance [%v] is already in progress", name)
		return pr
	}

	if ok && pr.Fulfilled() {
		log.Printf("instance [%s] will expire after [%v]", name, opts.ExpiresAfter)
		err := s.expirations.Put(name, name, opts.ExpiresAfter)
		if err != nil {
			log.Printf("failed to refresh instance [%v]: %v", name, err)
		}
		return pr
	}

	// Otherwise, create a new request
	pr = s.startInstancePromise(name, opts)
	log.Printf("request to start instance [%v] created", name)
	s.promises[name] = pr

	return pr
}

func (s *Sablier) startInstancePromise(name string, opts StartOptions) *promise.Promise[InstanceInfo] {
	return promise.New(func(resolve func(InstanceInfo), reject func(error)) {
		err := s.startInstance(name, opts)
		if err != nil {
			reject(err)
			return
		}

		started := InstanceInfo{
			Name:            name,
			CurrentReplicas: opts.DesiredReplicas, // Current replicas are assumed
			DesiredReplicas: opts.DesiredReplicas,
			Status:          InstanceReady,
		}
		resolve(started)
	})
}

func (s *Sablier) startInstance(name string, opts StartOptions) error {
	ctx, cancel := context.WithTimeout(context.Background(), opts.Timeout)
	defer cancel()

	log.Printf("starting instance [%s]", name)
	err := s.Provider.Start(ctx, name, provider.StartOptions{
		DesiredReplicas:    opts.DesiredReplicas,
		ConsiderReadyAfter: opts.ConsiderReadyAfter,
	})
	if err != nil {
		log.Printf("instance [%s] could not be started: %v", name, err)
		return err
	}

	log.Printf("instance [%s] will expire after [%v]", name, opts.ExpiresAfter)
	return s.expirations.Put(name, name, opts.ExpiresAfter)
}
