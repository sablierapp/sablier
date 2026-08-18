package api

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sablierapp/sablier/pkg/sablier"
)

type PokeRequest struct {
	Names           []string      `form:"names"`
	Group           string        `form:"group"`
	SessionDuration time.Duration `form:"session_duration"`
}

// StartPoke registers the poke strategy endpoint.
//
// @Summary      Poke strategy
// @Description  Starts the requested instances and immediately returns the session status without waiting for readiness. Provide either `names` or `group`, never both.
// @Tags         strategies
// @Produce      json
// @Param        names             query  []string  false  "Instance name(s); repeat for multiple. Mutually exclusive with group."
// @Param        group             query  string    false  "Group name. Mutually exclusive with names."
// @Param        session_duration  query  string    false  "Session duration as a Go duration (e.g. 5m). Defaults to the server default."
// @Success      200  {object}  SessionResponse
// @Header       200  {string}  X-Sablier-Session-Status  "ready or not-ready"
// @Failure      400  {object}  rfc7807.Problem  "Validation error"
// @Failure      404  {object}  rfc7807.Problem  "Group not found"
// @Failure      500  {object}  rfc7807.Problem  "Internal error"
// @Router       /api/strategies/poke [get]
func StartPoke(router *gin.RouterGroup, s *ServeStrategy) {
	router.GET("/strategies/poke", func(c *gin.Context) {
		request := PokeRequest{
			SessionDuration: s.SessionsConfig.DefaultDuration,
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

		recordSessionRequest(s.Metrics, "poke", request.Group)

		var sessionState *sablier.SessionState
		var err error
		if len(request.Names) > 0 {
			sessionState, err = s.Sablier.RequestSession(c.Request.Context(), request.Names, request.SessionDuration)
		} else {
			sessionState, err = s.Sablier.RequestSessionGroup(c.Request.Context(), request.Group, request.SessionDuration)
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

		c.JSON(http.StatusOK, map[string]any{"session": sessionState})
	})
}
