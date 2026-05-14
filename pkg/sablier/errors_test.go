package sablier_test

import (
	"errors"
	"testing"
	"time"

	"github.com/sablierapp/sablier/pkg/sablier"
	"gotest.tools/v3/assert"
)

func TestErrGroupNotFound_Error(t *testing.T) {
	err := sablier.ErrGroupNotFound{Group: "missing"}
	assert.Equal(t, err.Error(), "group missing not found")
}

func TestErrRequestBinding_Error(t *testing.T) {
	err := sablier.ErrRequestBinding{Err: errors.New("invalid payload")}
	assert.Equal(t, err.Error(), "invalid payload")
}

func TestErrTimeout_Error(t *testing.T) {
	err := sablier.ErrTimeout{Duration: 3 * time.Second}
	assert.Equal(t, err.Error(), "timeout after 3s")
}
