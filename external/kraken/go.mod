module github.com/drakos74/free-coin/external/kraken

go 1.16

require (
	github.com/beldur/kraken-go-api-client v0.0.0-20200330152217-ed78f31b987e
	github.com/drakos74/free-coin v0.0.0-20210210075349-61a134f2f6ea
	github.com/rs/zerolog v1.20.0
)

replace github.com/drakos74/free-coin => ../../
