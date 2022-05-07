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
		if ok {
			vv := b.Get()
			fmt.Printf("vv = %+v\n", vv)
		}
	}

}

func TestMultiBuffer_Push(t *testing.T) {

	b := NewMultiBuffer(2)

	for i := 0; i < 100; i++ {
		l, ok := b.Push(float64(i), float64(i*10))
		fmt.Printf("l = %+v\n", l)
		fmt.Printf("ok = %+v\n", ok)
		fmt.Printf("b.values = %+v\n", b.values)
		fmt.Printf("b.Get() = %+v\n", b.Get())
		if ok {
			vv := b.Get()
			fmt.Printf("vv = %+v\n", vv)
		}
	}

}
