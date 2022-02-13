module github.com/drakos74/free-coin

go 1.16

require (
	github.com/adshao/go-binance/v2 v2.2.1
	github.com/aopoltorzhicky/go_kraken/websocket v0.1.8
	github.com/beldur/kraken-go-api-client v0.0.0-20210113103835-3f11c80eba1a
	github.com/drakos74/go-ex-machina v0.0.0-20211107134813-9131c4153fda
	github.com/go-telegram-bot-api/telegram-bot-api v4.6.4+incompatible
	github.com/google/go-cmp v0.5.4 // indirect
	github.com/google/uuid v1.1.1
	github.com/gorilla/websocket v1.4.2 // indirect
	github.com/mjibson/go-dsp v0.0.0-20180508042940-11479a337f12
	github.com/prometheus/client_golang v1.9.0
	github.com/rs/zerolog v1.20.0
	github.com/sjwhitworth/golearn v0.0.0-20211014193759-a8b69c276cd8
	github.com/stretchr/testify v1.7.0
	github.com/technoweenie/multipartstreamer v1.0.1 // indirect
	golang.org/x/xerrors v0.0.0-20200804184101-5ec99f83aff1 // indirect
	gonum.org/v1/gonum v0.9.3
	google.golang.org/protobuf v1.25.0 // indirect
)

replace github.com/beldur/kraken-go-api-client => ../kraken-go-api-client

replace github.com/sjwhitworth/golearn => ../golearn
