package main

import (
	"fmt"
	"testing"
)

func testModels(t *testing.T) {

	_, _, err := models()(nil, nil)
	fmt.Printf("err = %+v\n", err)

}
