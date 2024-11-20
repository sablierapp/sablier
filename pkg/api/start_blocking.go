package api

import "github.com/gin-gonic/gin"

func StartBlocking(router *gin.RouterGroup) {
	router.GET("/blocking", func(context *gin.Context) {

	})
}
