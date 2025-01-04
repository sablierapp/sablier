package sablier

import (
	"context"
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
	s.log.Trace().Str("instance", name).Msg("request to start instance received")

	// If there is an ongoing request, return it
	// If the last request was rejected, recreate one
	pr, ok := s.promises[name]
	if ok && pr.Pending() {
		s.log.Trace().Str("instance", name).Msg("request to start instance is already in progress")
		return pr
	}

	if ok && pr.Fulfilled() {
		s.log.Trace().Str("instance", name).Dur("expiration", opts.ExpiresAfter).Msgf("instance will expire after [%v]", opts.ExpiresAfter)
		err := s.expirations.Put(name, name, opts.ExpiresAfter)
		if err != nil {
			s.log.Warn().Err(err).Str("instance", name).Msg("failed to refresh instance")
		}
		return pr
	}

	// Otherwise, create a new request
	pr = s.startInstancePromise(name, opts)
	s.log.Trace().Str("instance", name).Msg("request to start instance created")
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
			StartedAt:       time.Now(),
			ExpiresAt:       time.Now().Add(opts.ExpiresAfter),
		}
		resolve(started)
	})
}

func (s *Sablier) startInstance(name string, opts StartOptions) error {
	ctx, cancel := context.WithTimeout(context.Background(), opts.Timeout)
	defer cancel()

	s.log.Trace().Str("instance", name).Msg("starting instance")
	err := s.Provider.Start(ctx, name, provider.StartOptions{
		DesiredReplicas:    opts.DesiredReplicas,
		ConsiderReadyAfter: opts.ConsiderReadyAfter,
	})
	if err != nil {
		s.log.Trace().Str("instance", name).Err(err).Msg("instance could not be started")
		return err
	}

	s.log.Trace().Str("instance", name).Dur("expiration", opts.ExpiresAfter).Msgf("instance will expire after [%v]", opts.ExpiresAfter)
	return s.expirations.Put(name, name, opts.ExpiresAfter)
}
