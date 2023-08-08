package routes

import (
	"net/http"

	"github.com/acouvreur/sablier/app/instance"
	"github.com/acouvreur/sablier/app/sessions"
	"github.com/acouvreur/sablier/config"
	"github.com/gin-gonic/gin"
)

type Status struct {
	SessionsManager sessions.Manager
	StrategyConfig  config.Strategy
	SessionsConfig  config.Sessions
}

func NewStatus(sessionsManager sessions.Manager, strategyConf config.Strategy, sessionsConf config.Sessions) *Status {

	status := &Status{
		SessionsManager: sessionsManager,
		StrategyConfig:  strategyConf,
		SessionsConfig:  sessionsConf,
	}

	return status
}

type StatusResponse struct {
	ManagedServices []instance.State `json:"managed_services"`
}

func (s *Status) ServeStatus(c *gin.Context) {
	containers, _ := s.SessionsManager.GetManagedContainers()

	var managedServices []instance.State
	for _, name := range containers {

		state := s.SessionsManager.GetContainerStatus(name)

		if state.Error != nil {
			c.AbortWithStatus(500)
		}

		managedServices = append(managedServices, *state.Instance)
	}

	output := StatusResponse{
		ManagedServices: managedServices,
	}
	c.JSON(http.StatusOK, output)
}
