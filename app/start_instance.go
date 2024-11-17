package app

import (
	"context"
	"encoding/json"
	"errors"
	log "log/slog"
	"time"

	"github.com/sablierapp/sablier/pkg/promise"
)

type StartOptions struct {
	DesiredReplicas uint32
}

// StartInstance allows you to start an instance of a workload.
// An instance
func (s *Sablier) StartInstance(name string, opts StartOptions) *promise.Promise[Instance] {
	s.lock.Lock()
	defer s.lock.Unlock()
	log.Info("request to start instance [%v] received", name)

	// If there is an ongoing request, return it
	// If the last request was rejected, recreate one
	pr, ok := s.promises[name]
	if ok && !pr.Rejected() {
		log.Info("request to start instance %v is already in progress", name)
		return pr
	}

	// Otherwise, create a new request
	pr = s.StartInstancePromise(name, opts)
	log.Info("request to start instance %v created", name)
	s.promises[name] = pr

	return pr
}

func (s *Sablier) StartInstancePromise(name string, opts StartOptions) *promise.Promise[Instance] {
	return promise.New(func(resolve func(Instance), reject func(error)) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()

		log.Info("starting instance", "instance", name)
		ready, err := s.provider.Status(ctx, name)
		if err != nil {
			log.Error("error starting instance %v: %v", name, err)
			reject(err)
			return
		}

		if !ready {
			messages, err := s.pubsub.Subscribe(ctx, "sablier.instance")
			if err != nil {
				reject(err)
				return
			}

			if err := s.provider.Start(ctx, name, opts); err != nil {
				log.Info("error starting instance", "instance", name, "error", err)
				reject(err)
			}

			for {
				select {
				case msg, ok := <-messages:
					if !ok {
						reject(errors.New("publisher channel closed"))
						return
					}

					var event Message
					err := json.Unmarshal(msg.Payload, &event)
					if err != nil {
						return
					}
					if event.Name == name {
						cancel()
						if event.Action == EventActionReady {
							cancel()
						}
						if event.Action == Eve {
							// Something else ?
						}
					}
				case <-ctx.Done():
					reject(ctx.Err())
					return
				}
			}
		}

		started := Instance{
			Name:   name,
			Status: InstanceRunning,
		}
		log.Info("successfully started instance", "instance", name)
		resolve(started)
	})
}
