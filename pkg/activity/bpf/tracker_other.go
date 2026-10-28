//go:build !(linux && (amd64 || arm64))

package bpf

import "net/netip"

// Tracker is unavailable on this platform: New returns ErrNotSupported.
type Tracker struct{}

func New() (*Tracker, error) { return nil, ErrNotSupported }

func (t *Tracker) Watch(string, []Port) (Slot, error) { return 0, ErrNotSupported }

func (t *Tracker) Unwatch(Slot) error { return ErrNotSupported }

func (t *Tracker) Stale() ([]Slot, error) { return nil, ErrNotSupported }

func (t *Tracker) SetIgnored([]netip.Prefix) error { return ErrNotSupported }

func (t *Tracker) Read() (map[Slot]uint64, error) { return nil, ErrNotSupported }

func (t *Tracker) Close() error { return nil }
