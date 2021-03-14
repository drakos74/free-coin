package main

import (
	"context"
	"fmt"
	"time"

	"github.com/drakos74/free-coin/internal/server"

	"github.com/rs/zerolog/log"
)

const port = 6122

func main() {

	start := time.Now()

	ctx := context.Background()
	srv := server.NewServer("grafana", port).
		Add(server.Live()).
		AddRoute(server.POST, server.Data, "search", search).
		AddRoute(server.POST, server.Data, "tag-keys", keys).
		AddRoute(server.POST, server.Data, "tag-values", values).
		AddRoute(server.POST, server.Data, "annotations", annotations).
		Add(server.NewRoute(server.POST, server.Data).
			WithPath("query").
			Handler(query).
			AllowInterrupt().
			Create())
	go func() {
		err := srv.Run()
		if err != nil {
			panic(err.Error())
		}
	}()

	<-ctx.Done()
	duration := time.Since(start)
	log.Info().Str("duration", fmt.Sprintf("%v", duration)).Msg("back-test finished")
}
