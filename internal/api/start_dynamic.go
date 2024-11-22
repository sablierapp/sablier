package api

import (
	"github.com/gin-gonic/gin"
	"github.com/sablierapp/sablier/pkg/sablier"
	"net/http"
	"time"
)

type DynamicRequest struct {
	Names           []string      `form:"names"`
	Group           string        `form:"group"`
	SessionDuration time.Duration `form:"session_duration"`
	Timeout         time.Duration `form:"timeout"`
}

func StartDynamic(router *gin.RouterGroup, s *sablier.Sablier) {
	router.GET("/dynamic", func(c *gin.Context) {
		request := BlockingRequest{
			// Timeout: s.StrategyConfig.Blocking.DefaultTimeout,
		}

		if err := c.ShouldBind(&request); err != nil {
			_ = c.AbortWithError(http.StatusBadRequest, err)
			return
		}

		opts := sablier.StartSessionOptions{
			Wait: false,
			StartOptions: sablier.StartOptions{
				DesiredReplicas:    1,
				ExpiresAfter:       request.SessionDuration,
				ConsiderReadyAfter: 0,
				Timeout:            request.Timeout,
			},
		}

		session, err := s.StartSessionByGroup(c, request.Group, opts)
		if err != nil {
			_ = c.AbortWithError(http.StatusBadRequest, err)
			return
		}

		AddSablierHeader(c, session)

		c.JSON(http.StatusOK, map[string]interface{}{"session": session})
	})
}
