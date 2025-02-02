package api

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/sablierapp/sablier/app/http/routes"
	"github.com/sablierapp/sablier/app/http/routes/models"
	"github.com/sablierapp/sablier/app/instance"
	"github.com/sablierapp/sablier/app/sessions"
	"github.com/sablierapp/sablier/app/theme"
	log "github.com/sirupsen/logrus"
	"sort"
	"strconv"
	"strings"
)

func StartDynamic(router *gin.RouterGroup, s *routes.ServeStrategy) {
	router.GET("/strategies/dynamic", func(c *gin.Context) {
		request := models.DynamicRequest{
			Theme:            s.StrategyConfig.Dynamic.DefaultTheme,
			ShowDetails:      s.StrategyConfig.Dynamic.ShowDetailsByDefault,
			RefreshFrequency: s.StrategyConfig.Dynamic.DefaultRefreshFrequency,
			SessionDuration:  s.SessionsConfig.DefaultDuration,
		}

		if err := c.ShouldBind(&request); err != nil {
			AbortWithProblemDetail(c, ProblemValidation(err))
			return
		}

		if len(request.Names) == 0 && request.Group == "" {
			AbortWithProblemDetail(c, ProblemValidation(errors.New("'names' or 'group' query parameter must be set")))
			return
		}

		if len(request.Names) > 0 && request.Group != "" {
			AbortWithProblemDetail(c, ProblemValidation(errors.New("'names' and 'group' query parameters are both set, only one must be set")))
			return
		}

		var sessionState *sessions.SessionState
		var err error
		if len(request.Names) > 0 {
			sessionState, err = s.SessionsManager.RequestSession(request.Names, request.SessionDuration)
		} else {
			sessionState, err = s.SessionsManager.RequestSessionGroup(request.Group, request.SessionDuration)
			var groupNotFoundError sessions.ErrGroupNotFound
			if errors.As(err, &groupNotFoundError) {
				AbortWithProblemDetail(c, ProblemGroupNotFound(groupNotFoundError))
				return
			}
		}

		if err != nil {
			AbortWithProblemDetail(c, ProblemError(err))
			return
		}

		if sessionState == nil {
			AbortWithProblemDetail(c, ProblemError(errors.New("session could not be created, please check logs for more details")))
			return
		}

		AddSablierHeader(c, sessionState)

		renderOptions := theme.Options{
			DisplayName:      request.DisplayName,
			ShowDetails:      request.ShowDetails,
			SessionDuration:  request.SessionDuration,
			RefreshFrequency: request.RefreshFrequency,
			InstanceStates:   sessionStateToRenderOptionsInstanceState(sessionState),
		}

		buf := new(bytes.Buffer)
		writer := bufio.NewWriter(buf)
		err = s.Theme.Render(request.Theme, renderOptions, writer)
		var themeNotFound theme.ErrThemeNotFound
		if errors.As(err, &themeNotFound) {
			AbortWithProblemDetail(c, ProblemThemeNotFound(themeNotFound))
			return
		}
		writer.Flush()

		c.Header("Cache-Control", "no-cache")
		c.Header("Content-Type", "text/html")
		c.Header("Content-Length", strconv.Itoa(buf.Len()))
		c.Writer.Write(buf.Bytes())
	})
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
