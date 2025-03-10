package api

import (
	"github.com/gin-gonic/gin"
	"net/http"
)

func ListThemes(router *gin.RouterGroup, s *ServeStrategy) {
	handler := func(c *gin.Context) {
		c.JSON(http.StatusOK, map[string]interface{}{
			"themes": s.Theme.List(),
		})
	}

	router.GET("/themes", handler)
	router.GET("/dynamic/themes", handler) // Legacy path
}
