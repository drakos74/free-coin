package concurrent

import (
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

type Assertion struct {
	counter  *Counter
	expected int
}

func NewAssertion(expected int) *Assertion {
	wg := new(sync.WaitGroup)
	wg.Add(expected)
	return &Assertion{
		counter:  NewCounter(wg),
		expected: expected,
	}
}

func (a *Assertion) Expect(v interface{}) {
	if a.expected > 0 {
		a.counter.Track(v)
	}
	if a.expected == 0 {
		panic(fmt.Sprintf("unexpected event: %v", v))
	}
}

func (a *Assertion) Assert(t *testing.T) {
	a.counter.waitGroup.Wait()
	assert.Equal(t, a.expected, a.counter.Get())
	for _, v := range a.counter.Values() {
		fmt.Printf("v = %+v\n", v)
	}
}
