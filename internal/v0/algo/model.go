package algo

import "github.com/drakos74/free-coin/internal/model"

// SignalProcessor defines the method signature for a signal processor.
type SignalProcessor func(in <-chan *model.TrackedOrder, out chan<- *model.TrackedOrder)

// Void defines a dummy signal processor , that just propagates the signal to the next one.
func Void() SignalProcessor {
	return func(in <-chan *model.TrackedOrder, out chan<- *model.TrackedOrder) {
		for o := range in {
			out <- o
		}
	}
}
