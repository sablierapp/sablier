package routes

import (
	"bufio"
	"bytes"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/gin-gonic/gin"
	"github.com/sablierapp/sablier/app/http/routes/models"
	"github.com/sablierapp/sablier/app/instance"
	"github.com/sablierapp/sablier/app/sessions"
	"github.com/sablierapp/sablier/app/theme"
	"github.com/sablierapp/sablier/config"
)

var osDirFS = os.DirFS

type ServeStrategy struct {
	Theme *theme.Themes

	SessionsManager sessions.Manager
	StrategyConfig  config.Strategy
	SessionsConfig  config.Sessions
}

func NewServeStrategy(sessionsManager sessions.Manager, strategyConf config.Strategy, sessionsConf config.Sessions, themes *theme.Themes) *ServeStrategy {

	serveStrategy := &ServeStrategy{
		Theme:           themes,
		SessionsManager: sessionsManager,
		StrategyConfig:  strategyConf,
		SessionsConfig:  sessionsConf,
	}

	return serveStrategy
}

func (s *ServeStrategy) ServeDynamic(c *gin.Context) {
	request := models.DynamicRequest{
		Theme:            s.StrategyConfig.Dynamic.DefaultTheme,
		ShowDetails:      s.StrategyConfig.Dynamic.ShowDetailsByDefault,
		RefreshFrequency: s.StrategyConfig.Dynamic.DefaultRefreshFrequency,
		SessionDuration:  s.SessionsConfig.DefaultDuration,
	}

	if err := c.ShouldBind(&request); err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	var sessionState *sessions.SessionState
	if len(request.Names) > 0 {
		sessionState = s.SessionsManager.RequestSession(request.Names, request.SessionDuration)
	} else {
		sessionState = s.SessionsManager.RequestSessionGroup(request.Group, request.SessionDuration)
	}

	if sessionState == nil {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}

	if sessionState.IsReady() {
		c.Header("X-Sablier-Session-Status", "ready")
	} else {
		c.Header("X-Sablier-Session-Status", "not-ready")
	}

	renderOptions := theme.Options{
		DisplayName:      request.DisplayName,
		ShowDetails:      request.ShowDetails,
		SessionDuration:  request.SessionDuration,
		RefreshFrequency: request.RefreshFrequency,
		InstanceStates:   sessionStateToRenderOptionsInstanceState(sessionState),
	}

	buf := new(bytes.Buffer)
	writer := bufio.NewWriter(buf)
	if err := s.Theme.Render(request.Theme, renderOptions, writer); err != nil {
		log.Error(err)
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}
	writer.Flush()

	c.Header("Cache-Control", "no-cache")
	c.Header("Content-Type", "text/html")
	c.Header("Content-Length", strconv.Itoa(buf.Len()))
	c.Writer.Write(buf.Bytes())
}

func (s *ServeStrategy) ServeBlocking(c *gin.Context) {
	request := models.BlockingRequest{
		Timeout: s.StrategyConfig.Blocking.DefaultTimeout,
	}

	if err := c.ShouldBind(&request); err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	var sessionState *sessions.SessionState
	var err error
	if len(request.Names) > 0 {
		sessionState, err = s.SessionsManager.RequestReadySession(c.Request.Context(), request.Names, request.SessionDuration, request.Timeout)
	} else {
		sessionState, err = s.SessionsManager.RequestReadySessionGroup(c.Request.Context(), request.Group, request.SessionDuration, request.Timeout)
	}

	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	if sessionState == nil {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}

	if err != nil {
		c.Header("X-Sablier-Session-Status", "not-ready")
		c.JSON(http.StatusGatewayTimeout, map[string]interface{}{"error": err.Error()})
		return
	}

	if sessionState.IsReady() {
		c.Header("X-Sablier-Session-Status", "ready")
	} else {
		c.Header("X-Sablier-Session-Status", "not-ready")
	}

	c.JSON(http.StatusOK, map[string]interface{}{"session": sessionState})
}

func sessionStateToRenderOptionsInstanceState(sessionState *sessions.SessionState) (instances []theme.Instance) {
	if sessionState == nil {
		log.Warnf("sessionStateToRenderOptionsInstanceState: sessionState is nil")
		return
	}
	sessionState.Instances.Range(func(key, value any) bool {
		if value != nil {
			instances = append(instances, instanceStateToRenderOptionsRequestState(value.(sessions.InstanceState).Instance))
		} else {
			log.Warnf("sessionStateToRenderOptionsInstanceState: sessionState instance is nil, key: %v", key)
		}

		return true
	})

	sort.SliceStable(instances, func(i, j int) bool {
		return strings.Compare(instances[i].Name, instances[j].Name) == -1
	})

	return
}

func instanceStateToRenderOptionsRequestState(instanceState *instance.State) theme.Instance {

	var err error
	if instanceState.Message == "" {
		err = nil
	} else {
		err = fmt.Errorf(instanceState.Message)
	}

	return theme.Instance{
		Name:            instanceState.Name,
		Status:          instanceState.Status,
		CurrentReplicas: instanceState.CurrentReplicas,
		DesiredReplicas: instanceState.DesiredReplicas,
		Error:           err,
	}
}
