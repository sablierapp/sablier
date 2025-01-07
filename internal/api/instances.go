package api

import (
	"github.com/gin-gonic/gin"
	"github.com/sablierapp/sablier/pkg/sablier"
	"net/http"
)

func ListInstances(router *gin.RouterGroup, s *sablier.Sablier) {
	router.GET("/instances", func(c *gin.Context) {
		c.IndentedJSON(http.StatusOK, map[string]interface{}{"instances": s.InstancesInfo(c)})
	})
}
