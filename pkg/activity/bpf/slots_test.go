package bpf

import (
	"testing"

	"gotest.tools/v3/assert"
)

func TestSlotPool_AllocatesLowestFree(t *testing.T) {
	var p slotPool

	s, err := p.alloc()
	assert.NilError(t, err)
	assert.Equal(t, s, Slot(0))

	s, err = p.alloc()
	assert.NilError(t, err)
	assert.Equal(t, s, Slot(1))

	p.free(0)
	s, err = p.alloc()
	assert.NilError(t, err)
	assert.Equal(t, s, Slot(0))
}

func TestSlotPool_Exhausted(t *testing.T) {
	var p slotPool
	for i := range MaxWatched {
		s, err := p.alloc()
		assert.NilError(t, err)
		assert.Equal(t, s, Slot(i))
	}

	_, err := p.alloc()
	assert.ErrorIs(t, err, ErrNoFreeSlot)
}
