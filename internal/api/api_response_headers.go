package api

import (
	"github.com/gin-gonic/gin"
	"github.com/sablierapp/sablier/pkg/sablier"
)

const SablierStatusHeader = "X-Sablier-Session-Status"
const SablierStatusReady = "ready"
const SablierStatusNotReady = "not-ready"

func AddSablierHeader(c *gin.Context, session *sablier.SessionState) {
	if session.IsReady() {
		c.Header(SablierStatusHeader, SablierStatusReady)
	} else {
		c.Header(SablierStatusHeader, SablierStatusNotReady)
	}
}
