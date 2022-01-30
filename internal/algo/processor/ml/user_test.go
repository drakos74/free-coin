package ml

import (
	"testing"

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
			go trackUserActions("index", user, nil, nil, nil)

			user.Assert(t, local.NewUserMessage(tt.msg), tt.assertion)

		})
	}
}
