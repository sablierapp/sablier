package api

import (
	"github.com/acouvreur/sablier/internal/session"
	"github.com/gin-gonic/gin"
	"net/http"
)

func GetSessions(sm *session.Manager) func(*gin.Context) {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, sm.List())
	}
}
