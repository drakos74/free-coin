package signal

import (
	"testing"

	"github.com/drakos74/free-coin/user/local"
)

func testRCommand(t *testing.T) {

	user, _ := local.NewUser("")

	trader, _ := newTrader("", nil, nil, nil)

	trader.switchOnOff(user)
}
