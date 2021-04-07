package external

import (
	"context"
	"fmt"
	"net/http"

	"github.com/rs/zerolog/log"

	"github.com/drakos74/free-coin/internal/api"
	"github.com/drakos74/free-coin/internal/server"
)

func handle(user api.User, signal chan<- Message) server.Handler {
	return func(ctx context.Context, r *http.Request) ([]byte, int, error) {
		var message Message
		payload, jsonErr := server.ReadJson(r, false, &message)
		if jsonErr != nil {
			user.Send(api.External, api.NewMessage(fmt.Sprintf("error = %v \n raw = %+v", jsonErr.Error(), payload)), nil)
			return []byte{}, http.StatusBadRequest, nil
		}
		log.Debug().Str("message", fmt.Sprintf("%+v", message)).Msg("signal received")
		if signal != nil {
			signal <- message
		}
		return []byte{}, http.StatusOK, nil
	}
}
