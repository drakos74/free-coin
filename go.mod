module github.com/drakos74/free-coin

go 1.16

require (
	github.com/google/uuid v1.1.1
	github.com/rs/zerolog v1.20.0
	github.com/stretchr/testify v1.7.0
)

replace github.com/drakos74/free-coin/external/telegram => ./external/telegram
