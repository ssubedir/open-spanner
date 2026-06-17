package request

import (
	"errors"
	"strings"
	"testing"
)

func TestParseLimitRejectsInvalidValues(t *testing.T) {
	_, err := ParseLimit("0")
	if !errors.As(err, new(ValidationError)) {
		t.Fatalf("parse limit error = %v, want validation error", err)
	}
	if Code(err) != "invalid_limit" || Message(err) != "limit must be a positive integer" {
		t.Fatalf("parse limit error code/message = %q/%q", Code(err), Message(err))
	}
}

func TestValidateOptionalLimitRejectsNegativeAndOverMax(t *testing.T) {
	if err := ValidateOptionalLimit(0, 100); err != nil {
		t.Fatalf("zero optional limit error = %v", err)
	}
	if err := ValidateOptionalLimit(100, 100); err != nil {
		t.Fatalf("max optional limit error = %v", err)
	}

	err := ValidateOptionalLimit(-1, 100)
	if Code(err) != "invalid_limit" || Message(err) != "limit must be a positive integer" {
		t.Fatalf("negative optional limit error code/message = %q/%q", Code(err), Message(err))
	}

	err = ValidateOptionalLimit(101, 100)
	if Code(err) != "invalid_limit" || Message(err) != "limit must be less than or equal to 100" {
		t.Fatalf("over max optional limit error code/message = %q/%q", Code(err), Message(err))
	}
}

func TestParseOptionalBoolUsesParameterName(t *testing.T) {
	_, err := ParseOptionalBool("dry_run", "maybe")
	if Code(err) != "invalid_dry_run" || Message(err) != "dry_run must be true or false" {
		t.Fatalf("parse bool error code/message = %q/%q", Code(err), Message(err))
	}
}

func TestTimeParsersUseParameterName(t *testing.T) {
	_, err := OptionalTime("timestamp", "soon")
	if Code(err) != "invalid_timestamp" || Message(err) != "timestamp must be RFC3339" {
		t.Fatalf("optional time error code/message = %q/%q", Code(err), Message(err))
	}

	_, err = RequiredTime("from", "")
	if Code(err) != "invalid_from" || Message(err) != "from must be RFC3339" {
		t.Fatalf("required time error code/message = %q/%q", Code(err), Message(err))
	}
}

func TestDecodeJSONRejectsInvalidAndTrailingBody(t *testing.T) {
	var payload struct {
		Name string `json:"name"`
	}
	if err := DecodeJSON(strings.NewReader(`{"name":"ok"} {"name":"extra"}`), &payload); Code(err) != "invalid_json" {
		t.Fatalf("decode trailing JSON error = %v, code = %q", err, Code(err))
	}

	if err := DecodeJSON(strings.NewReader(`{"name":"ok"}`), &payload); err != nil {
		t.Fatalf("decode valid JSON: %v", err)
	}
}
