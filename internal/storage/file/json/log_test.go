package json

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/drakos74/free-coin/internal/storage"

	"github.com/google/uuid"
)

type Event struct {
	Name  string `json:"name"`
	ID    string `json:"id"`
	Index int    `json:"index"`
}

func newEvent(i int) Event {
	return Event{
		Name:  "test",
		ID:    uuid.New().String(),
		Index: i,
	}
}

func TestEvents_Put(t *testing.T) {

	// clear up the test dir
	os.Remove("file-storage/events/processor/pair/label/1.events.log")

	logger := NewEventRegistry("processor").WithHash(1)

	k := storage.K{
		Pair:  "pair",
		Label: "label",
	}

	events := make([]Event, 0)
	for i := 0; i < 10; i++ {
		ev := newEvent(i)
		events = append(events, ev)
		err := logger.Put(k, ev)
		assert.NoError(t, err)
	}

	loadedEvents := []Event{Event{}}
	err := logger.Get(k, &loadedEvents)
	assert.NoError(t, err)

	assert.Equal(t, 10, len(loadedEvents))
	for i, ev := range events {
		assert.Equal(t, ev, loadedEvents[i])
	}

}
