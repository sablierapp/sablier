package api

import (
	"github.com/gin-gonic/gin"
	"github.com/tniswong/go.rfcx/rfc7807"
	"net/url"
)

func AbortWithProblemDetail(c *gin.Context, p rfc7807.Problem) {
	_ = c.Error(p)
	instance, err := url.Parse(c.Request.RequestURI)
	if err != nil {
		instance = &url.URL{}
	}
	p.Instance = *instance
	c.Header("Content-Type", rfc7807.JSONMediaType)
	c.IndentedJSON(p.Status, p)
}
