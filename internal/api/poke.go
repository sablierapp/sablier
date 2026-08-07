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

// Poke registers GET /api/poke. It extends the TTL of an existing session
// without applying any start strategy. Instances that are not currently
// tracked by Sablier are reported in the response body but do not cause a
// non-2xx status code.
func Poke(router *gin.RouterGroup, s *ServeStrategy) {
	router.GET("/poke", func(c *gin.Context) {
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
			sessionState, err = s.Sablier.ExtendSession(c.Request.Context(), request.Names, request.SessionDuration)
		} else {
			sessionState, err = s.Sablier.ExtendSessionGroup(c.Request.Context(), request.Group, request.SessionDuration)
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

		AddSablierHeader(c, sessionState)
		c.JSON(http.StatusOK, map[string]interface{}{"session": sessionState})
	})
}
