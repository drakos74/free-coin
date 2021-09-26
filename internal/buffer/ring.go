package buffer

import (
	"fmt"
	"reflect"
)

// Transform is a operation acting on an object and returning another one.
// Because users know what objects they work with, this abstraction makes sense for the bucket scope.
// Otherwise it's clear that this method is too generic to be used in another context.
type Transform func(bucket interface{}) interface{}

// TimeBucketTransform is a operation acting on an TimeWindow slice.
type TimeBucketTransform func(bucket TimeBucket) interface{}

// Ring is a ring buffer keeping the last x elements
// TODO : use container/ring
type Ring struct {
	index  int
	count  int
	values []interface{}
	t      reflect.Type
}

// Size returns the number of non-nil elements within the ring.
func (r *Ring) Size() int {
	if r.count == r.index {
		return r.count
	}
	return len(r.values)
}

// NewRing creates a new ring with the given buffer size.
func NewRing(size int) *Ring {
	return &Ring{
		values: make([]interface{}, size),
	}
}

// Push adds an element to the ring.
func (r *Ring) Push(v interface{}) {

	tv := reflect.TypeOf(v)
	if r.t == nil {
		r.t = tv
	}

	if r.t != tv {
		panic(fmt.Sprintf("unepxected type added to ring %v vs %v", r.t, tv))
	}

	r.values[r.index] = v
	r.index = r.next(r.index)
	r.count++
}

func (r *Ring) next(index int) int {
	return (index + 1) % len(r.values)
}

// Get returns an ordered slice of the ring elements
func (r *Ring) Get(transform Transform) []interface{} {

	l := len(r.values)
	if r.count < l {
		l = r.count
	}

	v := make([]interface{}, l)
	for i := 0; i < l; i++ {
		idx := i
		if r.count > l {
			idx = r.next(r.index - 1 + i)
		}
		v[i] = transform(r.values[idx])
	}
	return v
}
