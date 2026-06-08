package timeparse

import (
	"errors"
	"time"
)

func Optional(value string) (time.Time, error) {
	if value == "" {
		return time.Time{}, nil
	}
	return time.Parse(time.RFC3339, value)
}

func Required(value string) (time.Time, error) {
	if value == "" {
		return time.Time{}, errors.New("missing time")
	}
	return time.Parse(time.RFC3339, value)
}
