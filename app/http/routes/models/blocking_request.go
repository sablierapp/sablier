package models

import "time"

type BlockingRequest struct {
	Names           []string
	SessionDuration time.Duration
	Timeout         time.Duration
}
