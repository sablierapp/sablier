package api

import (
	"github.com/acouvreur/sablier/internal/provider"
	"github.com/gin-gonic/gin"
	"net/http"
)

func GetGroups(d *provider.Discovery) func(*gin.Context) {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, d.Groups())
	}
}
