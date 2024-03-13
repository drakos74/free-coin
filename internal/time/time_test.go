package time

import (
	"fmt"
	"testing"
	"time"
)

func TestFormat(t *testing.T) {
	now := time.Now()
	date := now.Format("YYYY-MM-DD")
	fmt.Printf("date = %+v\n", date)
	formatted := now.Format("20060102.1504")
	fmt.Printf("formatted = %+v\n", formatted)
}
