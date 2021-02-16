package time

import (
	"time"

	"github.com/rs/zerolog/log"
)

const timeUnixNano = 1000000000

func FromNano(nano int64) time.Time {
	return time.Unix(nano/timeUnixNano, 0)
}

// ThisWeek returns the unix time in seconds for the last 7 days.
func ThisWeek() int64 {
	return time.Now().AddDate(0, 0, -7).Unix()
}

// ThisDay returns the unix time in seconds for the last 24 hours.
func ThisDay() int64 {
	return time.Now().Add(-24 * time.Hour).Unix()
}

// LastXHours returns the time x hours before the current time in nanoseconds.
func LastXHours(h int) int64 {
	return time.Now().Add(-1*time.Duration(h)*time.Hour).Unix() * time.Second.Nanoseconds()
}

// ThisInstant returns the current time in nanoseconds.
func ThisInstant() int64 {
	return time.Now().Unix() * time.Second.Nanoseconds()
}

// Execute executes the given function at the specified interval providing also a shutdown hook.
func Execute(stop <-chan struct{}, interval time.Duration, exec func() error, shutdown func()) {
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
			case <-stop:
				log.Info().Float64("interval", interval.Seconds()).Msg("execution stopped")
				ticker.Stop()
				return
			}
		}
	}()
}
