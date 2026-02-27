package shared

import "time"

const TimeFormat = time.RFC3339

func Now() time.Time { return time.Now().UTC() }

func ParseTime(s string) (time.Time, error) {
	return time.Parse(TimeFormat, s)
}
