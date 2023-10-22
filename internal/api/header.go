package api

import "github.com/gin-gonic/gin"

const (
	SablierSessionStatusHeader   string = "X-Sablier-Session-Status"
	SablierSessionStatusReady    string = "ready"
	SablierSessionStatusNotReady string = "not-ready"
)

func Ready(c *gin.Context) {
	c.Header(SablierSessionStatusHeader, SablierSessionStatusReady)
}

func NotReady(c *gin.Context) {
	c.Header(SablierSessionStatusHeader, SablierSessionStatusNotReady)
}
