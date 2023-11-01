package api

import (
	"errors"
	"github.com/acouvreur/sablier/internal/theme"
	"github.com/gin-gonic/gin"
	"net/http"
	"time"
)

func GetThemes(t *theme.Themes) func(*gin.Context) {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, t.List())
	}
}

func PreviewTheme(t *theme.Themes) func(*gin.Context) {
	return func(c *gin.Context) {
		name := c.Param("theme")
		NotReady(c)
		c.JSON(http.StatusOK, t.Execute(c.Writer, name, theme.Options{
			Title:       "Sablier",
			DisplayName: "my apps",
			ShowDetails: true,
			Instances: []theme.InstanceInfo{
				{Name: "app1", Status: "starting", Error: nil},
				{Name: "app2", Status: "running", Error: nil},
				{Name: "app3", Status: "error", Error: errors.New("container app3 does not exist")},
			},
			SessionDuration:  1 * time.Hour,
			RefreshFrequency: 10 * time.Minute,
		}))
	}
}
