package api

import (
	"github.com/acouvreur/sablier/internal/theme"
	"github.com/gin-gonic/gin"
	"net/http"
)

func GetThemes(c *gin.Context, t *theme.Themes) {
	c.JSON(http.StatusOK, t.List())
}
