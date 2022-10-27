package models

import (
	"time"
)

type DynamicRequest struct {
	Names           []string      `form:"names" binding:"required"`
	DisplayName     string        `form:"display-name" binding:"required"`
	Theme           string        `form:"theme" binding:"required"`
	SessionDuration time.Duration `form:"session-duration" binding:"required"`
}
