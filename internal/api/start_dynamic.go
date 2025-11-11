package api

import (
	"bufio"
	"bytes"
	"errors"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/sablierapp/sablier/pkg/sablier"

	"github.com/gin-gonic/gin"
	"github.com/sablierapp/sablier/pkg/theme"
)

type DynamicRequest struct {
	Group            string        `form:"group"`
	Names            []string      `form:"names"`
	ShowDetails      bool          `form:"show_details"`
	DisplayName      string        `form:"display_name"`
	Theme            string        `form:"theme"`
	SessionDuration  time.Duration `form:"session_duration"`
	RefreshFrequency time.Duration `form:"refresh_frequency"`
}

func StartDynamic(router *gin.RouterGroup, s *ServeStrategy) {
	router.GET("/strategies/dynamic", func(c *gin.Context) {
		request := DynamicRequest{
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

		var sessionState *sablier.SessionState
		var err error
		if len(request.Names) > 0 {
			sessionState, err = s.Sablier.RequestSession(c, request.Names, request.SessionDuration)
		} else {
			sessionState, err = s.Sablier.RequestSessionGroup(c, request.Group, request.SessionDuration)
			var groupNotFoundError sablier.ErrGroupNotFound
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
		if err := writer.Flush(); err != nil {
			AbortWithProblemDetail(c, ProblemError(err))
			return
		}

		c.Header("Cache-Control", "no-cache")
		c.Header("Content-Type", "text/html")
		c.Header("Content-Length", strconv.Itoa(buf.Len()))
		if _, err := c.Writer.Write(buf.Bytes()); err != nil {
			AbortWithProblemDetail(c, ProblemError(err))
			return
		}
	})
}

func sessionStateToRenderOptionsInstanceState(sessionState *sablier.SessionState) (instances []theme.Instance) {
	if sessionState == nil {
		return
	}

	for _, v := range sessionState.Instances {
		instances = append(instances, instanceStateToRenderOptionsRequestState(v.Instance))
	}

	sort.SliceStable(instances, func(i, j int) bool {
		return strings.Compare(instances[i].Name, instances[j].Name) == -1
	})

	return
}

func instanceStateToRenderOptionsRequestState(instanceState sablier.InstanceInfo) theme.Instance {

	var err error
	if instanceState.Message == "" {
		err = nil
	} else {
		err = errors.New(instanceState.Message)
	}

	return theme.Instance{
		Name:            instanceState.Name,
		Status:          string(instanceState.Status),
		CurrentReplicas: instanceState.CurrentReplicas,
		DesiredReplicas: instanceState.DesiredReplicas,
		Error:           err,
	}
}
