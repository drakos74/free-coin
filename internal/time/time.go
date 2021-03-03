package time

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/rs/zerolog/log"
)

const timeUnixNano = 1000000000

type Range struct {
	From    time.Time               `json:"from"`
	To      time.Time               `json:"to"`
	ToInt64 func(t time.Time) int64 `json:"-"`
}

func FromNano(nano int64) time.Time {
	return time.Unix(nano/timeUnixNano, 0)
}

func ToNano(t time.Time) int64 {
	return t.Unix() * timeUnixNano
}

func FromMilli(milli int64) time.Time {
	return time.Unix(milli/1000, 0)
}

func ToMilli(t time.Time) int64 {
	return t.Unix() * 1000
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

// Hash is a time hash helper
type Hash struct {
	duration int64
}

// NewHash creates a new time hash for the given duration.
func NewHash(duration time.Duration) Hash {
	return Hash{duration: int64(duration.Seconds())}
}

// Do converts the time to the hash.
func (h Hash) Do(t time.Time) int64 {
	return t.Unix() / h.duration
}

// Undo converts back the hash to the time.
func (h Hash) Undo(t int64) time.Time {
	return time.Unix(t*h.duration, 0)
}

type Duration struct {
	time.Duration
}

func (d Duration) MarshalJSON() ([]byte, error) {
	return json.Marshal(d.String())
}

func (d *Duration) UnmarshalJSON(b []byte) error {
	var v interface{}
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	switch value := v.(type) {
	case float64:
		d.Duration = time.Duration(value)
		return nil
	case string:
		var err error
		d.Duration, err = time.ParseDuration(value)
		if err != nil {
			return err
		}
		return nil
	default:
		return errors.New("invalid duration")
	}
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
