package main

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"
)

func main() {

	start := time.Now()

	ctx := context.Background()
	srv := New()
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
