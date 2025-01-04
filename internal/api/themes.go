package api

import (
	"errors"
	"github.com/gin-gonic/gin"
	"github.com/sablierapp/sablier/pkg/sablier"
	"github.com/sablierapp/sablier/pkg/theme"
	"net/http"
	"time"
)

func GetThemes(router *gin.RouterGroup, s *sablier.Sablier) {
	router.GET("/themes", func(c *gin.Context) {
		c.JSON(http.StatusOK, map[string]interface{}{"themes": s.Theme.List()})
	})
}

func PreviewTheme(router *gin.RouterGroup, s *sablier.Sablier) {
	router.GET("/themes/:theme", func(c *gin.Context) {
		t := c.Param("theme")

		opts := theme.Options{
			DisplayName: "Preview Theme",
			ShowDetails: true,
			InstanceStates: []theme.Instance{
				{
					Name:            "preview-ready",
					Status:          "ready",
					Error:           nil,
					CurrentReplicas: 1,
					DesiredReplicas: 1,
				},
				{
					Name:            "preview-starting",
					Status:          "not-ready",
					Error:           nil,
					CurrentReplicas: 0,
					DesiredReplicas: 1,
				},
				{
					Name:            "preview-error",
					Status:          "error",
					Error:           errors.New("container does not exist"),
					CurrentReplicas: 0,
					DesiredReplicas: 0,
				},
			},
			SessionDuration:  10 * time.Minute,
			RefreshFrequency: 10 * time.Second,
		}

		err := s.Theme.Render(t, opts, c.Writer)
		if err != nil {
			c.AbortWithError(500, err)
			return
		}
	})
}
