package net

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSlicePart(t *testing.T) {

	ss := make([]int, 0)

	for i := 0; i < 100; i++ {
		ss = append(ss, i)
	}

	v := 96
	vv := ss[len(ss)-v:]

	fmt.Printf("v.len = %+v\n", len(vv))
	fmt.Printf("s.len = %+v\n", len(ss))
	assert.Equal(t, v, len(vv))

}
