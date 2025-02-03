package sessions

import (
	"fmt"
	"time"
)

type GroupNotFoundError struct {
	Group           string
	AvailableGroups []string
}

func (g GroupNotFoundError) Error() string {
	return fmt.Sprintf("group %s not found", g.Group)
}

type RequestBindingError struct {
	Err error
}

func (e RequestBindingError) Error() string {
	return e.Err.Error()
}

type TimeoutError struct {
	Duration time.Duration
}

func (e TimeoutError) Error() string {
	return fmt.Sprintf("timeout after %s", e.Duration)
}
