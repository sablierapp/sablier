package models

import (
	"time"
)

type DynamicRequest struct {
	Names           []string      `form:"names" binding:"required"`
	DisplayName     string        `form:"display_name"`
	Theme           string        `form:"theme"`
	SessionDuration time.Duration `form:"session_duration" binding:"required"`
}
