package api

import (
	"github.com/gin-gonic/gin"
	"net/http"
)

// ListThemes registers the themes endpoint (also served at the legacy path /api/dynamic/themes).
//
// @Summary      List themes
// @Description  Lists the waiting-page themes available to the dynamic strategy (built-in and custom). Also served at the legacy path `/api/dynamic/themes`.
// @Tags         themes
// @Produce      json
// @Success      200  {object}  ThemesResponse
// @Router       /api/themes [get]
func ListThemes(router *gin.RouterGroup, s *ServeStrategy) {
	handler := func(c *gin.Context) {
		c.JSON(http.StatusOK, map[string]any{
			"themes": s.Theme.List(),
		})
	}

	router.GET("/themes", handler)
	router.GET("/dynamic/themes", handler) // Legacy path
}
