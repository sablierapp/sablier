package routes

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/acouvreur/sablier/version"
)

func GetVersion(c *gin.Context) {
	c.JSON(http.StatusOK, version.Map())
}
