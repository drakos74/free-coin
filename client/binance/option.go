package binance

import (
	"fmt"
	"os"

	"github.com/drakos74/free-coin/internal/account"

	"github.com/drakos74/free-coin/internal/api"
)

type Option int

const (
	Name api.ExchangeName = "binance"
)

func exchangeConfig(user account.Name) (k, s string) {
	format := account.NewFormat(user, Name)

	key := os.Getenv(format.Key())
	if key == "" {
		panic(fmt.Sprintf("key not found for %s", format.Key()))
	}

	secret := os.Getenv(format.Secret())
	if secret == "" {
		panic(fmt.Sprintf("secret not found for %s", format.Secret()))
	}

	return key, secret
}
