package api

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/sablierapp/sablier/pkg/sablier"
)

func AddServerHeader(c *gin.Context) {

}

func AddSablierHeader(c *gin.Context, session sablier.SessionInfo) {
	switch session.Status {
	case sablier.SessionStatusReady:
		c.Header("X-Sablier-Session-Status", "ready")
	case sablier.SessionStatusNotReady:
		c.Header("X-Sablier-Session-Status", "ready")
	default:
		c.Header("X-Sablier-Session-Status", fmt.Sprintf("unknown (%s)", session.Status))
	}
}
