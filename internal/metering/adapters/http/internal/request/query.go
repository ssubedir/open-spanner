package request

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"time"
)

var (
	ErrInvalidJSON  = NewValidationError("invalid_json", "invalid JSON body")
	ErrInvalidLimit = NewValidationError("invalid_limit", "limit must be a positive integer")
	ErrInvalidBool  = NewValidationError("invalid_bool", "value must be true or false")
)

type ValidationError struct {
	code    string
	message string
}

func NewValidationError(code string, message string) ValidationError {
	return ValidationError{code: code, message: message}
}

func (e ValidationError) Error() string {
	return e.message
}

func (e ValidationError) Code() string {
	return e.code
}

func (e ValidationError) Message() string {
	return e.message
}

func Code(err error) string {
	var validation ValidationError
	if errors.As(err, &validation) {
		return validation.Code()
	}
	return "invalid_input"
}

func Message(err error) string {
	var validation ValidationError
	if errors.As(err, &validation) {
		return validation.Message()
	}
	return err.Error()
}

func ParseLimit(value string) (int, error) {
	if value == "" {
		return 0, nil
	}
	limit, err := strconv.Atoi(value)
	if err != nil || limit <= 0 {
		return 0, ErrInvalidLimit
	}
	return limit, nil
}

func ParseOptionalBool(name string, value string) (bool, error) {
	if value == "" {
		return false, nil
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return false, NewValidationError("invalid_"+name, name+" must be true or false")
	}
	return parsed, nil
}

func DecodeJSON(body io.Reader, target any) error {
	decoder := json.NewDecoder(body)
	if err := decoder.Decode(target); err != nil {
		return ErrInvalidJSON
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return ErrInvalidJSON
	}
	return nil
}

func DecodeJSONArray(body io.Reader, target any, length func() int, max int, label string) error {
	if err := DecodeJSON(body, target); err != nil {
		return err
	}
	if max > 0 && length() > max {
		return NewValidationError("invalid_input", fmt.Sprintf("invalid input: %s limit is %d", label, max))
	}
	return nil
}

func OptionalTime(name string, value string) (time.Time, error) {
	if value == "" {
		return time.Time{}, nil
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}, NewValidationError("invalid_"+name, name+" must be RFC3339")
	}
	return parsed, nil
}

func RequiredTime(name string, value string) (time.Time, error) {
	if value == "" {
		return time.Time{}, NewValidationError("invalid_"+name, name+" must be RFC3339")
	}
	return OptionalTime(name, value)
}
