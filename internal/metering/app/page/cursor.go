package page

import (
	"encoding/base64"
	"encoding/json"
	"time"

	"github.com/ssubedir/open-spanner/internal/metering/domain"
)

type Cursor struct {
	Name string
	Time time.Time
	ID   string
}

type cursorPayload struct {
	Name string `json:"name,omitempty"`
	Time string `json:"time,omitempty"`
	ID   string `json:"id,omitempty"`
}

func Decode(value string) (Cursor, error) {
	if value == "" {
		return Cursor{}, nil
	}

	payload, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return Cursor{}, domain.ErrInvalidInput
	}

	var decoded cursorPayload
	if err := json.Unmarshal(payload, &decoded); err != nil {
		return Cursor{}, domain.ErrInvalidInput
	}

	var cursor Cursor
	cursor.Name = decoded.Name
	cursor.ID = decoded.ID
	if decoded.Time != "" {
		parsed, err := time.Parse(time.RFC3339Nano, decoded.Time)
		if err != nil {
			return Cursor{}, domain.ErrInvalidInput
		}
		cursor.Time = parsed
	}

	return cursor, nil
}

func Encode(cursor Cursor) (string, error) {
	if cursor.Name == "" && cursor.Time.IsZero() && cursor.ID == "" {
		return "", nil
	}

	payload := cursorPayload{
		Name: cursor.Name,
		ID:   cursor.ID,
	}
	if !cursor.Time.IsZero() {
		payload.Time = cursor.Time.UTC().Format(time.RFC3339Nano)
	}

	encoded, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	return base64.RawURLEncoding.EncodeToString(encoded), nil
}
