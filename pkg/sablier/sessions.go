package sablier

import (
	"context"
	"errors"
	"github.com/sablierapp/sablier/pkg/promise"
	"golang.org/x/exp/maps"
)

type StartSessionOptions struct {
	Wait bool
	StartOptions
}

type SessionStatus string

const (
	SessionStatusReady    SessionStatus = "ready"
	SessionStatusNotReady SessionStatus = "not-ready"
)

type InstanceInfoWithError struct {
	Error error `json:"error,omitempty"`

	InstanceInfo
}

type SessionInfo struct {
	Instances []InstanceInfoWithError `json:"instances"`
	Status    SessionStatus           `json:"status"`
}

func (s *Sablier) NewSessionInfo(ctx context.Context, promises map[string]*promise.Promise[InstanceInfo]) SessionInfo {
	instances := make([]InstanceInfoWithError, 0, len(promises))
	status := SessionStatusReady

	for name, p := range promises {
		if p.Fulfilled() {
			instance, _ := p.Await(ctx)
			instances = append(instances, InstanceInfoWithError{
				Error:        nil,
				InstanceInfo: *instance,
			})
		}
		if p.Rejected() {
			status = SessionStatusNotReady
			_, err := p.Await(ctx)
			instances = append(instances, InstanceInfoWithError{
				Error: err,
				InstanceInfo: InstanceInfo{
					Name:   name,
					Status: InstanceError,
				},
			})
		}
		if p.Pending() {
			status = SessionStatusNotReady
			instance, err := s.Provider.Info(ctx, name)
			// Go func for concurrent
			if err != nil {
				instances = append(instances, InstanceInfoWithError{
					Error: err,
					InstanceInfo: InstanceInfo{
						Name:   name,
						Status: InstanceError,
					},
				})
			} else {
				instances = append(instances, InstanceInfoWithError{
					Error:        nil,
					InstanceInfo: instance,
				})
			}
		}
	}

	return SessionInfo{
		Instances: instances,
		Status:    status,
	}
}

func (s *Sablier) StartSessionByNames(ctx context.Context, names []string, opts StartSessionOptions) (SessionInfo, error) {
	if len(names) == 0 {
		return SessionInfo{}, errors.New("at least one name is required")
	}

	promises := make(map[string]*promise.Promise[InstanceInfo], len(names))
	for _, name := range names {
		promises[name] = s.StartInstance(name, opts.StartOptions)
	}

	if opts.Wait {
		_, err := promise.AllSettled[InstanceInfo](ctx, maps.Values(promises)...).Await(ctx)
		if err != nil {
			return SessionInfo{}, err
		}
	}

	return s.NewSessionInfo(ctx, promises), nil
}

func (s *Sablier) StartSession(ctx context.Context, instances []InstanceConfig, opts StartSessionOptions) (SessionInfo, error) {
	if len(instances) == 0 {
		return SessionInfo{}, errors.New("at least one name is required")
	}

	promises := make(map[string]*promise.Promise[InstanceInfo], len(instances))
	for _, conf := range instances {
		promises[conf.Name] = s.StartInstance(conf.Name, StartOptions{
			DesiredReplicas:    conf.DesiredReplicas,
			ExpiresAfter:       opts.ExpiresAfter,
			ConsiderReadyAfter: opts.ConsiderReadyAfter,
			Timeout:            opts.Timeout,
		})
	}

	if opts.Wait {
		_, err := promise.AllSettled[InstanceInfo](ctx, maps.Values(promises)...).Await(ctx)
		if err != nil {
			return SessionInfo{}, err
		}
	}

	return s.NewSessionInfo(ctx, promises), nil
}
