package models

import (
	"time"
)

type DynamicRequest struct {
	Names           []string
	DisplayName     string
	Theme           string
	SessionDuration time.Duration
}
