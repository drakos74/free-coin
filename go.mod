module github.com/drakos74/free-coin

go 1.22

require (
	github.com/adshao/go-binance/v2 v2.2.1
	github.com/aopoltorzhicky/go_kraken/websocket v0.1.8
	github.com/beldur/kraken-go-api-client v0.0.0-20210113103835-3f11c80eba1a
	github.com/cdipaolo/goml v0.0.0-20220715001353-00e0c845ae1c
	github.com/drakos74/go-ex-machina v0.0.0-20211107134813-9131c4153fda
	github.com/go-telegram-bot-api/telegram-bot-api v4.6.4+incompatible
	github.com/google/uuid v1.1.1
	github.com/malaschitz/randomForest v0.0.0-20211016204141-298ff580d6a9
	github.com/mjibson/go-dsp v0.0.0-20180508042940-11479a337f12
	github.com/prometheus/client_golang v1.9.0
	github.com/rs/zerolog v1.21.0
	github.com/sjwhitworth/golearn v0.0.0-20211014193759-a8b69c276cd8
	github.com/stretchr/testify v1.7.0
	gonum.org/v1/gonum v0.14.0
)

require (
	github.com/aopoltorzhicky/go_kraken/rest v0.0.3 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/bitly/go-simplejson v0.5.0 // indirect
	github.com/cespare/xxhash/v2 v2.1.1 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/golang/protobuf v1.4.3 // indirect
	github.com/gonum/blas v0.0.0-20181208220705-f22b278b28ac // indirect
	github.com/google/go-cmp v0.5.8 // indirect
	github.com/gorilla/websocket v1.4.2 // indirect
	github.com/guptarohit/asciigraph v0.5.1 // indirect
	github.com/mattn/go-runewidth v0.0.7 // indirect
	github.com/matttproud/golang_protobuf_extensions v1.0.1 // indirect
	github.com/olekukonko/tablewriter v0.0.4 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/prometheus/client_model v0.2.0 // indirect
	github.com/prometheus/common v0.15.0 // indirect
	github.com/prometheus/procfs v0.2.0 // indirect
	github.com/rocketlaunchr/dataframe-go v0.0.0-20201007021539-67b046771f0b // indirect
	github.com/sirupsen/logrus v1.8.1 // indirect
	github.com/technoweenie/multipartstreamer v1.0.1 // indirect
	golang.org/x/exp v0.0.0-20230321023759-10a507213a29 // indirect
	golang.org/x/sync v0.0.0-20210220032951-036812b2e83c // indirect
	golang.org/x/sys v0.1.0 // indirect
	google.golang.org/protobuf v1.25.0 // indirect
	gopkg.in/yaml.v3 v3.0.0-20200313102051-9f266ea9e77c // indirect
)

replace github.com/beldur/kraken-go-api-client => ../kraken-go-api-client

replace github.com/sjwhitworth/golearn => ../golearn
