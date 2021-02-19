package file

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestKey_SaveAndLoad(t *testing.T) {

	os.RemoveAll("testdata/tmp")

	k := Key{
		Since: 111,
		From:  time.Now(),
		To:    time.Now().Add(1 * time.Hour),
		Count: 10,
	}

	err := k.Save("testdata/tmp")
	assert.NoError(t, err)

	_, err = os.Stat("testdata/tmp/111.json")
	assert.NoError(t, err)

	nk := Key{}

	err = nk.Load("testdata/tmp/111.json")
	assert.NoError(t, err)

	assert.Equal(t, k.Since, nk.Since)
	assert.Equal(t, k.Count, nk.Count)
	assert.Equal(t, k.From.Unix(), nk.From.Unix())
	assert.Equal(t, k.To.Unix(), nk.To.Unix())

}
