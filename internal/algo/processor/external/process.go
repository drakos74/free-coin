package external

import (
	"context"
	"fmt"
	"net/http"

	"github.com/drakos74/free-coin/internal/api"
	"github.com/drakos74/free-coin/internal/server"
)

func handle(user api.User) server.Handler {
	return func(ctx context.Context, r *http.Request) ([]byte, int, error) {
		var message Message
		payload, jsonErr := server.ReadJson(r, true, &message)
		if jsonErr != nil {
			user.Send(api.Private, api.NewMessage(fmt.Sprintf("error = %v \n raw = %+v", jsonErr.Error(), payload)), nil)
			return []byte{}, http.StatusOK, nil
		}
		payload = fmt.Sprintf("%+v", message)
		user.Send(api.Private, api.NewMessage(fmt.Sprintf("json = %+v", payload)), nil)
		return []byte{}, http.StatusOK, nil
	}
}
