package external

import (
	"context"
	"fmt"
	"net/http"

	"github.com/drakos74/free-coin/internal/api"
	"github.com/drakos74/free-coin/internal/server"
	"github.com/rs/zerolog/log"
)

func handle(user api.User) server.Handler {
	return func(ctx context.Context, r *http.Request) ([]byte, int, error) {
		payload, err := server.Read(r, true)
		if err != nil {
			log.Error().Err(err).Msg("error decoding request")
			return nil, http.StatusBadGateway, err
		}
		log.Warn().Str("body", fmt.Sprintf("%+v", payload)).Msg("api request")
		user.Send(api.Private, api.NewMessage(fmt.Sprintf("%+v", payload)), nil)
		return []byte{}, http.StatusOK, nil
	}
}
