package api

import (
	"github.com/acouvreur/sablier/internal/session"
	"github.com/gin-gonic/gin"
)

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

func applyStatusHeader(c *gin.Context, instances []session.Instance) {
	if session.Ready(instances) {
		Ready(c)
	} else {
		NotReady(c)
	}
}
