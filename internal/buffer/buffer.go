package buffer

import (
	"fmt"
)

// Buffer defines a simple float buffer that acts like a constant size queue
type Buffer struct {
	size   int
	values []interface{}
}

// NewBuffer creates a new buffer.
func NewBuffer(size int) *Buffer {
	return &Buffer{
		size:   size,
		values: make([]interface{}, 0),
	}
}

// Push adds an element to the buffer.
func (b *Buffer) Push(x interface{}) (interface{}, bool) {
	b.values = append(b.values, x)
	if len(b.values) > b.size {
		value := b.values[0]
		b.values = b.values[1:]
		return value, true
	}
	return 0, false
}

// GetAsFloats returns the buffer elements in the order they were added.
func (b *Buffer) GetAsFloats(reverse bool) []float64 {
	size := len(b.values)
	vv := make([]float64, len(b.values))
	for i := 0; i < size; i++ {
		j := i
		if reverse {
			j = (size - 1) - i
		}
		switch v := b.values[i].(type) {
		case float64:
			vv[j] = v
		default:
			panic(fmt.Sprintf("unexpected value as float64 : %v", b.values[i]))
		}

	}
	return vv
}

// GetAsStrings returns the buffer elements in the order they were added.
func (b *Buffer) GetAsStrings(reverse bool) []string {
	size := len(b.values)
	vv := make([]string, len(b.values))
	for i := 0; i < size; i++ {
		j := i
		if reverse {
			j = (size - 1) - i
		}
		switch v := b.values[i].(type) {
		case string:
			vv[j] = v
		default:
			panic(fmt.Sprintf("unexpected value as string : %v", b.values[i]))
		}
	}
	return vv
}

func (b *Buffer) Size() int {
	return len(b.values)
}

// MultiBuffer defines a simple float slice buffer that acts like a constant size queue
type MultiBuffer struct {
	size   int
	values [][]float64
}

// NewMultiBuffer creates a new buffer.
func NewMultiBuffer(size int) *MultiBuffer {
	return &MultiBuffer{
		size:   size,
		values: make([][]float64, 0),
	}
}

// Push adds an element to the buffer.
func (b *MultiBuffer) Push(x ...float64) ([]float64, bool) {
	b.values = append(b.values, x)
	if len(b.values) > b.size {
		value := b.values[0]
		b.values = b.values[1:]
		return value, true
	}
	return nil, false
}

// GetReverse returns the buffer elements in the reverse order they were added.
func (b *MultiBuffer) GetReverse() [][]float64 {
	size := len(b.values)
	vv := make([][]float64, len(b.values))
	for i := size - 1; i >= 0; i-- {
		vv[size-1-i] = b.values[i]
	}
	return vv
}

// Get returns the buffer elements in the order they were added.
func (b *MultiBuffer) Get() [][]float64 {
	size := len(b.values)
	vv := make([][]float64, len(b.values))
	for i := 0; i < size; i++ {
		vv[i] = b.values[i]
	}
	return vv
}

// Len returns the current length of the buffer.
func (b *MultiBuffer) Len() int {
	return len(b.values)
}

// Last returns the last element in the buffer.
func (b *MultiBuffer) Last() []float64 {
	size := len(b.values)
	if size > 0 {
		return b.values[size-1]
	}
	return []float64{}
}
