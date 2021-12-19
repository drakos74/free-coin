package ml

import (
	"github.com/drakos74/free-coin/internal/api"
	"github.com/rs/zerolog/log"
)

func trackUserActions(user api.User, collector *collector) {
	for command := range user.Listen("ml", "?ml") {
		log.Info().
			Str("user", command.User).
			Str("message", command.Content).
			Str("processor", Name).
			Msg("message received")
	}
}
