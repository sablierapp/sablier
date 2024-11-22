package api

import (
	"github.com/gin-gonic/gin"
	"github.com/sablierapp/sablier/pkg/sablier"
	"net/http"
)

func GetGroups(router *gin.RouterGroup, s *sablier.Sablier) {
	router.GET("/groups", func(c *gin.Context) {
		c.JSON(http.StatusOK, s.Groups())
	})
}
