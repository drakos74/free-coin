package time

import (
	"fmt"
	"testing"
	"time"
)

func TestNow_Format(t *testing.T) {

	n := time.Now().Unix()
	println(fmt.Sprintf("n = %v", n))

	f := n / 100
	println(fmt.Sprintf("f = %v", f))
	l := n % 1000000
	println(fmt.Sprintf("l = %v", l))

	println(fmt.Sprintf("d = %v", 24*time.Hour.Seconds()))

}
