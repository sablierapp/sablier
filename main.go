package main

import (
	"github.com/gin-gonic/gin"
	"github.com/sablierapp/sablier/cmd"
)

func main() {
	gin.SetMode(gin.ReleaseMode)
	cmd.Execute()
}
