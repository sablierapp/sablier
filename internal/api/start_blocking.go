package api

import (
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/sablierapp/sablier/pkg/sablier"
	"net/http"
	"time"
)

type BlockingRequest struct {
	// Names are the instances names on your provider.
	// - Container name for docker
	// - Service name for docker swarm
	// - Deployment or StatefulSet name for Kubernetes
	// - etc.
	//
	// Deprecated: Please use Group instead.
	Names []string `form:"names"`

	// Group is
	Group              string        `form:"group"`
	SessionDuration    time.Duration `form:"session_duration"`
	Timeout            time.Duration `form:"timeout"`
	ConsiderReadyAfter time.Duration `form:"consider_ready_after"`
	DesiredReplicas    uint32        `form:"desired_replicas"`
}

func StartBlocking(router *gin.RouterGroup, s *sablier.Sablier) {
	handler := func(c *gin.Context) {
		request := BlockingRequest{
			SessionDuration:    10 * time.Second,
			Timeout:            30 * time.Second,
			ConsiderReadyAfter: 0,
		}

		// Validation
		if err := c.ShouldBind(&request); err != nil {
			AbortWithProblemDetail(c, ValidationError(fmt.Errorf("could not bind request: %w", err)))
			return
		}

		if len(request.Names) == 0 && request.Group == "" {
			AbortWithProblemDetail(c, ValidationError(errors.New("'names' or 'group' query parameter must be set")))
			return
		}

		if len(request.Names) > 0 && request.Group != "" {
			AbortWithProblemDetail(c, ValidationError(errors.New("'names' and 'group' query parameters are both set, only one must be set")))
			return
		}

		// Build instance config
		var instances []sablier.InstanceConfig
		if request.Group != "" {
			i, ok := s.GetGroup(request.Group)
			if !ok {
				AbortWithProblemDetail(c, ValidationError(fmt.Errorf("group name [%s] does not exist", request.Group)))
				return
			}
			instances = i
		} else {
			instances = make([]sablier.InstanceConfig, 0, len(request.Names))
			for i := 0; i < len(instances); i++ {
				instances[i] = sablier.InstanceConfig{
					Name:            request.Names[i],
					DesiredReplicas: request.DesiredReplicas,
				}
			}
		}

		opts := sablier.StartSessionOptions{
			Wait: true,
			StartOptions: sablier.StartOptions{
				DesiredReplicas:    1,
				ExpiresAfter:       request.SessionDuration,
				ConsiderReadyAfter: request.ConsiderReadyAfter,
				Timeout:            request.Timeout,
			},
		}

		// Call
		session, err := s.StartSession(c, instances, opts)
		if err != nil {
			AbortWithProblemDetail(c, ValidationError(err))
			return
		}

		AddSablierHeader(c, session)

		c.IndentedJSON(http.StatusOK, map[string]interface{}{"session": session})
	}
	router.GET("/blocking", handler)
	router.GET("/strategies/blocking", handler) // Legacy support
}
