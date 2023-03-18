package model

import (
	"fmt"
	"testing"
)

func TestEvolveInt(t *testing.T) {
	value := 60

	for i := 0; i < 100; i++ {
		v := EvolveInt(value, 0.0)
		fmt.Printf("v = %+v\n", v)
	}
}

func TestEvolveIntForce(t *testing.T) {
	value := 60

	for i := 0; i < 100; i++ {
		v := EvolveInt(value, 1.0)
		fmt.Printf("v = %+v\n", v)
	}
}

func TestEvolveFloat(t *testing.T) {
	value := 0.05

	for i := 0; i < 100; i++ {
		value = EvolveFloat(value, 0.0, 1)
		fmt.Printf("v = %+v\n", value)
	}
}
