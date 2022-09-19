package fetcher

import (
	"time"
)

const TimeFormat = "2006-01-02 15:04:05"

func ParseTime(s string) time.Time {
	t, err := time.Parse(TimeFormat, s)
	if err != nil {
		panic(err)
	}
	return t
}
