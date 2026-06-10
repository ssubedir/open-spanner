package postgres

import (
	"strconv"
	"strings"
	"time"
)

func formatTime(value time.Time) string {
	return value.UTC().Format(time.RFC3339Nano)
}

func isUniqueConstraint(err error) bool {
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "unique constraint") || strings.Contains(message, "sqlstate 23505")
}

func rebind(query string) string {
	var builder strings.Builder
	builder.Grow(len(query) + 8)

	index := 1
	for _, char := range query {
		if char == '?' {
			builder.WriteByte('$')
			builder.WriteString(strconv.Itoa(index))
			index++
			continue
		}
		builder.WriteRune(char)
	}

	return builder.String()
}
