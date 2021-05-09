module github.com/drakos74/free-coin

go 1.16

require (
	github.com/adshao/go-binance/v2 v2.2.1
	github.com/beldur/kraken-go-api-client v0.0.0-20210113103835-3f11c80eba1a
	github.com/go-telegram-bot-api/telegram-bot-api v4.6.4+incompatible
	github.com/google/go-cmp v0.5.4 // indirect
	github.com/google/uuid v1.1.1
	github.com/gorilla/websocket v1.4.2 // indirect
	github.com/kr/text v0.2.0 // indirect
	github.com/niemeyer/pretty v0.0.0-20200227124842-a10e7caefd8e // indirect
	github.com/prometheus/client_golang v1.9.0
	github.com/rs/zerolog v1.20.0
	github.com/stretchr/testify v1.7.0
	github.com/technoweenie/multipartstreamer v1.0.1 // indirect
	golang.org/x/sys v0.0.0-20210228012217-479acdf4ea46 // indirect
	golang.org/x/xerrors v0.0.0-20200804184101-5ec99f83aff1 // indirect
	google.golang.org/protobuf v1.25.0 // indirect
	gopkg.in/check.v1 v1.0.0-20200227125254-8fa46927fb4f // indirect
)

replace github.com/beldur/kraken-go-api-client => ../kraken-go-api-client
