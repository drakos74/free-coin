package trader

import (
	"testing"

	"github.com/drakos74/free-coin/internal/storage"
	"github.com/drakos74/free-coin/user/local"
	"github.com/stretchr/testify/assert"
)

func TestRCommand(t *testing.T) {

	user := local.NewMockUser()

	trader, _ := newTrader("", nil, storage.VoidShard(""), nil)

	go trader.switchOnOff(user)
	assert.True(t, trader.running)

	user.Assert(t, local.NewUserMessage("?r"), local.NewAssertion(1, local.Void()))

}
