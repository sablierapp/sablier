// Package leakcheck makes a test binary fail when its tests leak goroutines.
// It uses the goroutineleak profile. See https://pkg.go.dev/runtime/pprof#Profile
package leakcheck

import (
	"bytes"
	"fmt"
	"os"
	"runtime/pprof"
	"strings"
	"testing"
)

// Main runs the tests and exits. If the tests pass but goroutines leaked,
// Main writes the leaked stacks and panics. Call Main from TestMain.
func Main(m *testing.M) {
	code := m.Run()
	if code == 0 {
		if report := find(); report != "" {
			fmt.Fprint(os.Stderr, report)
			// Do not use os.Exit(1). gotestsum --rerun-fails runs a failed TestMain again with no tests, and that run passes.
			// A panic stops the reruns. See https://github.com/gotestyourself/gotestsum/blob/v1.13.0/cmd/rerunfails.go#L98-L112
			panic("leakcheck: the goroutine leak check failed, see the output above")
		}
	}
	os.Exit(code)
}

// find returns the stacks of the leaked goroutines, or "" if no goroutine leaked.
func find() string {
	p := pprof.Lookup("goroutineleak")
	var b bytes.Buffer
	// WriteTo runs the GC cycle that finds the leaks, then Count gives their number.
	// With debug=2, the header of each leaked goroutine shows "(leaked)".
	if err := p.WriteTo(&b, 2); err != nil {
		return fmt.Sprintf("leakcheck: cannot write the goroutineleak profile: %v\n", err)
	}
	n := p.Count()
	if n == 0 {
		return ""
	}
	var leaked []string
	for stack := range strings.SplitSeq(b.String(), "\n\n") {
		if header, _, _ := strings.Cut(stack, "\n"); strings.Contains(header, "(leaked)") {
			leaked = append(leaked, stack)
		}
	}
	if len(leaked) == 0 {
		leaked = []string{b.String()}
	}
	return fmt.Sprintf("leakcheck: %d goroutine(s) leaked\n\n%s\n", n, strings.Join(leaked, "\n\n"))
}
