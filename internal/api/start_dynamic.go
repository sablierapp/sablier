package api

import (
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

// StartDynamic registers the dynamic strategy endpoint.
//
// @Summary      Dynamic strategy
// @Description  Returns a themed HTML waiting page reflecting the session status; the page self-refreshes until the instances are ready. Provide either `names` or `group`.
// @Tags         strategies
// @Produce      html
// @Param        names              query  []string  false  "Instance name(s). Mutually exclusive with group."
// @Param        group              query  string    false  "Group name. Mutually exclusive with names."
// @Param        session_duration   query  string    false  "Session duration as a Go duration (e.g. 5m)."
// @Param        refresh_frequency  query  string    false  "Waiting-page refresh interval as a Go duration (e.g. 5s)."
// @Param        show_details       query  bool      false  "Show per-instance details on the waiting page."
// @Param        display_name       query  string    false  "Display name shown on the waiting page."
// @Param        theme              query  string    false  "Theme name for the waiting page."
// @Success      200  {string}  string  "HTML waiting page"
// @Header       200  {string}  X-Sablier-Session-Status  "ready or not-ready"
// @Failure      404  {object}  rfc7807.Problem  "Group or theme not found"
// @Failure      500  {object}  rfc7807.Problem  "Internal error"
// @Router       /api/strategies/dynamic [get]
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

		// Validate the theme before any instance is started: a mistyped theme
		// in the plugin configuration must not start workloads on behalf of a
		// request that can only ever return 404 (and would start them again on
		// every retry).
		if !s.Theme.Exists(request.Theme) {
			AbortWithProblemDetail(c, ProblemThemeNotFound(theme.ErrThemeNotFound{
				Theme:           request.Theme,
				AvailableThemes: s.Theme.List(),
			}))
			return
		}

		recordSessionRequest(s.Metrics, "dynamic", request.Group)

		var sessionState *sablier.SessionState
		var err error
		if len(request.Names) > 0 {
			sessionState, err = s.Sablier.RequestSession(c, request.Names, request.SessionDuration)
		} else {
			sessionState, err = s.Sablier.RequestSessionGroup(c, request.Group, request.SessionDuration)
			if groupNotFoundError, ok := errors.AsType[sablier.ErrGroupNotFound](err); ok {
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

		// Render into a plain buffer: rendering must fully succeed before any
		// byte reaches the client, so a template that fails halfway (easy to
		// author in a custom theme) surfaces as a 500 problem instead of a 200
		// with a truncated page.
		buf := new(bytes.Buffer)
		err = s.Theme.Render(request.Theme, renderOptions, buf)
		if themeNotFound, ok := errors.AsType[theme.ErrThemeNotFound](err); ok {
			AbortWithProblemDetail(c, ProblemThemeNotFound(themeNotFound))
			return
		}
		if err != nil {
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

	for name, v := range sessionState.Instances {
		if v.Error != nil {
			instances = append(instances, theme.Instance{
				Name:   name,
				Status: string(sablier.InstanceStatusError),
				Error:  v.Error,
			})
			continue
		}
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
		Provider:        instanceState.Provider,
		Docker:          instanceState.Docker,
		Swarm:           instanceState.Swarm,
		Kubernetes:      instanceState.Kubernetes,
		Podman:          instanceState.Podman,
	}
}
