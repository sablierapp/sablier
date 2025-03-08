package sablier

import (
	"fmt"
	"time"
)

type ErrGroupNotFound struct {
	Group           string
	AvailableGroups []string
}

func (g ErrGroupNotFound) Error() string {
	return fmt.Sprintf("group %s not found", g.Group)
}

type ErrRequestBinding struct {
	Err error
}

func (e ErrRequestBinding) Error() string {
	return e.Err.Error()
}

type ErrTimeout struct {
	Duration time.Duration
}

func (e ErrTimeout) Error() string {
	return fmt.Sprintf("timeout after %s", e.Duration)
}
