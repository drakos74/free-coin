package main

import (
	"fmt"

	"github.com/drakos74/free-coin/internal/emoji"
)

func main() {

	s := emoji.MapValue(10 * 0.4 / 2)
	fmt.Println(fmt.Sprintf("s = %+v", s))
}
