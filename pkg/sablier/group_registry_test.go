package sablier

import (
	"fmt"
	"testing"
)

// TestGroupRegistry_ConcurrentGetAndRemove reproduces the data race between
// Get returning the internal member slice and removeFromGroup compacting the
// same backing array in place (members[:0] + append). This is the production
// interleaving of RequestSessionGroup (which iterates the slice returned by
// Get after releasing the lock) and GroupWatch handling a removal event.
// Run with -race: it fails on the unfixed registry.
func TestGroupRegistry_ConcurrentGetAndRemove(t *testing.T) {
	t.Parallel()

	r := newGroupRegistry()
	members := make([]string, 64)
	for i := range members {
		members[i] = fmt.Sprintf("instance-%d", i)
	}
	seed := map[string][]string{"g": members}
	r.Set(map[string][]string{"g": append([]string(nil), members...)})

	done := make(chan struct{})
	go func() {
		defer close(done)
		for range 1000 {
			names, ok := r.Get("g")
			if !ok {
				continue
			}
			// Iterate like RequestSessionGroup does after the lock is gone.
			for _, n := range names {
				_ = n
			}
		}
	}()

	// Concurrently trigger removeFromGroup's in-place compaction, then restore.
	for range 1000 {
		r.Remove("instance-0")
		r.Set(map[string][]string{"g": append([]string(nil), seed["g"]...)})
	}
	<-done
}

// TestGroupRegistry_GetReturnsCopy pins the ownership contract: mutating the
// slice returned by Get must not corrupt the registry's internal state.
func TestGroupRegistry_GetReturnsCopy(t *testing.T) {
	t.Parallel()

	r := newGroupRegistry()
	r.Set(map[string][]string{"g": {"a", "b"}})

	names, ok := r.Get("g")
	if !ok {
		t.Fatal("group not found")
	}
	names[0] = "corrupted"

	again, _ := r.Get("g")
	if again[0] != "a" {
		t.Fatalf("registry state mutated through Get's return value: %v", again)
	}
}

// TestGroupRegistry_SetCopiesInput pins the same contract on the way in:
// mutating the map passed to Set must not affect the registry.
func TestGroupRegistry_SetCopiesInput(t *testing.T) {
	t.Parallel()

	input := map[string][]string{"g": {"a", "b"}}
	r := newGroupRegistry()
	r.Set(input)

	input["g"][0] = "corrupted"
	delete(input, "g")

	names, ok := r.Get("g")
	if !ok || names[0] != "a" {
		t.Fatalf("registry state mutated through Set's input: %v (found=%v)", names, ok)
	}
}
