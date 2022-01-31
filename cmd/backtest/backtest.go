package main

import (
	"context"

	"github.com/rs/zerolog"

	"github.com/drakos74/free-coin/internal/server"
	"github.com/rs/zerolog/log"
)

const (
	port = 6090
)

func init() {
	zerolog.SetGlobalLevel(zerolog.DebugLevel)
}

func main() {

	go func() {
		err := server.NewServer("back-test", port).
			AddRoute(server.GET, server.Test, "train", train()).
			AddRoute(server.GET, server.Test, "models", models()).
			//AddRoute(server.GET, server.Test, "run", run()).
			AddRoute(server.GET, server.Test, "load", load()).
			AddRoute(server.GET, server.Test, "history", hist()).
			Run()
		if err != nil {
			log.Error().Err(err).Msg("could not start server")
		}
	}()

	<-context.Background().Done()

}
