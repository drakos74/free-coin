package ml

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"testing"

	"github.com/drakos74/free-coin/internal/storage/file/json"
	"github.com/stretchr/testify/assert"

	"github.com/drakos74/free-coin/user/local"
)

func TestTrackUserActions(t *testing.T) {

	type test struct {
		msg       string
		assertion local.Test
	}

	tests := map[string]test{
		"single_message": {
			msg:       "?ml",
			assertion: local.NewAssertion(1, local.Void()),
		},
		//"no_message": {
		//	msg:       "?other",
		//	assertion: local.NewAssertion(1, local.Void()),
		//},
		"reply_message": {
			msg:       "?ml",
			assertion: local.NewAssertion(1, local.Contains(Name)),
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			user := local.NewMockUser()
			go trackUserActions("index", user, nil, nil, nil, nil)

			user.Assert(t, local.NewUserMessage(tt.msg), tt.assertion)

		})
	}
}

func TestRegistryReport(t *testing.T) {

	registry := json.EventRegistry("ml-event-registry")

	log, err := registry("events")
	assert.NoError(t, err)

	root := log.Root()
	fmt.Printf("root = %+v\n", root)

	err = filepath.Walk(filepath.Join(json.RegistryPathPrefix(), root), func(path string, info fs.FileInfo, err error) error {
		if !info.IsDir() {
			fmt.Printf("path = %+v\n", path)
		}
		return nil
	})
	assert.NoError(t, err)

}
