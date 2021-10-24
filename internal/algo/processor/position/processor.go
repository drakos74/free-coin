package position

import (
	"github.com/drakos74/free-coin/internal/algo/processor"
	"github.com/drakos74/free-coin/internal/api"
)

const (
	Name = "position-tracker"
)

// Processor is the position processor main routine.
func Processor(index api.Index) func(u api.User, e api.Exchange) api.Processor {
	return func(u api.User, e api.Exchange) api.Processor {
		t := newTracker(index, u, e)
		go t.track()
		return processor.NoProcess(Name)
	}
}
