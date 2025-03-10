package api

import (
	"errors"
	"github.com/gin-gonic/gin"
	"github.com/sablierapp/sablier/pkg/sablier"
	"net/http"
	"time"
)

type BlockingRequest struct {
	Names           []string      `form:"names"`
	Group           string        `form:"group"`
	SessionDuration time.Duration `form:"session_duration"`
	Timeout         time.Duration `form:"timeout"`
}

func StartBlocking(router *gin.RouterGroup, s *ServeStrategy) {
	router.GET("/strategies/blocking", func(c *gin.Context) {
		request := BlockingRequest{
			SessionDuration: s.SessionsConfig.DefaultDuration,
			Timeout:         s.StrategyConfig.Blocking.DefaultTimeout,
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
			sessionState, err = s.Sablier.RequestReadySession(c.Request.Context(), request.Names, request.SessionDuration, request.Timeout)
		} else {
			sessionState, err = s.Sablier.RequestReadySessionGroup(c.Request.Context(), request.Group, request.SessionDuration, request.Timeout)
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

		c.JSON(http.StatusOK, map[string]interface{}{"session": sessionState})
	})
}
