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
	Error error

	InstanceInfo
}

type SessionInfo struct {
	Instances []InstanceInfoWithError
	Status    SessionStatus
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
		return SessionInfo{}, errors.New("no names")
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

func (s *Sablier) StartSessionByGroup(ctx context.Context, name string, opts StartSessionOptions) (SessionInfo, error) {
	if len(name) == 0 {
		return SessionInfo{}, errors.New("group name is mandatory")
	}

	instances, ok := s.GetGroup(name)
	if !ok {
		return SessionInfo{}, errors.New("group not found")
	}

	promises := make(map[string]*promise.Promise[InstanceInfo], len(instances))
	for _, instance := range instances {
		// TODO: Merge start options with the one defined in the InstanceConfig
		promises[instance.Name] = s.StartInstance(instance.Name, opts.StartOptions)
	}

	if opts.Wait {
		_, err := promise.AllSettled[InstanceInfo](ctx, maps.Values(promises)...).Await(ctx)
		if err != nil {
			return SessionInfo{}, err
		}
	}

	return s.NewSessionInfo(ctx, promises), nil
}
