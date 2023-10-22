package models

import (
	"time"
)

type DynamicRequest struct {
	Group            string        `form:"group"`
	Names            []string      `form:"names"`
	ShowDetails      bool          `form:"show_details"`
	DisplayName      string        `form:"display_name"`
	Theme            string        `form:"theme"`
	SessionDuration  time.Duration `form:"session_duration"`
	RefreshFrequency time.Duration `form:"refresh_frequency"`
}
