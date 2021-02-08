package time

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"
)

const UnixPrecision = 1000000

type Time struct {
	Stripped   float64
	Real       time.Time
	UnixSecond int64
	UnixNano   int64
}

// good to know how many nanoseconds a second is ... i always forget and have to google.
//const nano = 1000000000 // nolint:deadcode

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

func Nano(t time.Time) int64 {
	return t.Unix() * time.Second.Nanoseconds()
}

// New creates a new cointime.
func New(seconds int64) Time {
	var nano int64 = 0
	second := seconds
	return Time{
		Stripped:   UnixFloat32Precision(second),
		Real:       time.Unix(second, nano),
		UnixSecond: second,
		UnixNano:   nano,
	}
}

// At sets the cointime to the specifies time.
func At(t time.Time) Time {
	u := t.Unix()
	// keep the last 6 digits (enough to cover for a day)
	return Time{
		Stripped:   UnixFloat32Precision(u),
		UnixSecond: u,
		UnixNano:   t.UnixNano(),
		Real:       t,
	}
}

// Now returns the current time in cointime.
func Now() Time {
	return At(time.Now())
}

func UnixFloat32Precision(v int64) float64 {
	return float64(v % UnixPrecision)
}

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
