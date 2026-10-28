package api

import (
	"github.com/gin-gonic/gin"
	"net/http"
)

// ListThemes registers the themes endpoint (also served at the legacy path /api/dynamic/themes).
//
// @Summary      List themes
// @Description  Lists the waiting-page themes available to the dynamic strategy (built-in and custom).
// @ID           listThemes
// @Tags         themes
// @Produce      json
// @Success      200  {object}  ThemesResponse
// @Router       /api/themes [get]
func ListThemes(router *gin.RouterGroup, s *ServeStrategy) {
	handler := func(c *gin.Context) {
		c.JSON(http.StatusOK, ThemesResponse{Themes: s.Theme.List()})
	}

	router.GET("/themes", handler)
	listThemesLegacy(router, handler)
}

// listThemesLegacy registers the legacy path of the themes endpoint. It is a
// separate function only so the path gets its own operationId in the spec.
//
// @Summary      List themes (legacy path)
// @Description  Legacy path of `/api/themes`, kept for compatibility.
// @ID           listThemesLegacy
// @Tags         themes
// @Produce      json
// @Success      200  {object}  ThemesResponse
// @Deprecated
// @Router       /api/dynamic/themes [get]
func listThemesLegacy(router *gin.RouterGroup, handler gin.HandlerFunc) {
	router.GET("/dynamic/themes", handler)
}
