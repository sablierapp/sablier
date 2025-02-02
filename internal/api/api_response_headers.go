package api

import (
	"github.com/gin-gonic/gin"
	"github.com/sablierapp/sablier/app/sessions"
)

const SablierStatusHeader = "X-Sablier-Session-Status"
const SablierStatusReady = "ready"
const SablierStatusNotReady = "not-ready"

func AddSablierHeader(c *gin.Context, session *sessions.SessionState) {
	if session.IsReady() {
		c.Header(SablierStatusHeader, SablierStatusReady)
	} else {
		c.Header(SablierStatusHeader, SablierStatusNotReady)
	}
}
