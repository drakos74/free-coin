package binance

import (
	"fmt"
	"os"
)

type Option int

const (
	Free Option = iota + 1
	External
)

func ExchangeConfig(option Option) (k, s string) {
	switch option {
	case Free:
		return os.Getenv(key), os.Getenv(secret)
	case External:
		return os.Getenv(extKey), os.Getenv(extSecret)
	default:
		panic(fmt.Sprintf("unkown option: %v", option))
	}
}
