package sqlite

import (
	"math"
	"time"
)

func FromSecs(s float64) time.Time {
	if s > math.MaxInt64 || s <= 1e-9 {
		return time.Time{}
	}
	sec, nsec := math.Modf(s)
	return time.Unix(int64(sec), int64(nsec*1e9))
}

func ToSecs(t time.Time) float64 {
	if t.IsZero() {
		return 0
	}
	sec := t.Unix()
	nsec := t.Nanosecond()
	return float64(sec) + float64(nsec)*1e-9
}
