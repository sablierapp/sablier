package leakcheck

import (
	"strings"
	"testing"
	"time"
)

//go:noinline
func blockOnUnreachableChannel() {
	<-make(chan struct{})
}

//go:noinline
func blockOnReachableChannel(ch chan struct{}) {
	<-ch
}

func TestFind(t *testing.T) {
	ch := make(chan struct{})
	defer close(ch)
	go blockOnReachableChannel(ch)
	// This goroutine leaks on purpose. It stays blocked until the test binary exits.
	go blockOnUnreachableChannel()

	var report string
	for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		if report = find(); strings.Contains(report, "leakcheck.blockOnUnreachableChannel") {
			break
		}
	}

	if !strings.Contains(report, "leakcheck.blockOnUnreachableChannel") {
		t.Fatalf("find() does not report the leaked goroutine:\n%s", report)
	}
	if !strings.Contains(report, "created by github.com/sablierapp/sablier/internal/leakcheck.TestFind") {
		t.Errorf("find() does not report the goroutine that started the leak:\n%s", report)
	}
	if strings.Contains(report, "blockOnReachableChannel") {
		t.Errorf("find() reports a goroutine that waits on a reachable channel:\n%s", report)
	}
}
