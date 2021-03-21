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
		var bb []byte
		var payload string
		jsonErr := server.ReadJson(r, true, &bb)
		if jsonErr != nil {
			var err error
			payload, err = server.Read(r, true)
			if err != nil {
				log.Error().Err(err).Msg("error decoding request")
				return nil, http.StatusBadGateway, err
			}
			user.Send(api.Private, api.NewMessage(fmt.Sprintf("error = %v \n raw = %+v", jsonErr.Error(), payload)), nil)
			return []byte{}, http.StatusOK, nil
		}
		payload = string(bb)
		user.Send(api.Private, api.NewMessage(fmt.Sprintf("json = %+v", payload)), nil)
		return []byte{}, http.StatusOK, nil
	}
}
