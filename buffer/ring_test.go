package buffer

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestRing_Push(t *testing.T) {
	size := 10

	ring := NewRing(size)

	for i := 0; i < 1000; i++ {
		ring.Push(i)
		if i > size-1 {
			assert.Equal(t, size, ring.Size())
		} else {
			assert.Equal(t, i+1, ring.Size())
		}
	}
}

func TestRing_Get(t *testing.T) {

	size := 3
	type bb struct {
		index int
	}

	ring := NewRing(3)

	for i := 0; i < 100; i++ {
		ring.Push(bb{index: i})

		values := ring.Get(func(bucket interface{}) interface{} {
			if b, ok := bucket.(bb); ok {
				return b.index
			}
			return nil
		})

		if i > size-1 {
			assert.Equal(t, size, len(values))
			assert.Equal(t, i, values[2])
			assert.Equal(t, i-1, values[1])
			assert.Equal(t, i-2, values[0])
		} else {
			assert.Equal(t, i+1, len(values))
		}

	}

}
