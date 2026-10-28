// Package bpf counts the bytes containers exchange on chosen ports with a pair
// of cgroup_skb eBPF programs, one pair per watched cgroup.
package bpf

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
)

var (
	ErrNotSupported = errors.New("activity tracking requires Linux on amd64 or arm64")
	ErrNoFreeSlot   = errors.New("activity tracking: every slot is in use")
)

// MaxWatched is the number of cgroups that can be watched at once. It must
// match MAX_WATCHED in counter.c.
const MaxWatched = 256

// IP protocol numbers accepted in Port.Proto.
const (
	ProtoTCP uint8 = 6
	ProtoUDP uint8 = 17
)

// Port is a local port whose traffic is counted.
type Port struct {
	Number uint16
	Proto  uint8
}

// Slot identifies one watched cgroup and its byte total.
type Slot uint32

// slotPool hands out the lowest free slot.
type slotPool struct {
	used [MaxWatched]bool
}

func (p *slotPool) alloc() (Slot, error) {
	for i, used := range p.used {
		if !used {
			p.used[i] = true
			return Slot(i), nil
		}
	}
	return 0, ErrNoFreeSlot
}

func (p *slotPool) free(s Slot) {
	if int(s) < len(p.used) {
		p.used[s] = false
	}
}

// cgroupNotFoundError is returned when a watched cgroup doesn't exist. The
// usual cause is Sablier running in its own cgroup namespace, where it can't
// see the containers' cgroups.
type cgroupNotFoundError struct {
	path string
	err  error
}

func (e *cgroupNotFoundError) Error() string {
	return fmt.Sprintf("cgroup %s not found: run Sablier with cgroup: host on the Docker host", e.path)
}

func (e *cgroupNotFoundError) Unwrap() error { return e.err }

// checkCgroup checks that the cgroup directory at path exists.
func checkCgroup(path string) error {
	_, err := os.Stat(path)
	switch {
	case err == nil:
		return nil
	case errors.Is(err, fs.ErrNotExist):
		return &cgroupNotFoundError{path: path, err: err}
	default:
		return fmt.Errorf("cgroup %s: %w", path, err)
	}
}
