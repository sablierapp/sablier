package api

import (
	"github.com/gin-gonic/gin"
	"github.com/sablierapp/sablier/pkg/sablier"
	"net/http"
	"time"
)

// TODO: Add missing theme customization
type DynamicRequest struct {
	Names           []string      `form:"names"`
	Group           string        `form:"group"`
	SessionDuration time.Duration `form:"session_duration"`
	Timeout         time.Duration `form:"timeout"`
	Theme           string        `form:"theme"`
}

func StartDynamic(router *gin.RouterGroup, s *sablier.Sablier) {
	handler := func(c *gin.Context) {
		request := DynamicRequest{
			// Timeout: s.StrategyConfig.Blocking.DefaultTimeout,
			Group:           "",
			SessionDuration: 10 * time.Second,
			Timeout:         30 * time.Second,
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

		session, err := s.StartSessionByNames(c, request.Names, opts)
		if err != nil {
			_ = c.AbortWithError(http.StatusBadRequest, err)
			return
		}

		AddSablierHeader(c, session)

		c.JSON(http.StatusOK, map[string]interface{}{"session": session})
	}
	router.GET("/dynamic", handler)
	router.GET("/strategies/dynamic", handler)
}
