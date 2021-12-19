package buffer

import (
	"fmt"
	"testing"
)

func TestBuffer_Push(t *testing.T) {

	b := NewBuffer(10)

	for i := 0; i < 100; i++ {
		l, ok := b.Push(float64(i))
		fmt.Printf("l = %+v\n", l)
		fmt.Printf("ok = %+v\n", ok)
		fmt.Printf("b.values = %+v\n", b.values)
		fmt.Printf("b.Get() = %+v\n", b.Get())
	}

}
