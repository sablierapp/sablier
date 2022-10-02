package tinykv

import (
	"github.com/pkg/errors"
)

// Try tries to run a function and recovers from a panic, in case
// one happens, and returns the error, if there are any.
func try(f func() error) (errRun error) {
	defer func() {
		if e := recover(); e != nil {
			if err, ok := e.(error); ok {
				errRun = err
				return
			}
			errRun = errors.Errorf("RECOVERED, UNKNOWN ERROR: %+v", e)
		}
	}()
	return f()
}
