package routes

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/acouvreur/sablier/app/http/pages"
	"github.com/acouvreur/sablier/app/http/routes/models"
	"github.com/acouvreur/sablier/app/instance"
	"github.com/acouvreur/sablier/app/sessions"
	"github.com/acouvreur/sablier/version"
	"github.com/gin-gonic/gin"
)

type ServeStrategy struct {
	SessionsManager sessions.Manager
}

// ServeDynamic returns a waiting page displaying the session request if the session is not ready
// If the session is ready, returns a redirect 307 with an arbitrary location
func (s *ServeStrategy) ServeDynamic(c *gin.Context) {
	request := models.DynamicRequest{}

	if err := c.ShouldBind(&request); err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	sessionState := s.SessionsManager.RequestSession(request.Names, request.SessionDuration)

	if sessionState.IsReady() {
		c.Header("X-Sablier-Session-Status", "ready")
	} else {
		c.Header("X-Sablier-Session-Status", "not-ready")
	}

	renderOptions := pages.RenderOptions{
		DisplayName:      request.DisplayName,
		SessionDuration:  request.SessionDuration,
		Theme:            request.Theme,
		Version:          version.Version,
		RefreshFrequency: 5 * time.Second,
		InstanceStates:   sessionStateToRenderOptionsInstanceState(sessionState),
	}

	c.Header("Content-Type", "text/html")
	if err := pages.Render(renderOptions, c.Writer); err != nil {
		log.Error(err)
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}
}

func (s *ServeStrategy) ServeBlocking(c *gin.Context) {
	request := models.BlockingRequest{}

	if err := c.BindUri(&request); err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	sessionState := s.SessionsManager.RequestReadySession(request.Names, request.SessionDuration, request.Timeout)

	if sessionState.IsReady() {
		c.Header("X-Sablier-Session-Status", "ready")
	} else {
		c.Header("X-Sablier-Session-Status", "not-ready")
	}

}

func sessionStateToRenderOptionsInstanceState(sessionState *sessions.SessionState) (instances []pages.RenderOptionsInstanceState) {
	sessionState.Instances.Range(func(key, value any) bool {
		instances = append(instances, instanceStateToRenderOptionsRequestState(value.(sessions.InstanceState).Instance))
		return true
	})

	sort.SliceStable(instances, func(i, j int) bool {
		return strings.Compare(instances[i].Name, instances[j].Name) == -1
	})

	return
}

func instanceStateToRenderOptionsRequestState(instanceState *instance.State) pages.RenderOptionsInstanceState {

	var err error
	if instanceState.Message == "" {
		err = nil
	} else {
		err = fmt.Errorf(instanceState.Message)
	}

	return pages.RenderOptionsInstanceState{
		Name:            instanceState.Name,
		Status:          instanceState.Status,
		CurrentReplicas: instanceState.CurrentReplicas,
		DesiredReplicas: 1, //instanceState.DesiredReplicas,
		Error:           err,
	}
}
