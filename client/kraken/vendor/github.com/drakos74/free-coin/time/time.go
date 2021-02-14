package time

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"
)

// Execute executes the given function at the specified interval providing also a shutdown hook.
func Execute(ctx context.Context, interval time.Duration, exec func() error, shutdown func()) {
	ticker := time.NewTicker(interval)
	go func() {
		err := exec()
		if err != nil {
			log.Warn().Err(err).Msg("ERROR")
		}
		defer shutdown()
		for {
			select {
			case <-ticker.C:
				err := exec()
				if err != nil {
					log.Warn().Err(err).Msg("ERROR")
				}
			case <-ctx.Done():
				log.Info().Str("context", fmt.Sprintf("%v", ctx)).Float64("interval", interval.Seconds()).Msg("Ticker Done")
				ticker.Stop()
				return
			}
		}
	}()
}
