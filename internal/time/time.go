package time

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/rs/zerolog/log"
)

func ToString(t time.Time) string {
	return fmt.Sprintf("%d_%d_%d_%d", t.Year(), t.Month(), t.Day(), t.Hour())
}

func ToNow(t time.Time) float64 {
	return t.Sub(time.Now()).Seconds()
}

func IsValidTime(t time.Time) bool {
	now := time.Now()
	ms := t.Unix() - now.Unix()
	d := time.Second * time.Duration(math.Abs(float64(ms)))
	if d > time.Minute {
		log.Trace().Time("now", now).Time("trade-time", t).Int64("seconds", ms).Float64("offset", d.Minutes()).Msg("offset time")
		return false
	}
	return true
}

func ToMinutes(d int) time.Duration {
	return time.Duration(d) * time.Minute
}

type Range struct {
	From    time.Time               `json:"from"`
	To      time.Time               `json:"to"`
	ToInt64 func(t time.Time) int64 `json:"-"`
}

func (r Range) IsWithin(time time.Time) bool {
	return time.Before(r.To) && time.After(r.From)
}

func (r Range) IsBeforeEnd(time time.Time) bool {
	return time.Before(r.To)
}

func (r Range) IsAfterStart(time time.Time) bool {
	return time.After(r.From)
}

func FromNano(nano int64) time.Time {
	return time.Unix(nano/time.Second.Nanoseconds(), 0)
}

func ToNano(t time.Time) int64 {
	return t.Unix() * time.Second.Nanoseconds()
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

func At(year, month, day, hour int) int64 {
	return time.Date(year, time.Month(month), day, hour, 0, 0, 0, time.UTC).Unix()
}

// Hash is a time hash helper
type Hash struct {
	duration int64
}

// NewHash creates a new time hash for the given duration.
func NewHash(duration time.Duration) Hash {
	if duration == 0 {
		return Hash{duration: 1}
	}
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

func Format(t time.Time) string {
	return t.Format("20060102.1504")
}
