package api

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sablierapp/sablier/pkg/sablier"
)

type BlockingRequest struct {
	Names           []string      `form:"names"`
	Group           string        `form:"group"`
	SessionDuration time.Duration `form:"session_duration"`
	Timeout         time.Duration `form:"timeout"`
}

// StartBlocking registers the blocking strategy endpoint.
//
// @Summary      Blocking strategy
// @Description  Holds the request until the requested instances are ready, or until the timeout elapses. Provide either `names` (one or more) or `group`, never both.
// @Tags         strategies
// @Produce      json
// @Param        names             query  []string  false  "Instance name(s); repeat for multiple. Mutually exclusive with group."
// @Param        group             query  string    false  "Group name. Mutually exclusive with names."
// @Param        session_duration  query  string    false  "Session duration as a Go duration (e.g. 5m). Defaults to the server default."
// @Param        timeout           query  string    false  "Maximum time to wait as a Go duration (e.g. 1m)."
// @Success      200  {object}  SessionResponse
// @Header       200  {string}  X-Sablier-Session-Status  "ready or not-ready"
// @Failure      400  {object}  rfc7807.Problem  "Validation error"
// @Failure      404  {object}  rfc7807.Problem  "Group or instance not found"
// @Failure      500  {object}  rfc7807.Problem  "Internal error"
// @Failure      504  {object}  rfc7807.Problem  "Session was not ready before the timeout"
// @Router       /api/strategies/blocking [get]
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
			recordSessionRequest(s.Metrics, "blocking", "")
			sessionState, err = s.Sablier.RequestReadySession(c.Request.Context(), request.Names, request.SessionDuration, request.Timeout)
		} else {
			sessionState, err = s.Sablier.RequestReadySessionGroup(c.Request.Context(), request.Group, request.SessionDuration, request.Timeout)
			if groupNotFoundError, ok := errors.AsType[sablier.ErrGroupNotFound](err); ok {
				AbortWithProblemDetail(c, ProblemGroupNotFound(groupNotFoundError))
				return
			}
			// Record only after the group is known to exist: an unbounded group
			// label value would let arbitrary query params blow up cardinality.
			recordSessionRequest(s.Metrics, "blocking", request.Group)
		}
		if err != nil {
			if timeoutErr, ok := errors.AsType[sablier.ErrTimeout](err); ok {
				AbortWithProblemDetail(c, ProblemTimeout(timeoutErr))
				return
			}
			if notManagedErr, ok := errors.AsType[sablier.ErrInstanceNotManaged](err); ok {
				AbortWithProblemDetail(c, ProblemInstanceNotManaged(notManagedErr))
				return
			}
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				AbortWithProblemDetail(c, ProblemRequestCancelled())
				return
			}
			AbortWithProblemDetail(c, ProblemError(err))
			return
		}

		if sessionState == nil {
			AbortWithProblemDetail(c, ProblemError(errors.New("session could not be created, please check logs for more details")))
			return
		}

		AddSablierHeader(c, sessionState)

		c.JSON(http.StatusOK, NewSessionResponse(sessionState))
	})
}
