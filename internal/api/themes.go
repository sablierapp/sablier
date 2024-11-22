package api

import (
	"github.com/gin-gonic/gin"
	"github.com/sablierapp/sablier/pkg/sablier"
	"net/http"
)

func GetThemes(router *gin.RouterGroup, s *sablier.Sablier) {
	router.GET("/themes", func(c *gin.Context) {
		c.JSON(http.StatusOK, s.Groups())
	})
}

func PreviewTheme(router *gin.RouterGroup, s *sablier.Sablier) {
	router.GET("/themes/", func(c *gin.Context) {
		c.JSON(http.StatusOK, s.Groups())
	})
}
