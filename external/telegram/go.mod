module github.com/drakos74/free-coin/external/telegram

go 1.16

require (
	github.com/drakos74/free-coin v0.0.0-20210214162130-6d3e0c480937
	github.com/go-telegram-bot-api/telegram-bot-api v4.6.4+incompatible
	github.com/rs/zerolog v1.20.0
	github.com/stretchr/testify v1.7.0
	github.com/technoweenie/multipartstreamer v1.0.1 // indirect
)

replace github.com/drakos74/free-coin => ../../
