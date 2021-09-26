package local

import (
	"testing"

	"github.com/drakos74/free-coin/internal/api"
	"github.com/stretchr/testify/assert"
)

func TestLoggerSTD(t *testing.T) {

	lg, err := NewUser("")
	assert.NoError(t, err)
	lg.Send("", api.NewMessage("test"), nil)

}
