package routes

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func (s *ServeStrategy) ServeDynamicThemes(c *gin.Context) {
	c.JSON(http.StatusOK, map[string]interface{}{
		"themes": s.Theme.List(),
	})
}
